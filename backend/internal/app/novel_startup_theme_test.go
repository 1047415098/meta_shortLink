package app

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestNovelStartupAlwaysUsesCountdownTheme(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "Theme Story", "theme-story")
	chapterID := createDistributionChapter(t, a, admin, novelID, 1, "Theme Opening", true)
	connectionID, pixelID := testLinkMetaBinding(t, a)

	// 历史客户端即使提交 cover_wall，免费小说也统一保存为倒计时主题。
	created := call(a, http.MethodPost, "/api/v1/novel-links", fmt.Sprintf(`{"name":"Countdown campaign","code":"theme-link","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10,"startup_theme":"cover_wall","startup_tail_seconds":12}`, novelID, chapterID, connectionID, pixelID), admin)
	if created.Code != http.StatusOK || !strings.Contains(created.Body.String(), `"startup_theme":"countdown"`) || !strings.Contains(created.Body.String(), `"startup_tail_seconds":12`) {
		t.Fatalf("create countdown link: %d %s", created.Code, created.Body.String())
	}

	page := call(a, http.MethodGet, "/novel/theme-link", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `"startup_theme":"countdown"`) || !strings.Contains(page.Body.String(), `"startup_tail_seconds":12`) {
		t.Fatalf("countdown bootstrap: %d %s", page.Code, page.Body.String())
	}
	if strings.Contains(page.Body.String(), `"startup_theme":"cover_wall"`) {
		t.Fatalf("free novel bootstrap exposed removed cover-wall theme: %s", page.Body.String())
	}

	// 未知主题同样兼容为倒计时，避免旧管理端请求重新开启已拆分功能。
	normalized := call(a, http.MethodPost, "/api/v1/novel-links", fmt.Sprintf(`{"name":"Normalized theme","code":"theme-normalized","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10,"startup_theme":"unexpected","startup_tail_seconds":5}`, novelID, chapterID, connectionID, pixelID), admin)
	if normalized.Code != http.StatusOK || !strings.Contains(normalized.Body.String(), `"startup_theme":"countdown"`) {
		t.Fatalf("normalize historical theme: %d %s", normalized.Code, normalized.Body.String())
	}

	// 倒计时链接仍校验剩余 10% 的时长必须在 1–60 秒之间。
	invalidTail := call(a, http.MethodPost, "/api/v1/novel-links", fmt.Sprintf(`{"name":"Invalid tail","code":"tail-invalid","novel_id":%d,"entry_chapter_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10,"startup_theme":"countdown","startup_tail_seconds":61}`, novelID, chapterID, connectionID, pixelID), admin)
	if invalidTail.Code != http.StatusBadRequest {
		t.Fatalf("invalid countdown tail status=%d body=%s", invalidTail.Code, invalidTail.Body.String())
	}
}
