package meta

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGraphErrorRedactionPreservesUTF8(t *testing.T) {
	message := cleanMessage("x"+strings.Repeat("无权限", 500)+"secret", "secret")
	if !utf8.ValidString(message) || len(message) > 1200 || strings.Contains(message, "secret") {
		t.Fatal("bounded error must be valid UTF-8 and redact credentials")
	}
}

func TestGraphClientKeepsCredentialOutOfURLAndSanitizesErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-value" || strings.Contains(r.URL.String(), "secret-value") {
			t.Error("unsafe credential transport")
		}
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"message":"bad secret-value","code":190,"fbtrace_id":"trace"}}`))
	}))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	var out map[string]any
	// A generic CAPI POST verifies secret transport without exposing the token.
	e := c.Post(context.Background(), "v26.0", "secret-value", "123/events", map[string]any{"data": []any{}}, &out)
	if e == nil || strings.Contains(e.Error(), "secret-value") || retryable(e) {
		t.Fatalf("invalid auth error: %v", e)
	}
}
func TestGraphClientDoesNotFollowRedirect(t *testing.T) {
	hit := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) }))
	defer srv.Close()
	c := NewClient()
	c.BaseURL = srv.URL
	var out map[string]any
	if e := c.Post(context.Background(), "v26.0", "secret", "123/events", map[string]any{"data": []any{}}, &out); e == nil {
		t.Fatal("redirect accepted")
	}
	if hit {
		t.Fatal("credential-bearing request followed redirect")
	}
}
