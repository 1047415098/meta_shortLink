package novel

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequestLocalePrefersLanguageHeaderAndKeepsLegacyQueryFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{}

	request := httptest.NewRequest("GET", "/novel-api/demo/home?lang=th", nil)
	request.Header.Set("X-Novel-Language", "ja")
	request.AddCookie(&http.Cookie{Name: LanguageCookieName, Value: "ko"})
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	if got := handler.requestLocale(context); got != "ja" {
		t.Fatalf("header locale = %s, want ja", got)
	}

	legacy := httptest.NewRequest("GET", "/novel-api/demo/home?lang=th", nil)
	legacy.AddCookie(&http.Cookie{Name: LanguageCookieName, Value: "ko"})
	context, _ = gin.CreateTestContext(httptest.NewRecorder())
	context.Request = legacy
	if got := handler.requestLocale(context); got != "th" {
		t.Fatalf("legacy query locale = %s, want th", got)
	}
}
