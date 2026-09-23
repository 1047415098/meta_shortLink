package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/novel"
)

func TestJapaneseVisitorFallsBackToEnglishWhenNovelHasNoJapaneseTranslation(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "English Only Story", "english-only-story")
	chapter := call(a, "POST", "/api/v1/novels/"+itoa(novelID)+"/chapters", `{"chapter_number":1,"title":"Opening","body_markdown":"English chapter body","enabled":true}`, admin)
	if chapter.Code != 200 {
		t.Fatalf("create chapter: %d %s", chapter.Code, chapter.Body.String())
	}
	connectionID, pixelID := testLinkMetaBinding(t, a)
	linkBody := fmt.Sprintf(`{"name":"日本无译文投放","code":"japan-english-fallback","novel_id":%d,"enabled":true,"channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":0}`, novelID, connectionID, pixelID)
	if response := call(a, "POST", "/api/v1/novel-links", linkBody, admin); response.Code != 200 {
		t.Fatalf("create distribution link: %d %s", response.Code, response.Body.String())
	}
	distributionLink, err := (links.Repository{DB: a.DB}).ByCode(context.Background(), "japan-english-fallback")
	if err != nil {
		t.Fatal(err)
	}

	pageRequest := httptest.NewRequest("GET", "/novel/japan-english-fallback", nil)
	pageResponse := httptest.NewRecorder()
	pageContext, _ := gin.CreateTestContext(pageResponse)
	pageContext.Request = pageRequest
	// Tracking resolves the IP to JP before rendering; the renderer must limit that choice to this novel's published languages.
	(&novel.Handler{Core: a.Core}).Render(pageContext, distributionLink, "", false, "JP")
	if pageResponse.Code != 200 || pageResponse.Header().Get("Content-Language") != "en" || !strings.Contains(pageResponse.Body.String(), `"locale":"en"`) || !strings.Contains(pageResponse.Body.String(), `"available_locales":["en"]`) {
		t.Fatalf("Japanese visitor fallback page: %d %s %s", pageResponse.Code, pageResponse.Header().Get("Content-Language"), pageResponse.Body.String())
	}

	chapterRequest := httptest.NewRequest("GET", "/novel-api/japan-english-fallback/stories/english-only-story/chapters/1", nil)
	chapterRequest.Header.Set(novel.LanguageHeaderName, "en")
	chapterResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(chapterResponse, chapterRequest)
	if chapterResponse.Code != 200 || chapterResponse.Header().Get("Content-Language") != "en" || !strings.Contains(chapterResponse.Body.String(), "English chapter body") {
		t.Fatalf("English chapter fallback: %d %s %s", chapterResponse.Code, chapterResponse.Header().Get("Content-Language"), chapterResponse.Body.String())
	}
}

func TestPublishedNovelTranslationLocalizesContentAndFallsBackToEnglish(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "English Story", "english-story")
	chapter := call(a, "POST", "/api/v1/novels/"+itoa(novelID)+"/chapters", `{"chapter_number":1,"title":"Start","body_markdown":"English body","enabled":true}`, admin)
	if chapter.Code != 200 {
		t.Fatalf("create chapter: %d %s", chapter.Code, chapter.Body.String())
	}
	var chapterItem struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(chapter.Body.Bytes(), &chapterItem); err != nil {
		t.Fatal(err)
	}
	var revision int64
	if err := a.DB.QueryRow(context.Background(), "SELECT source_revision FROM novels WHERE id=$1", novelID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	var versionID int64
	if err := a.DB.QueryRow(context.Background(), `INSERT INTO novel_translation_versions(novel_id,locale,source_revision,title,author,category,excerpt,status,enabled,total_items,completed_items,finished_at,published_at) VALUES($1,'ja',$2,'日本語の物語','ナイン','ドラマ','日本語の概要','published',true,2,2,now(),now()) RETURNING id`, novelID, revision).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), `INSERT INTO novel_chapter_translations(version_id,chapter_id,chapter_number,title,body_markdown) VALUES($1,$2,1,'始まり','日本語の本文')`, versionID, chapterItem.ID); err != nil {
		t.Fatal(err)
	}
	story := call(a, "GET", "/novel-api/hello/stories/english-story?lang=ja", "", nil)
	if story.Code != 200 || !strings.Contains(story.Body.String(), "日本語の物語") || !strings.Contains(story.Body.String(), "始まり") {
		t.Fatalf("localized story: %d %s", story.Code, story.Body.String())
	}
	translated := call(a, "GET", "/novel-api/hello/stories/english-story/chapters/1?lang=ja", "", nil)
	if translated.Code != 200 || !strings.Contains(translated.Body.String(), "日本語の本文") {
		t.Fatalf("localized chapter: %d %s", translated.Code, translated.Body.String())
	}
	fallback := call(a, "GET", "/novel-api/hello/stories/english-story?lang=ko", "", nil)
	if fallback.Code != 200 || !strings.Contains(fallback.Body.String(), "English Story") || !strings.Contains(fallback.Body.String(), "Start") {
		t.Fatalf("English fallback: %d %s", fallback.Code, fallback.Body.String())
	}
	fallbackChapter := call(a, "GET", "/novel-api/hello/stories/english-story/chapters/1?lang=ko", "", nil)
	if fallbackChapter.Code != 200 || !strings.Contains(fallbackChapter.Body.String(), "English body") {
		t.Fatalf("English chapter fallback: %d %s", fallbackChapter.Code, fallbackChapter.Body.String())
	}
	search := call(a, "GET", "/novel-api/hello/stories?q="+"%E6%97%A5%E6%9C%AC%E8%AA%9E"+"&lang=ja", "", nil)
	if search.Code != 200 || !strings.Contains(search.Body.String(), "日本語の物語") {
		t.Fatalf("localized search: %d %s", search.Code, search.Body.String())
	}

	connectionID, pixelID := testLinkMetaBinding(t, a)
	linkBody := fmt.Sprintf(`{"name":"日本投放","code":"ja-story","novel_id":%d,"enabled":true,"channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":0}`, novelID, connectionID, pixelID)
	if response := call(a, "POST", "/api/v1/novel-links", linkBody, admin); response.Code != 200 {
		t.Fatalf("create distribution link: %d %s", response.Code, response.Body.String())
	}
	page := call(a, "GET", "/novel/ja-story?lang=ja", "", nil)
	if page.Code != 200 || !strings.Contains(page.Body.String(), `"locale":"ja"`) || !strings.Contains(page.Body.String(), `"available_locales":["en","ja"]`) {
		t.Fatalf("localized bootstrap: %d %s", page.Code, page.Body.String())
	}
	var languageCookieFound bool
	for _, cookie := range page.Result().Cookies() {
		if cookie.Name == "novel_lang" && cookie.Value == "ja" && !cookie.HttpOnly {
			languageCookieFound = true
		}
	}
	if !languageCookieFound {
		t.Fatal("localized bootstrap did not persist the browser-readable language cookie")
	}
}
