package requestlogs

import (
	"net/url"
	"reflect"
	"strings"
	"testing"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/platform/runtime"
)

// 验证本站的 token 提取边界，以及密钥和日志编号对解密的约束。
func TestQueryTokenStorageBoundary(t *testing.T) {
	a := &Handler{Core: &runtime.Core{Config: config.Config{Secret: "test-request-log-secret-at-least-32-characters"}}}
	want := url.Values{"token": {"first", "second"}, "TOKEN": {""}}
	input := url.Values{"token": {"first", "second"}, "TOKEN": {""}, "access_token": {"other"}}
	sealed, err := a.sealQueryToken("request-one", input)
	if err != nil || sealed == "" {
		t.Fatal("token encryption failed", err)
	}
	opened, err := a.openQueryToken("request-one", sealed)
	if err != nil || !reflect.DeepEqual(url.Values(opened), want) {
		t.Fatal("token values changed", err)
	}
	if _, err = a.openQueryToken("request-two", sealed); err == nil {
		t.Fatal("ciphertext can be swapped between request logs")
	}
	a.Config.Secret = "different-request-log-secret-at-least-32-characters"
	if _, err = a.openQueryToken("request-one", sealed); err == nil {
		t.Fatal("wrong key accepted")
	}
	for _, values := range []url.Values{
		{"campaign": {"no-token"}},
		{"token": {strings.Repeat("x", logBodyLimit)}},
	} {
		if cipher, err := a.sealQueryToken("request-three", values); err != nil || cipher != "" {
			t.Fatal("absent or oversized token should not be stored", err)
		}
	}
}
