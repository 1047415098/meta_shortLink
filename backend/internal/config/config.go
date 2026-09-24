package config

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MetaEncryptionKeys                                                                                                                                                                                       map[string]string
	MetaEncryptionKeyID                                                                                                                                                                                      string
	DatabaseURL, PublicURL, AdminUser, AdminPassword, Secret, CookieMode, Timezone, GeoDB, Listen, FrontendDir, LandingDir, AudioNovelDir, AudioNovelUploadDir, AudioNovelAudioDir, NovelDir, NovelUploadDir string
	SecureCookies                                                                                                                                                                                            bool
	RetentionDays                                                                                                                                                                                            int
	TrustedProxies                                                                                                                                                                                           []string
	APIHZTranslationID, APIHZTranslationKey, APIHZTranslationURL                                                                                                                                             string
	DeepLAuthKey, DeepLTranslationURL                                                                                                                                                                        string
	TikTokEnabled                                                                                                                                                                                            bool
	TikTokEventsURL                                                                                                                                                                                          string
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func LoadConfig() (Config, error) {
	days, e := strconv.Atoi(env("RETENTION_DAYS", "90"))
	if e != nil || days < 1 || days > 365 {
		return Config{}, errors.New("RETENTION_DAYS must be 1..365")
	}
	// 两个内容站使用独立构建和上传目录，部署时可以分别替换而不影响另一产品。
	// Translation provider secrets stay in the Go server environment and never enter either frontend build.
	c := Config{DatabaseURL: os.Getenv("DATABASE_URL"), PublicURL: env("PUBLIC_BASE_URL", "http://localhost:8080"), AdminUser: env("ADMIN_USER", "admin"), AdminPassword: os.Getenv("ADMIN_PASSWORD"), Secret: os.Getenv("APP_SECRET"), CookieMode: env("COOKIE_MODE", "off"), Timezone: env("REPORT_TIMEZONE", "Asia/Shanghai"), GeoDB: os.Getenv("GEOIP_DB_PATH"), Listen: env("LISTEN_ADDR", "127.0.0.1:8080"), FrontendDir: env("FRONTEND_DIR", "../frontend/dist"), LandingDir: env("LANDING_DIR", "../landing/dist"), AudioNovelDir: env("AUDIO_NOVEL_DIR", "../audio-novel/dist"), AudioNovelUploadDir: env("AUDIO_NOVEL_UPLOAD_DIR", "../data/audio-novel-uploads"), AudioNovelAudioDir: env("AUDIO_NOVEL_AUDIO_DIR", "../data/audio-novel-audio"), NovelDir: env("NOVEL_DIR", "../novel-h5/dist"), NovelUploadDir: env("NOVEL_UPLOAD_DIR", "../data/novel-uploads"), RetentionDays: days, APIHZTranslationID: os.Getenv("APIHZ_TRANSLATION_ID"), APIHZTranslationKey: os.Getenv("APIHZ_TRANSLATION_KEY"), APIHZTranslationURL: env("APIHZ_TRANSLATION_URL", "https://cn.apihz.cn/api/zici/fanyiapihz.php"), DeepLAuthKey: os.Getenv("DEEPL_AUTH_KEY"), DeepLTranslationURL: env("DEEPL_TRANSLATION_URL", "https://api-free.deepl.com/v2/translate"), TikTokEventsURL: env("TIKTOK_EVENTS_URL", "https://business-api.tiktok.com/open_api/v1.3/event/track/")}
	tiktokEnabled := strings.ToLower(env("TIKTOK_ENABLED", "false"))
	if tiktokEnabled != "true" && tiktokEnabled != "false" {
		return c, errors.New("TIKTOK_ENABLED must be true or false")
	}
	c.TikTokEnabled = tiktokEnabled == "true"
	c.MetaEncryptionKeyID = os.Getenv("META_ENCRYPTION_KEY_ID")
	if raw := os.Getenv("META_ENCRYPTION_KEYS"); raw != "" {
		if json.Unmarshal([]byte(raw), &c.MetaEncryptionKeys) != nil {
			return c, errors.New("META_ENCRYPTION_KEYS must be a JSON map of key IDs to base64 keys")
		}
	}
	for id, key := range c.MetaEncryptionKeys {
		b, err := base64.StdEncoding.DecodeString(key)
		if err != nil || len(b) != 32 || id == "legacy" || !regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`).MatchString(id) {
			return c, errors.New("Meta encryption key IDs must be 1..40 letters/digits/_/- and keys must be 32 base64-encoded bytes")
		}
	}
	if c.MetaEncryptionKeyID != "" && c.MetaEncryptionKeys[c.MetaEncryptionKeyID] == "" {
		return c, errors.New("META_ENCRYPTION_KEY_ID must select a configured key")
	}
	u, e := url.Parse(c.PublicURL)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return c, errors.New("PUBLIC_BASE_URL must be an origin")
	}
	c.PublicURL = strings.TrimRight(c.PublicURL, "/")
	// The token stays encrypted in PostgreSQL. Endpoint overrides exist for
	// deterministic local tests and controlled HTTPS delivery proxies only.
	tikTokURL, tikTokURLErr := url.Parse(c.TikTokEventsURL)
	localTikTokEndpoint := tikTokURL.Scheme == "http" && (tikTokURL.Hostname() == "127.0.0.1" || tikTokURL.Hostname() == "localhost")
	if tikTokURLErr != nil || tikTokURL.Host == "" || tikTokURL.User != nil || tikTokURL.RawQuery != "" || tikTokURL.Fragment != "" || (tikTokURL.Scheme != "https" && !localTikTokEndpoint) {
		return c, errors.New("TIKTOK_EVENTS_URL must be HTTPS or a local test endpoint")
	}
	c.SecureCookies = u.Scheme == "https"
	if c.DatabaseURL == "" || len(c.Secret) < 32 || len(c.AdminPassword) == 0 || len(c.AdminPassword) > 72 {
		return c, errors.New("DATABASE_URL, APP_SECRET (32+ characters), ADMIN_PASSWORD (1..72 bytes) required")
	}
	if c.CookieMode != "off" && c.CookieMode != "all" {
		return c, errors.New("COOKIE_MODE must be off or all")
	}
	if _, e = time.LoadLocation(c.Timezone); e != nil {
		return c, e
	}
	if p := os.Getenv("TRUSTED_PROXIES"); p != "" {
		c.TrustedProxies = strings.Split(p, ",")
	}
	return c, nil
}
func Location(s string) *time.Location {
	l, e := time.LoadLocation(s)
	if e != nil {
		return time.UTC
	}
	return l
}
