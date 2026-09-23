package novel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// RoutedTranslator keeps the existing APIHZ languages unchanged and sends Vietnamese to DeepL.
type RoutedTranslator struct {
	APIHZ TextTranslator
	DeepL TextTranslator
}

func (translator RoutedTranslator) Configured() bool {
	return translator.APIHZ != nil && translator.APIHZ.Configured() || translator.DeepL != nil && translator.DeepL.Configured()
}

func (translator RoutedTranslator) Translate(ctx context.Context, locale, text string) (string, error) {
	provider := translator.APIHZ
	if locale == "vi" {
		provider = translator.DeepL
	}
	if provider == nil || !provider.Configured() {
		return "", ErrTranslationNotConfigured
	}
	return provider.Translate(ctx, locale, text)
}

type DeepLTranslationClient struct {
	Endpoint   string
	AuthKey    string
	HTTPClient *http.Client
	RetryDelay time.Duration
	// MinRequestInterval limits request bursts shared by all novel translation workers.
	MinRequestInterval time.Duration
	requestMu          sync.Mutex
	lastRequest        time.Time
}

func (client *DeepLTranslationClient) Configured() bool {
	return client.Endpoint != "" && client.AuthKey != ""
}

func (client *DeepLTranslationClient) Translate(ctx context.Context, locale, text string) (string, error) {
	// DeepL supports Vietnamese here; project locales unsupported by this provider must not be sent accidentally.
	if locale != "vi" {
		return "", errors.New("DeepL 不支持该项目目标语言")
	}
	if !client.Configured() {
		return "", ErrTranslationNotConfigured
	}
	if strings.TrimSpace(text) == "" {
		return text, nil
	}
	httpClient := client.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	delay := client.RetryDelay
	if delay <= 0 {
		delay = time.Second
	}
	var lastErr error
	const maxAttempts = 8
	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, retry, err := client.translateOnce(ctx, httpClient, text)
		if err == nil {
			return result, nil
		}
		lastErr = err
		if !retry || attempt == maxAttempts-1 {
			return "", err
		}
		// DeepL requires exponential backoff for adaptive 429/5xx throttling.
		timer := time.NewTimer(delay * time.Duration(1<<attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}
	return "", lastErr
}

func (client *DeepLTranslationClient) translateOnce(ctx context.Context, httpClient *http.Client, text string) (string, bool, error) {
	if err := client.waitForRequestSlot(ctx); err != nil {
		return "", false, err
	}
	payload, err := json.Marshal(struct {
		Text       []string `json:"text"`
		SourceLang string   `json:"source_lang"`
		TargetLang string   `json:"target_lang"`
	}{Text: []string{text}, SourceLang: "EN", TargetLang: "VI"})
	if err != nil {
		return "", false, errors.New("DeepL 翻译请求编码失败")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", false, errors.New("DeepL 翻译请求创建失败")
	}
	request.Header.Set("Authorization", "DeepL-Auth-Key "+client.AuthKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return "", true, fmt.Errorf("DeepL 翻译服务请求失败: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil {
		return "", true, errors.New("DeepL 翻译响应读取失败")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		var providerError struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(body, &providerError)
		message := strings.TrimSpace(providerError.Message)
		if message == "" {
			message = strings.TrimSpace(string(body))
		}
		// Upstream detail helps operations diagnose quota and parameter errors, while any echoed secret is removed.
		message = strings.ReplaceAll(message, client.AuthKey, "[redacted]")
		if runes := []rune(message); len(runes) > 160 {
			message = string(runes[:160])
		}
		return "", response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500, fmt.Errorf("DeepL 翻译服务 HTTP 状态 %d: %s", response.StatusCode, message)
	}
	var result struct {
		Translations []struct {
			Text string `json:"text"`
		} `json:"translations"`
	}
	if err = json.Unmarshal(body, &result); err != nil || len(result.Translations) == 0 {
		return "", false, errors.New("DeepL 翻译服务返回格式无效")
	}
	translated := result.Translations[0].Text
	if strings.TrimSpace(translated) == "" {
		return "", false, errors.New("DeepL 翻译服务返回空译文")
	}
	return translated, false, nil
}

func (client *DeepLTranslationClient) waitForRequestSlot(ctx context.Context) error {
	interval := client.MinRequestInterval
	if interval <= 0 {
		// DeepL dynamically adjusts limits; a conservative shared interval prevents eight novel workers from bursting.
		interval = 250 * time.Millisecond
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
