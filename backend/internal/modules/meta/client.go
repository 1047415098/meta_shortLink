package meta

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"whatsapp-analytics/internal/platform/runtime"
)

// BaseURL is a dependency injection boundary. Production always uses graph.facebook.com.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func NewClient() *Client {
	return &Client{BaseURL: "https://graph.facebook.com", HTTP: &http.Client{Timeout: 25 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

type GraphError struct {
	Status, Code, Subcode int
	Message, Trace        string
	Temporary             bool
	RetryAfter            time.Duration
}

func (e *GraphError) Error() string {
	return fmt.Sprintf("Meta 请求失败（HTTP %d，code %d）：%s", e.Status, e.Code, e.Message)
}
func retryable(err error) bool { var e *GraphError; return errors.As(err, &e) && e.Temporary }
func cleanMessage(message string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			message = strings.ReplaceAll(message, secret, "[REDACTED]")
			message = strings.ReplaceAll(message, url.QueryEscape(secret), "[REDACTED]")
		}
	}
	message = strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		return r
	}, message)
	return runtime.Bounded(message, 1200)
}
func (c *Client) Post(ctx context.Context, version, token, path string, body any, out any) error {
	// CAPI uses one fixed-host POST transport; no Meta reporting reads are exposed.
	return c.call(ctx, version, token, path, body, out)
}
func (c *Client) call(ctx context.Context, version, token, path string, body any, out any) error {
	if !versionPattern.MatchString(version) || strings.ContainsAny(path, "?\\#") || strings.Contains(path, "..") {
		return errors.New("Meta 请求路径无效")
	}
	if token == "" {
		return errors.New("尚未配置 Meta 凭证")
	}
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/" + version + "/" + path
	var b []byte
	var err error
	if body != nil {
		b, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(b))
	if err != nil {
		return errors.New("Meta 请求创建失败")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := c.HTTP.Do(req)
	if err != nil {
		return &GraphError{Message: "网络连接失败或请求超时", Temporary: true}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
	if err != nil {
		return &GraphError{Status: response.StatusCode, Message: "读取响应失败", Temporary: true}
	}
	if len(raw) > 8<<20 {
		return &GraphError{Status: response.StatusCode, Message: "响应超过大小限制"}
	}
	var envelope struct {
		Error *struct {
			Message   string `json:"message"`
			Code      int    `json:"code"`
			Subcode   int    `json:"error_subcode"`
			Trace     string `json:"fbtrace_id"`
			Transient bool   `json:"is_transient"`
		} `json:"error"`
	}
	parseErr := json.Unmarshal(raw, &envelope)
	if envelope.Error != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		e := &GraphError{Status: response.StatusCode, Message: "接口返回非成功响应", Temporary: response.StatusCode == 429 || response.StatusCode >= 500}
		if envelope.Error != nil {
			x := envelope.Error
			e.Code = x.Code
			e.Subcode = x.Subcode
			e.Trace = x.Trace
			e.Message = cleanMessage(x.Message, token)
			e.Temporary = e.Temporary || x.Transient || x.Code == 1 || x.Code == 2 || x.Code == 4 || x.Code == 17 || x.Code == 32 || x.Code == 613
		}
		if e.Code == 190 {
			e.Temporary = false
		}
		if seconds, e2 := time.ParseDuration(response.Header.Get("Retry-After") + "s"); e2 == nil && seconds > 0 {
			e.RetryAfter = seconds
		}
		return e
	}
	if parseErr != nil {
		return &GraphError{Status: response.StatusCode, Message: "接口未返回有效 JSON", Temporary: true}
	}
	if out != nil {
		if err = json.Unmarshal(raw, out); err != nil {
			return &GraphError{Status: response.StatusCode, Message: "接口数据格式不符合预期"}
		}
	}
	return nil
}
