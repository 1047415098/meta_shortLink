package tiktok

import (
	"errors"
	"testing"
	"time"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/platform/runtime"
)

func TestDeterministicEventIDs(t *testing.T) {
	if got := VisitEventID("visit_123", "StartReading"); got != "novel_visit_123_start" {
		t.Fatalf("unexpected start id %q", got)
	}
	if got := VisitEventID("visit_123", "ViewContent"); got != "novel_visit_123_qualified" {
		t.Fatalf("unexpected qualified id %q", got)
	}
}

func TestTikTokEventPayloadKeepsOriginalTimeAndNoMoneyFields(t *testing.T) {
	at := time.Unix(1_797_000_000, 0)
	request := buildVisitEventRequest(frozenVisit{
		PixelCode: "C0ABC123", NovelID: 8, TTCLID: "click-id", TTP: "cookie-id",
		Context: VisitContext{IP: "203.0.113.8", UserAgent: "browser", PageURL: "https://example.com/novel/code", Referrer: "https://tiktok.com/"},
	}, "StartReading", "novel_visit_start", at)
	if request.EventSource != "web" || request.EventSourceID != "C0ABC123" || len(request.Data) != 1 {
		t.Fatalf("request=%+v", request)
	}
	event := request.Data[0]
	if event.EventTime != at.Unix() || event.EventID != "novel_visit_start" || event.Event != "StartReading" {
		t.Fatalf("event=%+v", event)
	}
	if _, exists := event.Properties["value"]; exists {
		t.Fatalf("money value must be absent: %+v", event.Properties)
	}
	if _, exists := event.Properties["currency"]; exists {
		t.Fatalf("currency must be absent: %+v", event.Properties)
	}
}

func TestTikTokWorkerClassifiesRetryAndPermanentFailures(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		attempts   int
		wantStatus string
	}{
		{"success", nil, 1, "accepted"},
		{"timeout", &APIError{Message: "timeout", Temporary: true}, 1, "retry"},
		{"rate limit", &APIError{HTTPStatus: 429, Message: "limited", Temporary: true}, 2, "retry"},
		{"server", &APIError{HTTPStatus: 503, Message: "server", Temporary: true}, 2, "retry"},
		{"transient business", &APIError{HTTPStatus: 200, BusinessCode: 50001, Message: "busy"}, 2, "retry"},
		{"bad request", &APIError{HTTPStatus: 400, BusinessCode: 40000, Message: "bad"}, 1, "failed"},
		{"attempt limit", &APIError{HTTPStatus: 503, Message: "server", Temporary: true}, 9, "failed"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			status, _ := deliveryDecision(testCase.attempts, testCase.err)
			if status != testCase.wantStatus {
				t.Fatalf("status=%q want=%q", status, testCase.wantStatus)
			}
		})
	}
}

func TestTikTokWorkerBackoffHonorsRetryAfterAndCap(t *testing.T) {
	if delay := eventBackoff(1, &APIError{RetryAfter: 17 * time.Minute}); delay < 17*time.Minute {
		t.Fatalf("Retry-After ignored: %s", delay)
	}
	if delay := eventBackoff(20, errors.New("network")); delay > 6*time.Hour {
		t.Fatalf("backoff exceeded cap: %s", delay)
	}
}

func TestTikTokLifecycleDoesNothingWhenDisabled(t *testing.T) {
	service := New(runtime.New(config.Config{TikTokEnabled: false}, nil))
	service.Start(t.Context())
	service.Start(t.Context())
	service.lifecycle.Lock()
	started := service.cancel != nil
	service.lifecycle.Unlock()
	if started {
		t.Fatal("disabled TikTok worker started")
	}
	service.Close()
	service.Close()
}
