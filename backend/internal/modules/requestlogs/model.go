package requestlogs

import (
	"encoding/json"
	"time"
)

type RequestLog struct {
	// 详情标记原值是否可用，避免将历史脱敏值误认为仍可恢复。
	QueryTokenState string          `json:"query_token_state,omitempty"`
	Trigger         string          `json:"trigger"`
	ID              string          `json:"id"`
	OccurredAt      time.Time       `json:"occurred_at"`
	Method          string          `json:"method"`
	Path            string          `json:"path"`
	ClientIP        string          `json:"client_ip"`
	Status          int             `json:"status"`
	DurationMS      float64         `json:"duration_ms"`
	RequestHeaders  json.RawMessage `json:"request_headers,omitempty"`
	Query           json.RawMessage `json:"query,omitempty"`
	RequestBody     json.RawMessage `json:"request_body,omitempty"`
	ResponseHeaders json.RawMessage `json:"response_headers,omitempty"`
	ResponseBody    json.RawMessage `json:"response_body,omitempty"`
	RequestBytes    int64           `json:"request_bytes"`
	ResponseBytes   int64           `json:"response_bytes"`
}

const logCols = "id,occurred_at,method,path,client_ip,status,duration_ms,request_bytes,response_bytes,coalesce(request_body->'trigger'->>0,'manual')"
