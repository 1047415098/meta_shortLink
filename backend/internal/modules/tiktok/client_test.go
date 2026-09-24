package tiktok

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func validRequest() EventRequest {
	return EventRequest{
		EventSource:   "web",
		EventSourceID: "C0ABC123",
		Data: []EventData{{
			Event:     "ViewContent",
			EventTime: 1_797_000_000,
			EventID:   "novel_visit_qualified",
			User:      UserContext{TTCLID: "click-id", TTP: "cookie-id", IP: "203.0.113.8", UserAgent: "browser"},
			Page:      PageContext{URL: "https://example.com/novel/code", Referrer: "https://tiktok.example/ref"},
		}},
	}
}

func TestClientPostsTokenInHeaderAndParsesSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Access-Token") != "secret-access-token" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unsafe TikTok request: method=%s token=%q content-type=%q", r.Method, r.Header.Get("Access-Token"), r.Header.Get("Content-Type"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"OK","request_id":"request-1"}`))
	}))
	defer server.Close()
	response, err := NewClient(server.URL).Post(context.Background(), "secret-access-token", validRequest())
	if err != nil || response.Code != 0 || response.RequestID != "request-1" {
		t.Fatalf("response=%+v err=%v", response, err)
	}
}

func TestClientRejectsRedirectWithoutLeakingToken(t *testing.T) {
	token := "secret-access-token"
	hitRedirectTarget := false
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hitRedirectTarget = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()
	_, err := NewClient(server.URL).Post(context.Background(), token, validRequest())
	if err == nil || strings.Contains(err.Error(), token) || hitRedirectTarget {
		t.Fatalf("expected a redacted non-followed redirect error, got %v", err)
	}
}

func TestClientClassifiesTikTokResponseFailures(t *testing.T) {
	tests := []struct {
		name, body, retryAfter string
		status                 int
		wantTemporary          bool
		wantBusinessCode       int64
	}{
		{"business failure", `{"code":40002,"message":"invalid novel_visit_qualified click-id cookie-id 203.0.113.8 browser https://example.com/novel/code https://tiktok.example/ref","request_id":"r1"}`, "", 200, false, 40002},
		{"bad request", `{"code":40000,"message":"bad request","request_id":"r2"}`, "", 400, false, 40000},
		{"rate limited", `{"code":0,"message":"limited","request_id":"r3"}`, "17", 429, true, 0},
		{"server failure", `{"code":0,"message":"failed","request_id":"r4"}`, "", 503, true, 0},
		{"invalid JSON", `{`, "", 200, true, 0},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if testCase.retryAfter != "" {
					w.Header().Set("Retry-After", testCase.retryAfter)
				}
				w.WriteHeader(testCase.status)
				_, _ = w.Write([]byte(testCase.body))
			}))
			defer server.Close()
			_, err := NewClient(server.URL).Post(context.Background(), "secret", validRequest())
			var apiError *APIError
			if !errors.As(err, &apiError) {
				t.Fatalf("expected APIError, got %v", err)
			}
			if apiError.Temporary != testCase.wantTemporary || apiError.BusinessCode != testCase.wantBusinessCode {
				t.Fatalf("error=%+v", apiError)
			}
			for _, secret := range []string{"novel_visit_qualified", "click-id", "cookie-id", "203.0.113.8", "browser", "https://example.com/novel/code", "https://tiktok.example/ref"} {
				if strings.Contains(apiError.Error(), secret) {
					t.Fatalf("sensitive event context leaked in error: %v", apiError)
				}
			}
			if testCase.status == 429 && apiError.RetryAfter != 17*time.Second {
				t.Fatalf("RetryAfter=%s", apiError.RetryAfter)
			}
		})
	}
}

func TestClientParsesHTTPDateRetryAfter(t *testing.T) {
	retryAt := time.Now().Add(10 * time.Second).UTC().Truncate(time.Second)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", retryAt.Format(http.TimeFormat))
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":0,"message":"limited","request_id":"r-date"}`))
	}))
	defer server.Close()
	_, err := NewClient(server.URL).Post(context.Background(), "secret", validRequest())
	var apiError *APIError
	if !errors.As(err, &apiError) || apiError.RetryAfter < 8*time.Second || apiError.RetryAfter > 10*time.Second {
		t.Fatalf("HTTP-date Retry-After=%s err=%v", apiError.RetryAfter, err)
	}
}

func TestClientRejectsOversizedAndTimedOutResponses(t *testing.T) {
	t.Run("oversized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("x", (1<<20)+1)))
		}))
		defer server.Close()
		_, err := NewClient(server.URL).Post(context.Background(), "secret", validRequest())
		if err == nil {
			t.Fatal("oversized response was accepted")
		}
	})
	t.Run("timeout", func(t *testing.T) {
		release := make(chan struct{})
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			<-release
		}))
		defer server.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		_, err := NewClient(server.URL).Post(ctx, "secret", validRequest())
		// Release the controlled slow handler before httptest closes its listener.
		close(release)
		var apiError *APIError
		if !errors.As(err, &apiError) || !apiError.Temporary {
			t.Fatalf("timeout error=%v", err)
		}
	})
}
