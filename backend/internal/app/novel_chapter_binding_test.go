package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func createDistributionChapter(t *testing.T, a *App, admin *http.Cookie, novelID int64, number int, title string, enabled bool) int64 {
	t.Helper()
	body := fmt.Sprintf(`{"chapter_number":%d,"title":%q,"body_markdown":"Chapter body","enabled":%t}`, number, title, enabled)
	response := call(a, http.MethodPost, "/api/v1/novels/"+itoa(novelID)+"/chapters", body, admin)
	if response.Code != http.StatusOK {
		t.Fatalf("create chapter %d: %d %s", number, response.Code, response.Body.String())
	}
	var item struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	return item.ID
}

func TestNovelDistributionBindsFreezesAndValidatesEntryChapter(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "Chapter Campaign", "chapter-campaign")
	otherNovelID := createDistributionNovel(t, a, admin, "Other Campaign", "other-campaign")
	firstChapterID := createDistributionChapter(t, a, admin, novelID, 1, "Opening", true)
	entryChapterID := createDistributionChapter(t, a, admin, novelID, 2, "Paid Entry", true)
	otherChapterID := createDistributionChapter(t, a, admin, otherNovelID, 1, "Other Entry", true)
	otherSameNumberChapterID := createDistributionChapter(t, a, admin, otherNovelID, 2, "Other Paid Entry", true)
	disabledChapterID := createDistributionChapter(t, a, admin, novelID, 3, "Disabled Entry", false)
	connectionID, pixelID := testLinkMetaBinding(t, a)
	ordinaryBody := fmt.Sprintf(`{"code":"ordinary-entry","name":"Ordinary","target_url":"https://wa.me/13365661092","enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"entry_chapter_id":%d}`, connectionID, pixelID, entryChapterID)
	if response := call(a, http.MethodPost, "/api/v1/links", ordinaryBody, admin); response.Code != http.StatusBadRequest {
		t.Fatalf("ordinary link accepted entry chapter: %d %s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), `INSERT INTO short_links(code,name,target_url,product_type,entry_chapter_id)
		VALUES('ordinary-entry-db','Ordinary DB','https://wa.me/13365661092','short_link',$1)`, entryChapterID); err == nil {
		t.Fatal("database accepted an entry chapter on an ordinary short link")
	}

	base := fmt.Sprintf(`{"name":"Chapter Link","code":"chapter-link","novel_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, connectionID, pixelID)
	if response := call(a, http.MethodPost, "/api/v1/novel-links", base, admin); response.Code != http.StatusBadRequest {
		t.Fatalf("missing entry chapter status=%d body=%s", response.Code, response.Body.String())
	}
	for _, chapterID := range []int64{otherChapterID, disabledChapterID} {
		body := strings.TrimSuffix(base, "}") + fmt.Sprintf(`,"entry_chapter_id":%d}`, chapterID)
		if response := call(a, http.MethodPost, "/api/v1/novel-links", body, admin); response.Code != http.StatusBadRequest {
			t.Fatalf("invalid entry chapter %d status=%d body=%s", chapterID, response.Code, response.Body.String())
		}
	}

	createBody := strings.TrimSuffix(base, "}") + fmt.Sprintf(`,"entry_chapter_id":%d}`, entryChapterID)
	created := call(a, http.MethodPost, "/api/v1/novel-links", createBody, admin)
	if created.Code != http.StatusOK || !strings.Contains(created.Body.String(), `"entry_chapter_number":2`) || !strings.Contains(created.Body.String(), `"entry_chapter_title":"Paid Entry"`) {
		t.Fatalf("create bound link: %d %s", created.Code, created.Body.String())
	}
	var link struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	// The database repeats the ownership invariant so future write paths cannot
	// attach a chapter from another novel by bypassing this API.
	if _, err := a.DB.Exec(context.Background(), "UPDATE short_links SET entry_chapter_id=$1 WHERE id=$2", otherChapterID, link.ID); err == nil {
		t.Fatal("database accepted a cross-novel entry chapter binding")
	}

	// Keeping the original binding is an ordinary settings edit even if the
	// unpublished chapter temporarily makes the public link unavailable.
	if _, err := a.DB.Exec(context.Background(), "UPDATE novel_chapters SET enabled=false WHERE id=$1", entryChapterID); err != nil {
		t.Fatal(err)
	}
	pausedBody := fmt.Sprintf(`{"name":"Paused Chapter Link","novel_id":%d,"entry_chapter_id":%d,"enabled":false,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, entryChapterID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(link.ID), pausedBody, admin); response.Code != http.StatusOK {
		t.Fatalf("keep disabled unused chapter binding: %d %s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE novel_chapters SET enabled=true,deleted_at=now() WHERE id=$1", entryChapterID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(link.ID), pausedBody, admin); response.Code != http.StatusOK {
		t.Fatalf("keep deleted unused chapter binding: %d %s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE novel_chapters SET deleted_at=NULL WHERE id=$1", entryChapterID); err != nil {
		t.Fatal(err)
	}

	// An unused link can move to another readable chapter before attribution is frozen.
	editBody := fmt.Sprintf(`{"name":"Chapter Link","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, firstChapterID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(link.ID), editBody, admin); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"entry_chapter_number":1`) {
		t.Fatalf("edit unused chapter binding: %d %s", response.Code, response.Body.String())
	}
	editBody = fmt.Sprintf(`{"name":"Chapter Link","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, entryChapterID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(link.ID), editBody, admin); response.Code != http.StatusOK {
		t.Fatalf("restore unused chapter binding: %d %s", response.Code, response.Body.String())
	}

	page := call(a, http.MethodGet, "/novel/chapter-link", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `"entry_chapter_number":2`) {
		t.Fatalf("bound chapter bootstrap: %d %s", page.Code, page.Body.String())
	}
	var visitID string
	var frozenID *int64
	var frozenNumber *int
	var frozenTitle string
	if err := a.DB.QueryRow(context.Background(), `SELECT id,entry_chapter_id,entry_chapter_number,entry_chapter_title
		FROM click_events WHERE link_id=$1 AND method='GET' AND classification='normal' ORDER BY occurred_at DESC LIMIT 1`, link.ID).
		Scan(&visitID, &frozenID, &frozenNumber, &frozenTitle); err != nil || frozenID == nil || *frozenID != entryChapterID || frozenNumber == nil || *frozenNumber != 2 || frozenTitle != "Paid Entry" {
		t.Fatalf("entry chapter snapshot id=%v number=%v title=%q err=%v", frozenID, frozenNumber, frozenTitle, err)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET entry_chapter_id=$1 WHERE id=$2", otherChapterID, visitID); err == nil {
		t.Fatal("database accepted a cross-novel visit chapter snapshot")
	}
	// The frozen identity is one atomic snapshot: partial NULL combinations
	// must not bypass the composite ownership foreign key.
	for label, statement := range map[string]string{
		"missing novel":  "UPDATE click_events SET novel_id=NULL WHERE id=$1",
		"missing id":     "UPDATE click_events SET entry_chapter_id=NULL WHERE id=$1",
		"missing number": "UPDATE click_events SET entry_chapter_number=NULL WHERE id=$1",
	} {
		tx, beginErr := a.DB.Begin(context.Background())
		if beginErr != nil {
			t.Fatal(beginErr)
		}
		_, updateErr := tx.Exec(context.Background(), statement, visitID)
		_ = tx.Rollback(context.Background())
		if updateErr == nil {
			t.Fatalf("database accepted incomplete visit chapter snapshot: %s", label)
		}
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE novel_chapters SET title='Renamed Paid Entry' WHERE id=$1", entryChapterID); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT entry_chapter_title FROM click_events WHERE id=$1", visitID).Scan(&frozenTitle); err != nil || frozenTitle != "Paid Entry" {
		t.Fatalf("chapter rename rewrote visit snapshot: title=%q err=%v", frozenTitle, err)
	}

	// The first normal GET locks both the novel and chapter; ordinary settings stay editable.
	lockedBody := fmt.Sprintf(`{"name":"Chapter Link","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":20}`, novelID, firstChapterID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(link.ID), lockedBody, admin); response.Code != http.StatusConflict {
		t.Fatalf("visited chapter rebinding status=%d body=%s", response.Code, response.Body.String())
	}
	settingsBody := fmt.Sprintf(`{"name":"Renamed Chapter Link","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":20}`, novelID, entryChapterID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(link.ID), settingsBody, admin); response.Code != http.StatusOK {
		t.Fatalf("edit locked link settings: %d %s", response.Code, response.Body.String())
	}

	ticket := visitID + "." + a.Sign("contact:novel:chapter-link:"+visitID)
	if response := postNovelForm(a, "/novel/chapter-link/start-reading", url.Values{"ticket": {ticket}, "novel_id": {itoa(novelID)}, "chapter_id": {itoa(entryChapterID)}, "chapter": {"1"}}); response.Code != http.StatusBadRequest {
		t.Fatalf("wrong entry chapter start status=%d body=%s", response.Code, response.Body.String())
	}
	// A chapter number is not an identity: another novel can have the same
	// number, but it must never promote this visit's frozen attribution.
	wrongIdentity := url.Values{"ticket": {ticket}, "novel_id": {itoa(otherNovelID)}, "chapter_id": {itoa(otherSameNumberChapterID)}, "chapter": {"2"}}
	if response := postNovelForm(a, "/novel/chapter-link/start-reading", wrongIdentity); response.Code != http.StatusBadRequest {
		t.Fatalf("cross-novel chapter identity status=%d body=%s", response.Code, response.Body.String())
	}
	validStart := url.Values{"ticket": {ticket}, "novel_id": {itoa(novelID)}, "chapter_id": {itoa(entryChapterID)}, "chapter": {"2"}}
	if response := postNovelForm(a, "/novel/chapter-link/start-reading", validStart); response.Code != http.StatusOK {
		t.Fatalf("bound entry chapter start status=%d body=%s", response.Code, response.Body.String())
	}

	// A disabled or soft-deleted entry chapter makes the link unavailable until restored.
	if _, err := a.DB.Exec(context.Background(), "UPDATE novel_chapters SET enabled=false WHERE id=$1", entryChapterID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, http.MethodGet, "/novel/chapter-link", "", nil); response.Code != http.StatusGone {
		t.Fatalf("disabled entry status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE novel_chapters SET enabled=true,deleted_at=now() WHERE id=$1", entryChapterID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, http.MethodGet, "/novel/chapter-link", "", nil); response.Code != http.StatusGone {
		t.Fatalf("deleted entry status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE novel_chapters SET deleted_at=NULL WHERE id=$1", entryChapterID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, http.MethodGet, "/novel/chapter-link", "", nil); response.Code != http.StatusOK {
		t.Fatalf("restored entry status=%d body=%s", response.Code, response.Body.String())
	}

	stats := call(a, http.MethodPost, "/api/v1/novel-links/"+itoa(link.ID)+"/stats", `{"tz":"Etc/GMT+8","page":1}`, admin)
	if stats.Code != http.StatusOK || !strings.Contains(stats.Body.String(), `"entry_chapter_number":2`) {
		t.Fatalf("chapter snapshot stats: %d %s", stats.Code, stats.Body.String())
	}
	var report struct {
		Link struct {
			EntryChapterTitle string `json:"entry_chapter_title"`
		} `json:"link"`
		Items []struct {
			ID                string `json:"id"`
			EntryChapterTitle string `json:"entry_chapter_title"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stats.Body.Bytes(), &report); err != nil || report.Link.EntryChapterTitle != "Renamed Paid Entry" {
		t.Fatalf("current chapter metadata missing from stats: %s err=%v", stats.Body.String(), err)
	}
	var historicalSnapshotFound bool
	for _, item := range report.Items {
		if item.ID == visitID && item.EntryChapterTitle == "Paid Entry" {
			historicalSnapshotFound = true
		}
	}
	if !historicalSnapshotFound {
		t.Fatalf("historical visit chapter title was not frozen: %s", stats.Body.String())
	}
}

func TestLegacyNovelLinkKeepsNullableIntroductionEntry(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "Legacy Story", "legacy-story")
	entryChapterID := createDistributionChapter(t, a, admin, novelID, 1, "Legacy Opening", true)
	connectionID, pixelID := testLinkMetaBinding(t, a)

	var visitedID, unusedID int64
	if err := a.DB.QueryRow(context.Background(), `INSERT INTO short_links(code,name,target_url,enabled,channel,meta_connection_id,meta_pixel_id,product_type,novel_id,first_visited_at)
		VALUES('legacy-chapter-entry','Legacy visited','',true,'facebook',$1,$2,'novel',$3,now()) RETURNING id`, connectionID, pixelID, novelID).Scan(&visitedID); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(context.Background(), `INSERT INTO short_links(code,name,target_url,enabled,channel,meta_connection_id,meta_pixel_id,product_type,novel_id)
		VALUES('legacy-unused-entry','Legacy unused','',true,'facebook',$1,$2,'novel',$3) RETURNING id`, connectionID, pixelID, novelID).Scan(&unusedID); err != nil {
		t.Fatal(err)
	}
	legacyBody := fmt.Sprintf(`{"name":"Legacy renamed","novel_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(visitedID), legacyBody, admin); response.Code != http.StatusOK {
		t.Fatalf("edit visited legacy link settings: %d %s", response.Code, response.Body.String())
	}
	bindVisitedBody := fmt.Sprintf(`{"name":"Legacy visited","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, entryChapterID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(visitedID), bindVisitedBody, admin); response.Code != http.StatusConflict {
		t.Fatalf("visited legacy link accepted a new chapter binding: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(unusedID), legacyBody, admin); response.Code != http.StatusOK {
		t.Fatalf("unused legacy link could not keep introduction entry: %d %s", response.Code, response.Body.String())
	}
	disabledLegacyBody := fmt.Sprintf(`{"name":"Legacy paused","novel_id":%d,"enabled":false,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(unusedID), disabledLegacyBody, admin); response.Code != http.StatusOK {
		t.Fatalf("unused legacy introduction link could not be paused: %d %s", response.Code, response.Body.String())
	}
	bindUnusedBody := fmt.Sprintf(`{"name":"Legacy bound","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, entryChapterID, connectionID, pixelID)
	if response := call(a, http.MethodPatch, "/api/v1/novel-links/"+itoa(unusedID), bindUnusedBody, admin); response.Code != http.StatusOK {
		t.Fatalf("unused legacy link could not adopt an entry chapter: %d %s", response.Code, response.Body.String())
	}
	page := call(a, http.MethodGet, "/novel/legacy-chapter-entry", "", nil)
	if page.Code != http.StatusOK || strings.Contains(page.Body.String(), `"entry_chapter_number"`) {
		t.Fatalf("legacy introduction bootstrap: %d %s", page.Code, page.Body.String())
	}
	var visitID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM click_events WHERE link_id=$1 AND method='GET' AND classification='normal' ORDER BY occurred_at DESC LIMIT 1", visitedID).Scan(&visitID); err != nil {
		t.Fatal(err)
	}
	ticket := visitID + "." + a.Sign("contact:novel:legacy-chapter-entry:"+visitID)
	if response := postNovelForm(a, "/novel/legacy-chapter-entry/start-reading", url.Values{"ticket": {ticket}, "novel_id": {itoa(novelID)}, "chapter_id": {itoa(entryChapterID)}, "chapter": {"1"}}); response.Code != http.StatusOK {
		t.Fatalf("legacy first chapter start: %d %s", response.Code, response.Body.String())
	}
}
