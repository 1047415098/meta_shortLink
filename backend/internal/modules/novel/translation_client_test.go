package novel

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

type recordingTranslator struct {
	configured bool
	result     string
	locales    []string
}

func (translator *recordingTranslator) Configured() bool { return translator.configured }
func (translator *recordingTranslator) Translate(_ context.Context, locale, _ string) (string, error) {
	translator.locales = append(translator.locales, locale)
	return translator.result, nil
}

func TestRoutedTranslatorUsesDeepLOnlyForVietnamese(t *testing.T) {
	apihz := &recordingTranslator{configured: true, result: "apihz"}
	deepl := &recordingTranslator{configured: true, result: "deepl"}
	translator := RoutedTranslator{APIHZ: apihz, DeepL: deepl}

	got, err := translator.Translate(context.Background(), "vi", "Hello")
	if err != nil || got != "deepl" || len(deepl.locales) != 1 || len(apihz.locales) != 0 {
		t.Fatalf("Vietnamese route = %q, api=%v, deepl=%v, err=%v", got, apihz.locales, deepl.locales, err)
	}
	got, err = translator.Translate(context.Background(), "ja", "Hello")
	if err != nil || got != "apihz" || len(apihz.locales) != 1 {
		t.Fatalf("Japanese route = %q, api=%v, deepl=%v, err=%v", got, apihz.locales, deepl.locales, err)
	}
}

func TestDeepLTranslationClientPostsVietnameseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "DeepL-Auth-Key server-secret" {
			t.Fatalf("unexpected DeepL request: method=%s authorization=%q", r.Method, r.Header.Get("Authorization"))
		}
		var payload struct {
			Text       []string `json:"text"`
			SourceLang string   `json:"source_lang"`
			TargetLang string   `json:"target_lang"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Text) != 1 || payload.Text[0] != "Hello world" || payload.SourceLang != "EN" || payload.TargetLang != "VI" {
			t.Fatalf("unexpected DeepL payload: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translations":[{"detected_source_language":"EN","text":"Xin chào thế giới"}]}`))
	}))
	defer server.Close()

	client := DeepLTranslationClient{Endpoint: server.URL, AuthKey: "server-secret", HTTPClient: server.Client()}
	got, err := client.Translate(context.Background(), "vi", "Hello world")
	if err != nil || got != "Xin chào thế giới" {
		t.Fatalf("DeepL translation = %q, %v", got, err)
	}
	if _, err = client.Translate(context.Background(), "km", "Hello"); err == nil {
		t.Fatal("DeepL accepted an unsupported project locale")
	}
}

func TestDeepLTranslationClientRetriesRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts == 1 {
			http.Error(w, `{"message":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"translations":[{"text":"Xin chào"}]}`))
	}))
	defer server.Close()

	// Removing transient retry handling would leave a whole-book job failed after one provider throttle.
	client := DeepLTranslationClient{Endpoint: server.URL, AuthKey: "server-secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond}
	got, err := client.Translate(context.Background(), "vi", "Hello")
	if err != nil || got != "Xin chào" || attempts != 2 {
		t.Fatalf("DeepL retry = %q, attempts=%d, err=%v", got, attempts, err)
	}
}

func TestDeepLTranslationClientRetriesSustainedRateLimit(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 4 {
			http.Error(w, `{"message":"too many requests"}`, http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"translations":[{"text":"Xin chào"}]}`))
	}))
	defer server.Close()

	// DeepL can throttle for several consecutive attempts; stopping after three loses the whole novel job.
	client := DeepLTranslationClient{Endpoint: server.URL, AuthKey: "server-secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond}
	got, err := client.Translate(context.Background(), "vi", "Hello")
	if err != nil || got != "Xin chào" || attempts != 4 {
		t.Fatalf("DeepL sustained retry = %q, attempts=%d, err=%v", got, attempts, err)
	}
}

func TestDeepLTranslationClientKeepsSafeProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(456)
		_, _ = w.Write([]byte(`{"message":"quota exceeded for server-secret"}`))
	}))
	defer server.Close()

	// Provider detail is useful to operations, but the server-only key must never reach an API error response.
	client := DeepLTranslationClient{Endpoint: server.URL, AuthKey: "server-secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond}
	_, err := client.Translate(context.Background(), "vi", "Hello")
	if err == nil || !strings.Contains(err.Error(), "quota exceeded") || strings.Contains(err.Error(), "server-secret") {
		t.Fatalf("unsafe or unhelpful DeepL error: %v", err)
	}
}

func TestDeepLTranslationClientLimitsConcurrentRequestStartRate(t *testing.T) {
	var mu sync.Mutex
	starts := []time.Time{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		_, _ = w.Write([]byte(`{"translations":[{"text":"Xin chào"}]}`))
	}))
	defer server.Close()

	// All novel workers share this client, so request starts must be serialized before DeepL sees a burst.
	client := &DeepLTranslationClient{Endpoint: server.URL, AuthKey: "server-secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond, MinRequestInterval: 20 * time.Millisecond}
	var workers sync.WaitGroup
	for index := 0; index < 3; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := client.Translate(context.Background(), "vi", "Hello"); err != nil {
				t.Errorf("translate: %v", err)
			}
		}()
	}
	workers.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 3 {
		t.Fatalf("request starts = %d, want 3", len(starts))
	}
	for index := 1; index < len(starts); index++ {
		if gap := starts[index].Sub(starts[index-1]); gap < 15*time.Millisecond {
			t.Fatalf("request gap = %v, want at least 15ms", gap)
		}
	}
}

func TestTranslationClientLocaleMappings(t *testing.T) {
	want := map[string]int{"id": 24, "ja": 27, "ko": 28, "ms": 31, "pt": 35, "fil": 43, "th": 44, "vi": 48}
	if len(TargetLocales) != len(want) {
		t.Fatalf("target locales = %d, want %d", len(TargetLocales), len(want))
	}
	for code, apiType := range want {
		got, ok := TranslationType(code)
		if !ok || got != apiType {
			t.Fatalf("locale %s = %d, %v; want %d", code, got, ok, apiType)
		}
	}
	if _, ok := TranslationType("en"); ok {
		t.Fatal("English must not be submitted as a target translation")
	}
}

func TestTranslationChunksPreserveUTF8AndByteLimit(t *testing.T) {
	input := strings.Repeat("日本語 paragraph with words. ", 30) + "\n\n" + strings.Repeat("tail ", 40)
	chunks := SplitTranslationText(input, 120)
	if strings.Join(chunks, "") != input {
		t.Fatal("chunks did not preserve the source text")
	}
	for _, chunk := range chunks {
		if !utf8.ValidString(chunk) || len([]byte(chunk)) > 120 {
			t.Fatalf("invalid chunk: bytes=%d valid=%v", len([]byte(chunk)), utf8.ValidString(chunk))
		}
	}
}

func TestTranslationClientPostsAPIHZFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"id": "developer", "key": "secret", "words": "Hello world", "ytype": "1", "etype": "27", "htype": "1"}
		for key, value := range want {
			if r.PostForm.Get(key) != value {
				t.Fatalf("%s = %q, want %q", key, r.PostForm.Get(key), value)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"words":"こんにちは世界","msg":"ok"}`))
	}))
	defer server.Close()

	client := TranslationClient{Endpoint: server.URL, DeveloperID: "developer", DeveloperKey: "secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond}
	got, err := client.Translate(context.Background(), "ja", "Hello world")
	if err != nil || got != "こんにちは世界" {
		t.Fatalf("translate = %q, %v", got, err)
	}
}

func TestTranslationClientRetriesAndRejectsBadResponses(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"words":"translated","msg":"ok"}`))
	}))
	defer server.Close()
	client := TranslationClient{Endpoint: server.URL, DeveloperID: "developer", DeveloperKey: "secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond}
	if got, err := client.Translate(context.Background(), "th", "source"); err != nil || got != "translated" || attempts != 3 {
		t.Fatalf("retry result = %q, attempts=%d, err=%v", got, attempts, err)
	}

	for _, body := range []string{`{"code":400,"msg":"bad key"}`, `{"code":200,"words":"","msg":"ok"}`, `{broken`} {
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
		client.Endpoint, client.HTTPClient = bad.URL, bad.Client()
		if _, err := client.Translate(context.Background(), "vi", "source"); err == nil || strings.Contains(err.Error(), "secret") {
			bad.Close()
			t.Fatalf("bad response %q returned unsafe error %v", body, err)
		}
		bad.Close()
	}
	if _, err := client.Translate(context.Background(), "xx", "source"); err == nil {
		t.Fatal("unsupported locale was accepted")
	}
	if _, err := client.Translate(context.Background(), "ja", strings.Repeat("x", 5001)); err == nil {
		t.Fatal("oversized source was accepted")
	}
}

func TestTranslationClientRetriesTemporaryProviderErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 7 {
			_, _ = w.Write([]byte(`{"code":400,"msg":"失败，请重试！"}`))
			return
		}
		_, _ = w.Write([]byte(`{"code":200,"words":"translated","msg":"ok"}`))
	}))
	defer server.Close()

	// 上游短暂返回业务错误时，应覆盖可持续数十秒的抖动窗口。
	client := TranslationClient{Endpoint: server.URL, DeveloperID: "developer", DeveloperKey: "secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond}
	got, err := client.Translate(context.Background(), "id", "source")
	if err != nil || got != "translated" || attempts != 7 {
		t.Fatalf("temporary provider error result = %q, attempts=%d, err=%v", got, attempts, err)
	}
}

func TestTranslationClientStopsRetryingUnsupportedProviderLocale(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		_, _ = w.Write([]byte(`{"code":400,"msg":"错误：vi is not supported"}`))
	}))
	defer server.Close()

	client := TranslationClient{Endpoint: server.URL, DeveloperID: "developer", DeveloperKey: "secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond}
	_, err := client.Translate(context.Background(), "vi", "source")
	if err == nil || !strings.Contains(err.Error(), "vi is not supported") || attempts != 1 {
		t.Fatalf("unsupported provider locale attempts=%d, err=%v", attempts, err)
	}
}

func TestTranslationClientLimitsConcurrentRequestStartRate(t *testing.T) {
	var mu sync.Mutex
	starts := []time.Time{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		starts = append(starts, time.Now())
		mu.Unlock()
		_, _ = w.Write([]byte(`{"code":200,"words":"translated","msg":"ok"}`))
	}))
	defer server.Close()

	// 多个翻译 worker 共用同一客户端时，请求起点仍必须保持全局间隔。
	client := &TranslationClient{Endpoint: server.URL, DeveloperID: "developer", DeveloperKey: "secret", HTTPClient: server.Client(), RetryDelay: time.Millisecond, MinRequestInterval: 20 * time.Millisecond}
	var workers sync.WaitGroup
	for index := 0; index < 3; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if _, err := client.Translate(context.Background(), "ja", "source"); err != nil {
				t.Errorf("translate: %v", err)
			}
		}()
	}
	workers.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(starts) != 3 {
		t.Fatalf("request starts = %d, want 3", len(starts))
	}
	for index := 1; index < len(starts); index++ {
		if gap := starts[index].Sub(starts[index-1]); gap < 15*time.Millisecond {
			t.Fatalf("request gap = %v, want at least 15ms", gap)
		}
	}
}
