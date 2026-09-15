package runtime

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oschwald/geoip2-golang"

	"whatsapp-analytics/internal/config"
)

type bucket struct {
	Count int
	Until time.Time
}
type Core struct {
	Config        config.Config
	DB            *pgxpool.Pool
	Geo           *geoip2.Reader
	WriteFailures atomic.Uint64
	mu            sync.Mutex
	rates         map[string]bucket
}

func Token() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func Digest(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func (a *Core) Sign(s string) string {
	h := hmac.New(sha256.New, []byte(a.Config.Secret))
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}
func (a *Core) Exceed(key string, limit int, d time.Duration) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	if len(a.rates) > 10000 {
		for k, v := range a.rates {
			if now.After(v.Until) {
				delete(a.rates, k)
			}
		}
		if len(a.rates) > 20000 {
			return true
		}
	}
	v := a.rates[key]
	if now.After(v.Until) {
		v = bucket{Until: now.Add(d)}
	}
	v.Count++
	a.rates[key] = v
	return v.Count > limit
}
func (a *Core) SessionName() string {
	if a.Config.SecureCookies {
		return "__Host-admin"
	}
	return "wa_admin_dev"
}
func (a *Core) SetCookie(c *gin.Context, name, value string, age int) {
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: age, HttpOnly: true, Secure: a.Config.SecureCookies, SameSite: http.SameSiteLaxMode})
}
func Bad(c *gin.Context, s string) { c.JSON(400, gin.H{"error": s}) }
func ServerError(c *gin.Context, e error) {
	slog.Error("api failure", "error", e)
	c.JSON(500, gin.H{"error": "服务暂时不可用，请重试"})
}
func (a *Core) Audit(ctx context.Context, action string, detail any) error {
	b, e := json.Marshal(detail)
	if e != nil {
		return e
	}
	_, e = a.DB.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", a.Config.AdminUser, action, b)
	return e
}
func Bounded(s string, n int) string {
	s = strings.ToValidUTF8(s, "")
	if len(s) > n {
		s = s[:n]
		s = strings.ToValidUTF8(s, "")
	}
	return s
}

var VisitorPattern = regexp.MustCompile(`^[a-f0-9]{48}$`)

func New(c config.Config, db *pgxpool.Pool) *Core {
	return &Core{Config: c, DB: db, rates: map[string]bucket{}}
}
