package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNovelTranslationAdminRequiresSelectionAndConfiguredServerCredentials(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	created := call(a, "POST", "/api/v1/novels", `{"title":"Story","slug":"story","author":"Nine","category":"Drama","excerpt":"Excerpt","cover_path":"","published_at":"2026-09-22","enabled":true,"featured":false,"sort_order":0}`, admin)
	if created.Code != 200 {
		t.Fatalf("create novel: %d %s", created.Code, created.Body.String())
	}
	var novel struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &novel); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/novels/" + itoa(novel.ID) + "/translations"
	list := call(a, "GET", path, "", admin)
	if list.Code != 200 || !strings.Contains(list.Body.String(), `"locale":"ja"`) || !strings.Contains(list.Body.String(), `"status":"not_generated"`) {
		t.Fatalf("translation list: %d %s", list.Code, list.Body.String())
	}
	if invalid := call(a, "POST", path, `{"locales":["en"]}`, admin); invalid.Code != 400 {
		t.Fatalf("invalid locale status = %d", invalid.Code)
	}
	if missing := call(a, "POST", path, `{"locales":["ja"]}`, admin); missing.Code != 503 {
		t.Fatalf("missing credentials status = %d body=%s", missing.Code, missing.Body.String())
	}
}
