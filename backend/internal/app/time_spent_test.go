package app

import (
	"context"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestTimeSpentUsesFrozenThresholdAndQueuesOnce(t *testing.T) {
	a := setup(t)
	connectionID := metaConnection(t, a, "12345", "98765", true)
	metaLanding(t, a, connectionID)
	admin := login(t, a)
	if response := call(a, "PATCH", "/api/v1/links/1", `{"time_spent_threshold":5}`, admin); response.Code != 200 {
		t.Fatalf("configure threshold: %d %s", response.Code, response.Body.String())
	}
	ticket := metaTicket(t, a)
	visitID := strings.SplitN(ticket, ".", 2)[0]
	post := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", "/hello/time-spent", strings.NewReader(url.Values{"ticket": {ticket}}.Encode()))
		request.RemoteAddr = "192.0.2.7:43123"
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.Header.Set("User-Agent", "Mozilla/5.0 iPhone")
		result := httptest.NewRecorder()
		a.Router.ServeHTTP(result, request)
		return result
	}
	// A signed browser cannot report before server-observed elapsed time.
	if result := post(); result.Code != 409 {
		t.Fatalf("early report status = %d", result.Code)
	}
	// Editing the code affects future visits only; this open page keeps five seconds.
	if response := call(a, "PATCH", "/api/v1/links/1", `{"time_spent_threshold":0}`, admin); response.Code != 200 {
		t.Fatalf("disable future threshold: %d %s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '6 seconds' WHERE id=$1", visitID); err != nil {
		t.Fatal(err)
	}
	if result := post(); result.Code != 204 {
		t.Fatalf("qualified report status = %d %s", result.Code, result.Body.String())
	}
	if result := post(); result.Code != 204 {
		t.Fatalf("idempotent retry status = %d", result.Code)
	}
	var threshold, timestamps, events int
	var eventName, eventID string
	if err := a.DB.QueryRow(context.Background(), `SELECT e.time_spent_threshold,
		count(e.time_spent_reported_at)::int,
		count(m.id)::int,
		COALESCE(max(m.event_name),''),COALESCE(max(m.id),'')
		FROM click_events e LEFT JOIN meta_events m ON m.visit_id=e.id
		WHERE e.id=$1 GROUP BY e.time_spent_threshold`, visitID).Scan(&threshold, &timestamps, &events, &eventName, &eventID); err != nil {
		t.Fatal(err)
	}
	if threshold != 5 || timestamps != 1 || events != 1 || eventName != "TimeSpent" || eventID != "wa_"+visitID+"_time_spent" {
		t.Fatalf("threshold=%d timestamps=%d events=%d name=%q id=%q", threshold, timestamps, events, eventName, eventID)
	}
}
