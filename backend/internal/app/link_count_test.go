package app

import (
	"context"
	"encoding/json"
	"testing"
)

func TestAvailableLinkCountIgnoresVisitFilters(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	// Dashboard availability follows only the same switch used by link management.
	_, err := a.DB.Exec(context.Background(), `INSERT INTO short_links(code,name,target_url,enabled) VALUES
 ('disabled','Disabled','https://wa.me/13365661092',false),
 ('active','Active','https://wa.me/13365661092',true)`)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"", "?start=2020-01-01&end=2020-01-01&link_id=1&ad_id=missing"} {
		w := call(a, "GET", "/api/v1/analytics"+query, "", admin)
		var result struct{ Summary map[string]int }
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Summary["available_links"] != 2 {
			t.Fatalf("available link count: %s", w.Body.String())
		}
	}
}
