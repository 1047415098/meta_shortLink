package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestFreeNovelContentFlow(t *testing.T) {
	a := setup(t)
	if got := call(a, "GET", "/api/v1/novels", "", nil).Code; got != 401 {
		t.Fatalf("anonymous novel list status = %d", got)
	}
	admin := login(t, a)
	input := `{"title":"Test Story","slug":"test-story","author":"Nine","category":"Drama","excerpt":"A concise story.","cover_path":"","published_at":"2026-09-21","enabled":true,"featured":true,"sort_order":10}`
	created := call(a, "POST", "/api/v1/novels", input, admin)
	if created.Code != 200 {
		t.Fatalf("create novel: %d %s", created.Code, created.Body.String())
	}
	var item struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &item); err != nil || item.ID < 1 {
		t.Fatalf("created novel id = %d, %v", item.ID, err)
	}
	if duplicate := call(a, "POST", "/api/v1/novels", input, admin); duplicate.Code != 409 {
		t.Fatalf("duplicate slug status = %d", duplicate.Code)
	}
	chapterInput := `{"chapter_number":1,"title":"Prologue","body_markdown":"A **safe** paragraph.","enabled":true}`
	chapter := call(a, "POST", "/api/v1/novels/"+itoa(item.ID)+"/chapters", chapterInput, admin)
	if chapter.Code != 200 {
		t.Fatalf("create chapter: %d %s", chapter.Code, chapter.Body.String())
	}
	if duplicate := call(a, "POST", "/api/v1/novels/"+itoa(item.ID)+"/chapters", chapterInput, admin); duplicate.Code != 409 {
		t.Fatalf("duplicate chapter status = %d", duplicate.Code)
	}
	var before, after int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events").Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/novel-api/hello/home",
		"/novel-api/hello/stories?q=Nine",
		"/novel-api/hello/stories/test-story",
		"/novel-api/hello/stories/test-story/chapters/1",
	} {
		response := call(a, "GET", path, "", nil)
		if response.Code != 200 {
			t.Fatalf("public content %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	chapterResponse := call(a, "GET", "/novel-api/hello/stories/test-story/chapters/1", "", nil)
	var publicChapter struct {
		Chapter struct {
			BodyHTML     string `json:"body_html"`
			BodyMarkdown string `json:"body_markdown"`
		} `json:"chapter"`
	}
	// JSON may escape HTML characters on the wire; validate the decoded public contract instead.
	if err := json.Unmarshal(chapterResponse.Body.Bytes(), &publicChapter); err != nil || !strings.Contains(publicChapter.Chapter.BodyHTML, "<strong>safe</strong>") || publicChapter.Chapter.BodyMarkdown != "" {
		t.Fatalf("unsafe chapter contract: %s", chapterResponse.Body.String())
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events").Scan(&after); err != nil || before != after {
		t.Fatalf("content API wrote visits: %d -> %d, %v", before, after, err)
	}
	page := call(a, "GET", "/novel/hello/stories/test-story", "", nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), `"surface":"novel"`) {
		t.Fatalf("novel page: %d %s", page.Code, page.Body.String())
	}
	if !strings.Contains(page.Body.String(), `"ticket":"`) {
		t.Fatalf("novel page did not include a signed visit ticket: %s", page.Body.String())
	}
	if invalid := call(a, "POST", "/novel/hello/view", "", nil); invalid.Code != 400 {
		t.Fatalf("invalid novel view ticket status = %d", invalid.Code)
	}
}
