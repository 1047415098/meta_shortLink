package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func TestAudioNovelAudioLifecycle(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	firstName := "11111111111111111111111111111111.mp3"
	secondName := "22222222222222222222222222222222.mp3"
	thirdName := "33333333333333333333333333333333.mp3"
	for _, name := range []string{firstName, secondName, thirdName} {
		if err := os.WriteFile(filepath.Join(a.Config.AudioNovelAudioDir, name), []byte("ID3test"), 0600); err != nil {
			t.Fatalf("write audio fixture: %v", err)
		}
	}
	payload := func(title, slug, name string) string {
		return fmt.Sprintf(`{"title":%q,"slug":%q,"category":"Fantasy","excerpt":"Audio lifecycle.","body_markdown":"Story.","cover_path":"","audio_path":%q,"audio_duration":"01:05","audio_size_bytes":7,"published_at":"2026-09-22","enabled":true,"featured":false}`, title, slug, "/audio-novel-audio/"+name)
	}
	created := call(a, "POST", "/api/v1/audio-novels", payload("Audio Story", "audio-story", firstName), admin)
	if created.Code != 200 || !strings.Contains(created.Body.String(), firstName) {
		t.Fatalf("create audio story: %d %s", created.Code, created.Body.String())
	}
	var item struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil {
		t.Fatalf("decode created item: %v", err)
	}

	disabledPayload := strings.Replace(payload("Disabled Audio", "disabled-audio", thirdName), `"enabled":true`, `"enabled":false`, 1)
	if disabled := call(a, "POST", "/api/v1/audio-novels", disabledPayload, admin); disabled.Code != 200 {
		t.Fatalf("create disabled audio: %d %s", disabled.Code, disabled.Body.String())
	}
	freeNovel := `{"title":"Free twin","slug":"audio-story","author":"","category":"Fantasy","excerpt":"Separate data center.","cover_path":"","published_at":"2026-09-22","enabled":true,"featured":false,"sort_order":1}`
	if response := call(a, "POST", "/api/v1/novels", freeNovel, admin); response.Code != 200 {
		t.Fatalf("create equal-slug free novel: %d %s", response.Code, response.Body.String())
	}

	audioList := call(a, "GET", "/audio-novel-api/hello/audio", "", nil)
	if audioList.Code != 200 || !strings.Contains(audioList.Body.String(), `"slug":"audio-story"`) || strings.Contains(audioList.Body.String(), `"slug":"disabled-audio"`) || strings.Contains(audioList.Body.String(), `"slug":"the-glass-orchard"`) {
		t.Fatalf("public audio filtering: %d %s", audioList.Code, audioList.Body.String())
	}
	audioDetail := call(a, "GET", "/audio-novel-api/hello/audio/audio-story", "", nil)
	if audioDetail.Code != 200 || !strings.Contains(audioDetail.Body.String(), firstName) || strings.Contains(audioDetail.Body.String(), "body_markdown") || strings.Contains(audioDetail.Body.String(), "body_html") {
		t.Fatalf("public audio detail contract: %d %s", audioDetail.Code, audioDetail.Body.String())
	}
	for _, path := range []string{"/audio-novel-api/hello/audio/disabled-audio", "/audio-novel-api/missing/audio", "/audio-novel-api/hello/audio/missing"} {
		if response := call(a, "GET", path, "", nil); response.Code != 404 {
			t.Fatalf("unavailable audio %s status = %d", path, response.Code)
		}
	}
	freeDetail := call(a, "GET", "/novel-api/hello/stories/audio-story", "", nil)
	if freeDetail.Code != 200 || strings.Contains(freeDetail.Body.String(), "audio_path") || strings.Contains(freeDetail.Body.String(), "audio_duration") || strings.Contains(freeDetail.Body.String(), "audio_size_bytes") {
		t.Fatalf("free novel leaked audio metadata: %d %s", freeDetail.Code, freeDetail.Body.String())
	}
	for _, path := range []string{"/audio-novel/hello/audio", "/audio-novel/hello/audio/audio-story"} {
		page := call(a, "GET", path, "", nil)
		if page.Code != 200 || !strings.Contains(page.Body.String(), `"surface":"audio_novel"`) || !strings.Contains(page.Header().Get("Content-Security-Policy"), "media-src 'self'") {
			t.Fatalf("audio page route %s: %d %s", path, page.Code, page.Body.String())
		}
	}

	// 失败的替换不能破坏数据库中的旧地址或仍可播放的旧文件。
	failed := call(a, "PATCH", "/api/v1/audio-novels/"+itoa(item.ID), payload("Duplicate", "the-glass-orchard", secondName), admin)
	if failed.Code != 409 {
		t.Fatalf("failed replacement status = %d %s", failed.Code, failed.Body.String())
	}
	detail := call(a, "GET", "/api/v1/audio-novels/"+itoa(item.ID), "", admin)
	if !strings.Contains(detail.Body.String(), firstName) {
		t.Fatalf("failed replacement changed stored audio: %s", detail.Body.String())
	}
	if _, err := os.Stat(filepath.Join(a.Config.AudioNovelAudioDir, firstName)); err != nil {
		t.Fatalf("failed replacement removed old audio: %v", err)
	}

	replaced := call(a, "PATCH", "/api/v1/audio-novels/"+itoa(item.ID), payload("Audio Story", "audio-story", secondName), admin)
	if replaced.Code != 200 || !strings.Contains(replaced.Body.String(), secondName) {
		t.Fatalf("replace audio: %d %s", replaced.Code, replaced.Body.String())
	}
	if _, err := os.Stat(filepath.Join(a.Config.AudioNovelAudioDir, firstName)); !os.IsNotExist(err) {
		t.Fatalf("replaced audio still exists: %v", err)
	}

	removed := call(a, "DELETE", "/api/v1/audio-novels/"+itoa(item.ID)+"/audio", "", admin)
	if removed.Code != 204 {
		t.Fatalf("remove audio status = %d %s", removed.Code, removed.Body.String())
	}
	detail = call(a, "GET", "/api/v1/audio-novels/"+itoa(item.ID), "", admin)
	if !strings.Contains(detail.Body.String(), `"audio_path":""`) {
		t.Fatalf("audio metadata was not cleared: %s", detail.Body.String())
	}
	if _, err := os.Stat(filepath.Join(a.Config.AudioNovelAudioDir, secondName)); !os.IsNotExist(err) {
		t.Fatalf("removed audio still exists: %v", err)
	}
	if missing := call(a, "DELETE", "/api/v1/audio-novels/999999/audio", "", admin); missing.Code != 404 {
		t.Fatalf("missing audio row status = %d", missing.Code)
	}

	reattached := call(a, "PATCH", "/api/v1/audio-novels/"+itoa(item.ID), payload("Audio Story", "audio-story", thirdName), admin)
	if reattached.Code != 200 {
		t.Fatalf("reattach audio: %d %s", reattached.Code, reattached.Body.String())
	}
	if deleted := call(a, "DELETE", "/api/v1/audio-novels/"+itoa(item.ID), "", admin); deleted.Code != 204 {
		t.Fatalf("delete audio story status = %d", deleted.Code)
	}
	if _, err := os.Stat(filepath.Join(a.Config.AudioNovelAudioDir, thirdName)); !os.IsNotExist(err) {
		t.Fatalf("soft-deleted story audio still exists: %v", err)
	}
	var auditCount int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM audit_logs WHERE action='audio_novel.audio_remove'").Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatalf("audio removal audit count = %d, err = %v", auditCount, err)
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

}

func itoa(value int64) string { return strconv.FormatInt(value, 10) }
