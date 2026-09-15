package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// 管理员需要看到 URL token 原值，但数据库、列表及其他敏感字段不能因此暴露明文。
func TestRequestLogQueryTokenReadableOnlyInAuthenticatedDetail(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	w := call(a, "GET", "/hello?token=first-token&token=second-token&access_token=other-secret&campaign=test", "", nil)
	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("request log ID missing")
	}
	if denied := call(a, "GET", "/api/v1/request-logs/"+id, "", nil); denied.Code != 401 || strings.Contains(denied.Body.String(), "first-token") {
		t.Fatal("anonymous detail exposes token")
	}
	w = call(a, "GET", "/api/v1/request-logs/"+id, "", admin)
	var detail struct {
		Query      map[string]any `json:"query"`
		TokenState string         `json:"query_token_state"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil || w.Code != 200 {
		t.Fatal("detail request failed", err, w.Code)
	}
	values, ok := detail.Query["token"].([]any)
	if !ok || len(values) != 2 || values[0] != "first-token" || values[1] != "second-token" || detail.TokenState != "available" {
		t.Fatal("administrator cannot view the original repeated token values")
	}
	if detail.Query["access_token"] != "[REDACTED]" {
		t.Fatal("unrelated credential redaction changed")
	}
	var query, encrypted string
	if err := a.DB.QueryRow(context.Background(), "SELECT query::text,query_token_cipher FROM request_logs WHERE id=$1", id).Scan(&query, &encrypted); err != nil {
		t.Fatal(err)
	}
	if encrypted == "" || strings.Contains(query+encrypted, "first-token") || strings.Contains(query+encrypted, "second-token") {
		t.Fatal("request log token was not encrypted at rest")
	}
	listed := call(a, "GET", "/api/v1/request-logs", "", admin)
	if strings.Contains(listed.Body.String(), "first-token") || strings.Contains(listed.Body.String(), encrypted) {
		t.Fatal("list endpoint exposes token material")
	}
	var audits int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM audit_logs WHERE action='request_log.token_view' AND detail->>'request_id'=$1", id).Scan(&audits); err != nil || audits != 1 {
		t.Fatal("token view was not audited", err, audits)
	}
}

func TestRequestLogsCaptureAndRedact(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	r := httptest.NewRequest("POST", "/hello/contact?token=query-secret&campaign=test", strings.NewReader(`{"payload":"Log example","password":"body-secret","ticket":"ticket-secret"}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer header-secret")
	// Browser headers use percent-encoded UTF-8, but administrators need readable campaign names.
	r.Header.Set("X-Meta-Utm-Campaign", "%E5%B9%BF%E5%91%8A%E7%B3%BB%E5%88%97")
	r.AddCookie(admin)
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal(w.Body.String())
	}
	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Fatal("visitor request trace missing")
	}
	w = call(a, "GET", "/api/v1/request-logs/"+id, "", admin)
	if w.Code != 200 {
		t.Fatalf("log detail %d %s", w.Code, w.Body.String())
	}
	// URL token 按管理员授权展示；正文、认证头和登录 Cookie 仍保持脱敏。
	for _, secret := range []string{"body-secret", "header-secret", "ticket-secret", admin.Value} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("secret leaked")
		}
	}
	var row map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &row); err != nil {
		t.Fatal(err)
	}
	if row["status"] != float64(400) || row["method"] != "POST" || !strings.Contains(w.Body.String(), "Log example") || !strings.Contains(w.Body.String(), "campaign") {
		t.Fatal(w.Body.String())
	}
	headers, ok := row["request_headers"].(map[string]any)
	values, valuesOK := headers["X-Meta-Utm-Campaign"].([]any)
	if !ok || !valuesOK || len(values) != 1 || values[0] != "广告系列" {
		t.Fatalf("Meta campaign header is not readable: %#v", row["request_headers"])
	}
	if !strings.Contains(w.Body.String(), "query-secret") {
		t.Fatal("URL token not visible to administrator")
	}
	if call(a, "GET", "/api/v1/request-logs/"+id, "", nil).Code != 401 {
		t.Fatal("anonymous log access")
	}
}

// 历史日志和损坏的密文不能恢复原值，也不能妨碍其余详情的读取。
func TestRequestLogQueryTokenUnavailable(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	for _, tc := range []struct{ name, cipher, state string }{
		{"historical", "", "not_saved"},
		{"corrupted", "v1:invalid-ciphertext", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := call(a, "GET", "/hello?token=unrecoverable-token", "", nil)
			id := w.Header().Get("X-Request-ID")
			if _, err := a.DB.Exec(context.Background(), "UPDATE request_logs SET query_token_cipher=$2 WHERE id=$1", id, tc.cipher); err != nil {
				t.Fatal(err)
			}
			w = call(a, "GET", "/api/v1/request-logs/"+id, "", admin)
			var detail struct {
				Query map[string]any `json:"query"`
				State string         `json:"query_token_state"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil || w.Code != 200 || detail.State != tc.state || detail.Query["token"] != "[REDACTED]" {
				t.Fatal("unavailable token should keep masked detail", err, w.Code)
			}
		})
	}
}

// 明文返回必须有审计记录；审计写入失败时不能静默放行。
func TestRequestLogQueryTokenRequiresAudit(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	w := call(a, "GET", "/hello?token=audit-protected-token", "", nil)
	id := w.Header().Get("X-Request-ID")
	if _, err := a.DB.Exec(context.Background(), "ALTER TABLE audit_logs RENAME TO unavailable_audit_logs"); err != nil {
		t.Fatal(err)
	}
	w = call(a, "GET", "/api/v1/request-logs/"+id, "", admin)
	if w.Code != 500 || strings.Contains(w.Body.String(), "audit-protected-token") {
		t.Fatal("token disclosed without audit")
	}
}

func TestRequestLogsOnlyVisitor(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	for _, path := range []string{"/healthz", "/assets/missing.js", "/", "/api/v1/settings", "/api/v1/request-logs"} {
		call(a, "GET", path, "", admin)
	}
	var count int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM request_logs").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("non-visitor requests logged: %d", count)
	}
	call(a, "GET", "/hello", "", nil)
	call(a, "POST", "/hello/contact", "", nil)
	call(a, "POST", "/hello/view", "", nil)
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM request_logs").Scan(&count); err != nil || count != 3 {
		t.Fatal("visitor logs missing", err, count)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events").Scan(&count); err != nil || count != 1 {
		t.Fatal("business tracking changed", err, count)
	}
	w := call(a, "GET", "/api/v1/request-logs", "", admin)
	var result struct{ Total int }
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || result.Total != 3 {
		t.Fatal("visitor list mismatch", err, w.Body.String())
	}
}
