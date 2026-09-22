package requestlogs

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/platform/runtime"
)

const logBodyLimit = 32 * 1024

type responseCapture struct {
	gin.ResponseWriter
	data  []byte
	total int64
}

func (w *responseCapture) capture(b []byte) {
	w.total += int64(len(b))
	if n := logBodyLimit - len(w.data); n > 0 {
		if len(b) > n {
			b = b[:n]
		}
		w.data = append(w.data, b...)
	}
}

func (w *responseCapture) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *responseCapture) WriteString(s string) (int, error) { return w.Write([]byte(s)) }

func secretKey(k string) bool {
	k = strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(k))
	for _, v := range []string{"password", "passwd", "pwd", "secret", "token", "authorization", "cookie", "ticket", "apikey", "session"} {
		if strings.Contains(k, v) {
			return true
		}
	}
	return false
}

func redact(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, item := range x {
			if secretKey(k) {
				x[k] = "[REDACTED]"
			} else {
				x[k] = redact(item)
			}
		}
	case []any:
		for i, item := range x {
			x[i] = redact(item)
		}
	}
	return v
}

func safeValues(values map[string][]string) any {
	out := map[string]any{}
	for k, v := range values {
		if secretKey(k) {
			out[k] = "[REDACTED]"
		} else if strings.HasPrefix(strings.ToLower(k), "x-meta-") {
			// Landing headers encode Unicode as UTF-8 percent sequences; decode bounded values for readable logs.
			decoded := make([]string, 0, len(v))
			for _, raw := range v {
				value, err := url.PathUnescape(raw)
				if err != nil {
					value = raw
				}
				decoded = append(decoded, runtime.Bounded(value, 2048))
			}
			out[k] = decoded
		} else {
			out[k] = v
		}
	}
	// URLs in Referer or Location may also carry credentials in their query.
	for k, v := range values {
		if strings.EqualFold(k, "Referer") || strings.EqualFold(k, "Location") {
			cleaned := []string{}
			for _, s := range v {
				u, e := url.Parse(s)
				if e != nil {
					cleaned = append(cleaned, "[invalid URL]")
					continue
				}
				q := u.Query()
				for key := range q {
					if secretKey(key) {
						q.Set(key, "[REDACTED]")
					}
				}
				u.RawQuery = q.Encode()
				u.User = nil
				cleaned = append(cleaned, u.String())
			}
			out[k] = cleaned
		}
	}
	return out
}

func bodyLog(contentType string, data []byte, total int64, private bool) any {
	if private {
		return map[string]any{"note": "敏感或日志接口正文不保存"}
	}
	if total == 0 {
		return nil
	}
	if total > logBodyLimit {
		return map[string]any{"note": "正文超过 32 KB，未保存", "bytes": total}
	}
	if strings.Contains(contentType, "json") {
		var v any
		if json.Unmarshal(data, &v) == nil {
			return redact(v)
		}
	}
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		if values, e := url.ParseQuery(string(data)); e == nil {
			return safeValues(values)
		}
	}
	return map[string]any{"note": "非 JSON / 表单正文，仅记录类型与大小", "content_type": contentType, "bytes": total}
}

func logJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func (a *Handler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		visitorRoute := c.FullPath() == "/:code" || c.FullPath() == "/:code/contact" || c.FullPath() == "/:code/view" || c.FullPath() == "/:code/time-spent" ||
			c.FullPath() == "/audio-novel/:code" || c.FullPath() == "/audio-novel/:code/stories" || c.FullPath() == "/audio-novel/:code/stories/:slug" ||
			c.FullPath() == "/audio-novel/:code/contact" || c.FullPath() == "/audio-novel/:code/view" || c.FullPath() == "/audio-novel/:code/time-spent" ||
			c.FullPath() == "/novel/:code" || c.FullPath() == "/novel/:code/search" || c.FullPath() == "/novel/:code/stories" || c.FullPath() == "/novel/:code/stories/:slug"
		if !visitorRoute {
			c.Next()
			return
		}
		start := time.Now()
		id := runtime.Token()
		c.Header("X-Request-ID", id)
		path := runtime.Bounded(c.Request.URL.Path, 2048)
		private := strings.HasPrefix(path, "/api/v1/auth/") || strings.HasPrefix(path, "/api/v1/request-logs")
		var body []byte
		requestBytes := c.Request.ContentLength
		ct := c.GetHeader("Content-Type")
		if c.Request.Body != nil && !private && (strings.Contains(ct, "json") || strings.Contains(ct, "application/x-www-form-urlencoded")) {
			body, _ = io.ReadAll(io.LimitReader(c.Request.Body, logBodyLimit+1))
			c.Request.Body = struct {
				io.Reader
				io.Closer
			}{io.MultiReader(bytes.NewReader(body), c.Request.Body), c.Request.Body}
			if requestBytes < 0 {
				requestBytes = int64(len(body))
			}
		}
		requestHeaders := c.Request.Header.Clone()
		requestHeaders.Set("Host", c.Request.Host)
		headers := logJSON(safeValues(requestHeaders))
		query := logJSON(safeValues(c.Request.URL.Query()))
		// 脱敏日志照常保存，URL token 的原值单独加密，供管理员详情查看。
		queryTokenCipher, tokenErr := a.sealQueryToken(id, c.Request.URL.Query())
		if tokenErr != nil {
			a.WriteFailures.Add(1)
			slog.Error("REQUEST_LOG_TOKEN_ENCRYPT_FAILED", "request_id", id)
		}
		writer := &responseCapture{ResponseWriter: c.Writer}
		c.Writer = writer
		c.Next()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := a.DB.Exec(ctx, `INSERT INTO request_logs(id,occurred_at,method,path,client_ip,status,duration_ms,request_headers,query,request_body,response_headers,response_body,request_bytes,response_bytes,is_visitor,query_token_cipher) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,true,$15)`, id, start, c.Request.Method, path, c.ClientIP(), writer.Status(), float64(time.Since(start).Microseconds())/1000, headers, query, logJSON(bodyLog(ct, body, requestBytes, private)), logJSON(safeValues(writer.Header())), logJSON(bodyLog(writer.Header().Get("Content-Type"), writer.data, writer.total, private)), requestBytes, writer.total, queryTokenCipher)
		if err != nil {
			a.WriteFailures.Add(1)
			slog.Error("REQUEST_LOG_WRITE_FAILED", "request_id", id, "error", err)
		}
	}
}
