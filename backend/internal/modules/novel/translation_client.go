package novel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type LocaleOption struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	APIType int    `json:"-"`
}

var TargetLocales = []LocaleOption{
	{Code: "id", Name: "印度尼西亚语", APIType: 24},
	{Code: "ja", Name: "日语", APIType: 27},
	{Code: "ko", Name: "韩语", APIType: 28},
	{Code: "ms", Name: "马来语", APIType: 31},
	{Code: "pt", Name: "葡萄牙语", APIType: 35},
	{Code: "fil", Name: "菲律宾语", APIType: 43},
	{Code: "th", Name: "泰语", APIType: 44},
	{Code: "vi", Name: "越南语", APIType: 48},
}

func TranslationType(locale string) (int, bool) {
	for _, option := range TargetLocales {
		if option.Code == locale {
			return option.APIType, true
		}
	}
	return 0, false
}

// SplitTranslationText preserves the exact source while preferring paragraph boundaries.
func SplitTranslationText(text string, maxBytes int) []string {
	if text == "" {
		return nil
	}
	if maxBytes < 1 {
		return []string{text}
	}
	chunks := []string{}
	for len([]byte(text)) > maxBytes {
		cut := maxBytes
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut--
		}
		if cut == 0 {
			cut = len(text)
		}
		window := text[:cut]
		minimum := cut / 3
		for _, separator := range []string{"\n\n", "\n", " "} {
			if index := strings.LastIndex(window, separator); index >= minimum {
				cut = index + len(separator)
				break
			}
		}
		chunks = append(chunks, text[:cut])
		text = text[cut:]
	}
	if text != "" {
		chunks = append(chunks, text)
	}
	return chunks
}

type TranslationClient struct {
	Endpoint     string
	DeveloperID  string
	DeveloperKey string
	HTTPClient   *http.Client
	RetryDelay   time.Duration
	// MinRequestInterval applies one shared request-start limit across all translation workers.
	MinRequestInterval time.Duration
	requestMu          sync.Mutex
	lastRequest        time.Time
}

type translationProviderError struct {
	code    int
	message string
}

func (err translationProviderError) Error() string {
	if err.message == "" {
		return fmt.Sprintf("翻译服务返回错误 code=%d", err.code)
	}
	return fmt.Sprintf("翻译服务返回错误 code=%d: %s", err.code, err.message)
}

func (err translationProviderError) permanent() bool {
	message := strings.ToLower(err.message)
	// 服务商明确拒绝的语言不会随重试恢复，立即失败可避免无效消耗调用次数。
	return strings.Contains(message, "not supported") || strings.Contains(message, "不支持")
}

func (err translationProviderError) splittable() bool {
	// APIHZ 对特定长句只返回此通用错误，缩小文本后可正常翻译。
	return strings.Contains(err.message, "失败，请重试")
}

func (client *TranslationClient) Configured() bool {
	return client.Endpoint != "" && client.DeveloperID != "" && client.DeveloperKey != ""
}

func (client *TranslationClient) Translate(ctx context.Context, locale, text string) (string, error) {
	apiType, ok := TranslationType(locale)
	if !ok {
		return "", errors.New("不支持的目标语言")
	}
	if !client.Configured() {
		return "", errors.New("翻译服务尚未配置")
	}
	if len([]byte(text)) > 5000 {
		return "", errors.New("单次翻译文本不能超过 5000 字节")
	}
	if strings.TrimSpace(text) == "" {
		return text, nil
	}
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	delay := client.RetryDelay
	if delay <= 0 {
		// 服务商偶发的业务错误可能持续数秒，扩大退避窗口后再判定任务失败。
		delay = 2 * time.Second
	}
	var lastErr error
	const maxAttempts = 8
	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := client.translateOnce(ctx, httpClient, apiType, text)
		if err == nil {
			return result, nil
		}
		lastErr = err
		var providerErr translationProviderError
		if errors.As(err, &providerErr) && providerErr.permanent() {
			return "", err
		}
		if attempt < maxAttempts-1 {
			// 生产环境通用业务错误偶尔持续超过 20 秒，线性退避覆盖约 1 分钟后再失败。
			timer := time.NewTimer(delay * time.Duration(attempt+1))
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
	}
	return "", lastErr
}

func (client *TranslationClient) translateOnce(ctx context.Context, httpClient *http.Client, apiType int, text string) (string, error) {
	if err := client.waitForRequestSlot(ctx); err != nil {
		return "", err
	}
	form := url.Values{
		"id":    {client.DeveloperID},
		"key":   {client.DeveloperKey},
		"words": {text},
		"ytype": {"1"},
		"etype": {strconv.Itoa(apiType)},
		"htype": {"1"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.Endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", errors.New("翻译请求创建失败")
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=utf-8")
	request.Header.Set("Accept", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("翻译服务请求失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("翻译服务 HTTP 状态 %d", response.StatusCode)
	}
	var payload struct {
		Code  int    `json:"code"`
		Words string `json:"words"`
		Msg   string `json:"msg"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1024*1024))
	if err = decoder.Decode(&payload); err != nil {
		return "", errors.New("翻译服务返回格式无效")
	}
	if payload.Code != 200 {
		message := strings.TrimSpace(payload.Msg)
		// 上游信息用于区分临时失败和永久不支持，同时隐藏可能意外回显的凭证。
		message = strings.ReplaceAll(message, client.DeveloperKey, "[redacted]")
		if runes := []rune(message); len(runes) > 160 {
			message = string(runes[:160])
		}
		return "", translationProviderError{code: payload.Code, message: message}
	}
	if strings.TrimSpace(payload.Words) == "" {
		return "", errors.New("翻译服务返回空译文")
	}
	return payload.Words, nil
}

func (client *TranslationClient) waitForRequestSlot(ctx context.Context) error {
	interval := client.MinRequestInterval
	if interval <= 0 {
		// 当前账号限制为 116 次/分钟，750ms 间隔将稳定上限控制在约 80 次/分钟。
		interval = 750 * time.Millisecond
	}
	client.requestMu.Lock()
	defer client.requestMu.Unlock()
	if wait := interval - time.Since(client.lastRequest); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	client.lastRequest = time.Now()
	return nil
}
