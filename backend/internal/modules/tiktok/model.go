package tiktok

import (
	"context"
	"sync"

	"whatsapp-analytics/internal/platform/runtime"
)

type UserContext struct {
	TTCLID    string `json:"ttclid,omitempty"`
	TTP       string `json:"ttp,omitempty"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

type PageContext struct {
	URL      string `json:"url"`
	Referrer string `json:"referrer,omitempty"`
}

// VisitContext contains the private request data needed for later server-side
// TikTok delivery. It is encrypted before the visit row is inserted.
type VisitContext struct {
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	PageURL   string `json:"page_url"`
	Referrer  string `json:"referrer"`
}

type Content struct {
	ContentID string `json:"content_id"`
	Quantity  int    `json:"quantity"`
}

type EventData struct {
	Event      string         `json:"event"`
	EventTime  int64          `json:"event_time"`
	EventID    string         `json:"event_id"`
	User       UserContext    `json:"user"`
	Page       PageContext    `json:"page"`
	Properties map[string]any `json:"properties,omitempty"`
}

type EventRequest struct {
	EventSource   string      `json:"event_source"`
	EventSourceID string      `json:"event_source_id"`
	Data          []EventData `json:"data"`
	TestEventCode string      `json:"test_event_code,omitempty"`
}

type DeliveryResponse struct {
	Code      int64  `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

type Service struct {
	Core      *runtime.Core
	Client    *Client
	lifecycle sync.Mutex
	cancel    context.CancelFunc
	closed    bool
	workers   sync.WaitGroup
}

func New(core *runtime.Core) *Service {
	return &Service{Core: core, Client: NewClient(core.Config.TikTokEventsURL)}
}
