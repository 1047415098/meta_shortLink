package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestMetaPixelAndConnectionDeleteLifecycle(t *testing.T) {
	a := setup(t)
	admin := login(t, a)

	// Destructive configuration endpoints remain behind the administrator session.
	if w := call(a, "DELETE", "/api/v1/meta/pixels/1", "", nil); w.Code != 401 {
		t.Fatalf("anonymous Pixel delete returned %d", w.Code)
	}
	if w := call(a, "DELETE", "/api/v1/meta/connections/1", "", nil); w.Code != 401 {
		t.Fatalf("anonymous account delete returned %d", w.Code)
	}

	connectionID := metaConnection(t, a, "445566", "778899", true)
	var pixelID int64
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM meta_pixels WHERE connection_id=$1", connectionID).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}

	// An account is the owner of its Pixels and cannot disappear first.
	w := call(a, "DELETE", fmt.Sprintf("/api/v1/meta/connections/%d", connectionID), "", admin)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "Pixel") {
		t.Fatalf("account with Pixel delete: %d %s", w.Code, w.Body.String())
	}

	// An unused Pixel can be removed, including its encrypted CAPI credential.
	w = call(a, "DELETE", fmt.Sprintf("/api/v1/meta/pixels/%d", pixelID), "", admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("unused Pixel delete: %d %s", w.Code, w.Body.String())
	}
	var count int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM meta_pixels WHERE id=$1", pixelID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("Pixel remains after delete: count=%d error=%v", count, err)
	}

	// Once no business record references the account, the account can be removed.
	w = call(a, "DELETE", fmt.Sprintf("/api/v1/meta/connections/%d", connectionID), "", admin)
	if w.Code != 200 {
		t.Fatalf("unused account delete: %d %s", w.Code, w.Body.String())
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM meta_connections WHERE id=$1", connectionID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("account remains after delete: count=%d error=%v", count, err)
	}

	// Both deletions leave an administrator audit trail without retaining the token.
	var actions []string
	rows, err := a.DB.Query(context.Background(), "SELECT action FROM audit_logs WHERE action IN ('meta.pixel.delete','meta.connection.delete') ORDER BY action")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var action string
		if err = rows.Scan(&action); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, action)
	}
	if got, _ := json.Marshal(actions); string(got) != `["meta.connection.delete","meta.pixel.delete"]` {
		t.Fatalf("delete audit actions: %s", got)
	}
}

func TestMetaDeleteRejectsBusinessReferences(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	connectionID := metaConnection(t, a, "123450", "987650", true)
	var pixelID int64
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM meta_pixels WHERE connection_id=$1", connectionID).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}

	// A bound link must be explicitly moved to another Pixel before its target is deleted.
	metaLanding(t, a, connectionID)
	w := call(a, "DELETE", fmt.Sprintf("/api/v1/meta/pixels/%d", pixelID), "", admin)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "短链接") {
		t.Fatalf("bound Pixel delete: %d %s", w.Code, w.Body.String())
	}

	// Removing the link binding alone still preserves the visit/event history contract.
	if _, err := a.DB.Exec(context.Background(), "UPDATE short_links SET meta_connection_id=NULL,meta_pixel_id=NULL WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), `INSERT INTO meta_events(id,connection_id,pixel_record_id,event_name,event_time,pixel_id,status) VALUES('history',$1,$2,'Contact',now(),$3,'succeeded')`, connectionID, pixelID, "987650"); err != nil {
		t.Fatal(err)
	}
	w = call(a, "DELETE", fmt.Sprintf("/api/v1/meta/pixels/%d", pixelID), "", admin)
	if w.Code != 409 || !strings.Contains(w.Body.String(), "历史") {
		t.Fatalf("historical Pixel delete: %d %s", w.Code, w.Body.String())
	}

	// Unknown records use a normal not-found response and never create audit rows.
	if w = call(a, "DELETE", "/api/v1/meta/pixels/999999", "", admin); w.Code != 404 {
		t.Fatalf("missing Pixel delete: %d %s", w.Code, w.Body.String())
	}
	if w = call(a, "DELETE", "/api/v1/meta/connections/999999", "", admin); w.Code != 404 {
		t.Fatalf("missing account delete: %d %s", w.Code, w.Body.String())
	}
}
