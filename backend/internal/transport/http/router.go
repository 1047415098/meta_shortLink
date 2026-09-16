package httptransport

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/modules/adspend"
	"whatsapp-analytics/internal/modules/analytics"
	"whatsapp-analytics/internal/modules/auth"
	"whatsapp-analytics/internal/modules/landing"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/modules/requestlogs"
	"whatsapp-analytics/internal/modules/tracking"
	"whatsapp-analytics/internal/platform/runtime"
	"whatsapp-analytics/internal/platform/staticfiles"
)

type Handlers struct {
	Auth      *auth.Handler
	Links     *links.Handler
	Analytics *analytics.Handler
	Spend     *adspend.Handler
	Logs      *requestlogs.Handler
	Landing   *landing.Handler
	Tracking  *tracking.Handler
	Meta      *meta.Handler
}

func New(core *runtime.Core, h Handlers) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(h.Logs.Middleware(), gin.Recovery())
	if e := r.SetTrustedProxies(core.Config.TrustedProxies); e != nil {
		return nil, e
	}
	r.Use(func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Cache-Control", "no-store")
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
		c.Next()
	})
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		defer cancel()
		if core.DB.Ping(ctx) != nil {
			c.JSON(503, gin.H{"status": "degraded"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})
	api := r.Group("/api/v1")
	api.Use(func(c *gin.Context) {
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" && c.GetHeader("X-Requested-With") != "XMLHttpRequest" {
			c.AbortWithStatusJSON(403, gin.H{"error": "请求校验失败"})
			return
		}
		c.Next()
	})
	api.POST("/auth/login", h.Auth.Login)
	api.Use(h.Auth.Require)
	if h.Meta != nil {
		h.Meta.Register(api)
	}
	api.GET("/auth/me", func(c *gin.Context) { c.JSON(200, gin.H{"username": core.Config.AdminUser}) })
	api.POST("/auth/logout", h.Auth.Logout)
	api.GET("/links", h.Links.List)
	// Statistics filters are submitted as JSON; POST retains both login and request-header validation.
	api.POST("/links/:id/stats", h.Analytics.LinkStats)
	api.POST("/links", h.Links.Create)
	// Bulk deletion uses POST so proxies reliably forward the validated JSON selection.
	api.POST("/links/batch-delete", h.Links.DeleteBatch)
	api.PATCH("/links/:id", h.Links.Update)
	api.GET("/analytics", h.Analytics.Overview)
	api.GET("/clicks", h.Analytics.Clicks)
	api.GET("/exports/clicks", h.Analytics.Export)
	api.POST("/ad-spend/import", h.Spend.Import)
	api.GET("/request-logs", h.Logs.List)
	api.GET("/request-logs/:id", h.Logs.Detail)
	api.GET("/settings", func(c *gin.Context) {
		c.JSON(200, gin.H{"public_base_url": core.Config.PublicURL, "timezone": core.Config.Timezone, "cookie_mode": core.Config.CookieMode, "retention_days": core.Config.RetentionDays, "geo_enabled": core.Geo != nil})
	})
	r.GET("/favicon.ico", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.HEAD("/favicon.ico", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	r.GET("/robots.txt", func(c *gin.Context) { c.String(http.StatusOK, "User-agent: *\nDisallow: /admin/\nDisallow: /api/\n") })
	r.GET("/", func(c *gin.Context) { c.Redirect(302, "/admin/overview") })
	adminIndex := func(c *gin.Context) { c.File(filepath.Join(core.Config.FrontendDir, "index.html")) }
	r.GET("/admin", adminIndex)
	r.HEAD("/admin", adminIndex)
	r.GET("/admin/*path", adminIndex)
	r.HEAD("/admin/*path", adminIndex)
	for prefix, root := range map[string]string{"/admin-assets/": filepath.Join(core.Config.FrontendDir, "admin-assets"), "/landing-assets/": filepath.Join(core.Config.LandingDir, "landing-assets"), "/assets/": filepath.Join(core.Config.FrontendDir, "assets")} {
		r.GET(prefix+"*filepath", staticfiles.Handler(root))
		r.HEAD(prefix+"*filepath", staticfiles.Handler(root))
	}
	r.POST("/:code/contact", h.Landing.Contact)
	r.POST("/:code/view", h.Landing.View)
	r.GET("/:code", h.Tracking.Redirect)
	r.HEAD("/:code", h.Tracking.Redirect)
	return r, nil
}
