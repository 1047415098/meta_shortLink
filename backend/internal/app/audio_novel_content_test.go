package app

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestAudioNovelContentFlow(t *testing.T) {
	a := setup(t)
	if got := call(a, "GET", "/api/v1/audio-novels", "", nil).Code; got != 401 {
		t.Fatalf("anonymous audio novel list status = %d", got)
	}
	admin := login(t, a)
	list := call(a, "GET", "/api/v1/audio-novels?page=1&page_size=20", "", admin)
	if list.Code != 200 || strings.Contains(list.Body.String(), `"author"`) || !strings.Contains(list.Body.String(), `"total":8`) {
		t.Fatalf("initial list: %d %s", list.Code, list.Body.String())
	}
	input := `{"title":"Test Story","slug":"test-story","category":"Fantasy","excerpt":"A concise test story.","body_markdown":"A safe paragraph.","cover_path":"","published_at":"2026-09-20","enabled":true,"featured":false}`
	created := call(a, "POST", "/api/v1/audio-novels", input, admin)
	if created.Code != 200 {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var item struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil || item.ID < 1 {
		t.Fatal("created audio novel id", item.ID, err)
	}
	if duplicate := call(a, "POST", "/api/v1/audio-novels", input, admin); duplicate.Code != 409 {
		t.Fatalf("duplicate slug status = %d", duplicate.Code)
	}
	updatedInput := strings.Replace(input, "Test Story", "Edited Story", 1)
	if updated := call(a, "PATCH", "/api/v1/audio-novels/"+itoa(item.ID), updatedInput, admin); updated.Code != 200 || !strings.Contains(updated.Body.String(), "Edited Story") {
		t.Fatalf("update: %d %s", updated.Code, updated.Body.String())
	}
	if detail := call(a, "GET", "/api/v1/audio-novels/"+itoa(item.ID), "", admin); detail.Code != 200 || !strings.Contains(detail.Body.String(), "Edited Story") {
		t.Fatalf("admin detail: %d %s", detail.Code, detail.Body.String())
	}
	if disabled := call(a, "PATCH", "/api/v1/audio-novels/"+itoa(item.ID)+"/status", `{"enabled":false}`, admin); disabled.Code != 200 {
		t.Fatalf("disable status = %d", disabled.Code)
	}
	if hidden := call(a, "GET", "/audio-novel-api/hello/stories/test-story", "", nil); hidden.Code != 404 {
		t.Fatalf("disabled story status = %d", hidden.Code)
	}
	if enabled := call(a, "PATCH", "/api/v1/audio-novels/"+itoa(item.ID)+"/status", `{"enabled":true}`, admin); enabled.Code != 200 {
		t.Fatalf("enable status = %d", enabled.Code)
	}
	featured := call(a, "PATCH", "/api/v1/audio-novels/"+itoa(item.ID)+"/featured", `{"featured":true}`, admin)
	if featured.Code != 200 || !strings.Contains(featured.Body.String(), `"featured":true`) {
		t.Fatalf("feature: %d %s", featured.Code, featured.Body.String())
	}
	var featuredCount, before, after int
	ctx := context.Background()
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM audio_novels WHERE featured AND enabled AND deleted_at IS NULL").Scan(&featuredCount); err != nil || featuredCount != 1 {
		t.Fatal("effective featured count", featuredCount, err)
	}
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM click_events").Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/audio-novel-api/hello/home", "/audio-novel-api/hello/stories", "/audio-novel-api/hello/stories/test-story"} {
		if response := call(a, "GET", path, "", nil); response.Code != 200 || strings.Contains(response.Body.String(), `"author"`) {
			t.Fatalf("public content %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM click_events").Scan(&after); err != nil || before != after {
		t.Fatal("public content API wrote click events", before, after, err)
	}
	if deleted := call(a, "DELETE", "/api/v1/audio-novels/"+itoa(item.ID), "", admin); deleted.Code != 204 {
		t.Fatalf("delete status = %d", deleted.Code)
	}
	if missing := call(a, "GET", "/audio-novel-api/hello/stories/test-story", "", nil); missing.Code != 404 {
		t.Fatalf("deleted story status = %d", missing.Code)
	}
}

func TestAudioNovelRoutesReplaceRetiredNovelRoutes(t *testing.T) {
	a := setup(t)
	for _, path := range []string{
		"/audio-novel/hello",
		"/audio-novel/hello/stories",
		"/audio-novel/hello/stories/the-glass-orchard",
	} {
		response := call(a, "GET", path, "", nil)
		if response.Code != 200 || !strings.Contains(response.Body.String(), `"surface":"audio_novel"`) {
			t.Fatalf("audio novel page %s: %d %s", path, response.Code, response.Body.String())
		}
	}

	admin := login(t, a)
	for _, path := range []string{
		"/novel/hello",
		"/novel-api/hello/home",
		"/novel-assets/missing.js",
		"/novel-uploads/missing.webp",
	} {
		if response := call(a, "GET", path, "", nil); response.Code != 404 {
			t.Fatalf("retired route %s status = %d", path, response.Code)
		}
	}
	if response := call(a, "GET", "/api/v1/novels", "", admin); response.Code != 404 {
		t.Fatalf("retired admin route status = %d", response.Code)
	}
}

func itoa(value int64) string { return strconv.FormatInt(value, 10) }
