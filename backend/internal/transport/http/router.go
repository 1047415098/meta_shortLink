package httptransport

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/modules/adspend"
	"whatsapp-analytics/internal/modules/analytics"
	"whatsapp-analytics/internal/modules/audionovel"
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
	Auth       *auth.Handler
	Links      *links.Handler
	Analytics  *analytics.Handler
	Spend      *adspend.Handler
	Logs       *requestlogs.Handler
	Landing    *landing.Handler
	AudioNovel *audionovel.Handler
	Tracking   *tracking.Handler
	Meta       *meta.Handler
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
		// 封面上传最大 5MB，额外空间用于 multipart 边界和字段头。
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 6<<20)
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
	api.GET("/audio-novels", h.AudioNovel.ListAdmin)
	api.POST("/audio-novels", h.AudioNovel.CreateAdmin)
	api.GET("/audio-novels/:id", h.AudioNovel.GetAdmin)
	api.PATCH("/audio-novels/:id", h.AudioNovel.UpdateAdmin)
	api.DELETE("/audio-novels/:id", h.AudioNovel.DeleteAdmin)
	api.PATCH("/audio-novels/:id/status", h.AudioNovel.SetEnabled)
	api.PATCH("/audio-novels/:id/featured", h.AudioNovel.SetFeatured)
	api.POST("/audio-novels/preview", h.AudioNovel.Preview)
	api.POST("/audio-novel-covers", h.AudioNovel.UploadCover)
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
	for prefix, root := range map[string]string{"/admin-assets/": filepath.Join(core.Config.FrontendDir, "admin-assets"), "/landing-assets/": filepath.Join(core.Config.LandingDir, "landing-assets"), "/audio-novel-assets/": filepath.Join(core.Config.AudioNovelDir, "audio-novel-assets"), "/assets/": filepath.Join(core.Config.FrontendDir, "assets")} {
		r.GET(prefix+"*filepath", staticfiles.Handler(root))
		r.HEAD(prefix+"*filepath", staticfiles.Handler(root))
	}
	// Uploaded covers remain independent from frontend build artifacts and use persistent storage.
	r.GET("/audio-novel-uploads/*filepath", staticfiles.Handler(core.Config.AudioNovelUploadDir))
	r.HEAD("/audio-novel-uploads/*filepath", staticfiles.Handler(core.Config.AudioNovelUploadDir))
	r.GET("/audio-novel-api/:code/home", h.AudioNovel.PublicHome)
	r.GET("/audio-novel-api/:code/stories", h.AudioNovel.PublicList)
	r.GET("/audio-novel-api/:code/stories/:slug", h.AudioNovel.PublicStory)
	// Audio novel routes stay before the generic short-code route so the product prefix is never treated as a code.
	r.GET("/audio-novel/:code", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code", h.Tracking.AudioNovel)
	r.GET("/audio-novel/:code/stories", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code/stories", h.Tracking.AudioNovel)
	r.GET("/audio-novel/:code/stories/:slug", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code/stories/:slug", h.Tracking.AudioNovel)
	r.POST("/audio-novel/:code/contact", h.AudioNovel.Contact)
	r.POST("/audio-novel/:code/view", h.AudioNovel.View)
	// TimeSpent reuses the signed visit while remaining scoped to the audio novel surface.
	r.POST("/audio-novel/:code/time-spent", h.AudioNovel.TimeSpent)
	r.POST("/:code/contact", h.Landing.Contact)
	r.POST("/:code/view", h.Landing.View)
	r.POST("/:code/time-spent", h.Landing.TimeSpent)
	r.GET("/:code", h.Tracking.Redirect)
	r.HEAD("/:code", h.Tracking.Redirect)
	return r, nil
}
