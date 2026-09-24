package tiktok

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"whatsapp-analytics/internal/platform/runtime"
)

type Client struct {
	Endpoint string
	HTTP     *http.Client
}

type APIError struct {
	HTTPStatus   int
	BusinessCode int64
	Message      string
	RequestID    string
	Temporary    bool
	RetryAfter   time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("TikTok 请求失败（HTTP %d，code %d）：%s", e.HTTPStatus, e.BusinessCode, e.Message)
}

func NewClient(endpoint string) *Client {
	return &Client{
		Endpoint: endpoint,
		HTTP: &http.Client{
			Timeout:       10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *Client) Post(ctx context.Context, token string, payload EventRequest) (DeliveryResponse, error) {
	if strings.TrimSpace(token) == "" {
		return DeliveryResponse{}, errors.New("尚未配置 TikTok Access Token")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return DeliveryResponse{}, errors.New("TikTok 事件数据无法编码")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return DeliveryResponse{}, errors.New("TikTok 请求创建失败")
	}
	request.Header.Set("Access-Token", token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	response, err := c.HTTP.Do(request)
	if err != nil {
		return DeliveryResponse{}, &APIError{Message: "网络连接失败或请求超时", Temporary: true}
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	if err != nil {
		return DeliveryResponse{}, &APIError{HTTPStatus: response.StatusCode, Message: "读取响应失败", Temporary: true}
	}
	if len(raw) > 1<<20 {
		return DeliveryResponse{}, &APIError{HTTPStatus: response.StatusCode, Message: "响应超过大小限制"}
	}
	var result DeliveryResponse
	if err = json.Unmarshal(raw, &result); err != nil {
		return DeliveryResponse{}, &APIError{HTTPStatus: response.StatusCode, Message: "接口未返回有效 JSON", Temporary: response.StatusCode >= 500 || response.StatusCode == http.StatusTooManyRequests || response.StatusCode < 300}
	}
	if response.StatusCode >= 200 && response.StatusCode < 300 && result.Code == 0 {
		return result, nil
	}
	secrets := []string{token}
	// TikTok may echo rejected values. Diagnostics must never expose private
	// matching context or identifiers tied to one anonymous reading session.
	for _, event := range payload.Data {
		secrets = append(secrets, event.EventID, event.User.TTCLID, event.User.TTP,
			event.User.IP, event.User.UserAgent, event.Page.URL, event.Page.Referrer)
	}
	apiError := &APIError{
		HTTPStatus:   response.StatusCode,
		BusinessCode: result.Code,
		Message:      cleanMessage(result.Message, secrets...),
		RequestID:    runtime.Bounded(result.RequestID, 255),
		Temporary:    response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500,
		RetryAfter:   retryAfter(response.Header.Get("Retry-After"), time.Now()),
	}
	if apiError.Message == "" {
		apiError.Message = "接口返回非成功响应"
	}
	return DeliveryResponse{}, apiError
}

func retryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}

func cleanMessage(message string, secrets ...string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		message = strings.ReplaceAll(message, secret, "[REDACTED]")
		message = strings.ReplaceAll(message, url.QueryEscape(secret), "[REDACTED]")
	}
	message = strings.Map(func(r rune) rune {
		if r < 32 {
			return ' '
		}
		return r
	}, message)
	return runtime.Bounded(message, 1200)
}
