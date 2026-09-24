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
	"whatsapp-analytics/internal/modules/novel"
	"whatsapp-analytics/internal/modules/requestlogs"
	"whatsapp-analytics/internal/modules/tiktok"
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
	Novel      *novel.Handler
	Tracking   *tracking.Handler
	Meta       *meta.Handler
	TikTok     *tiktok.Handler
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
		requestLimit := int64(6 << 20)
		if c.Request.URL.Path == "/api/v1/audio-novel-audio" {
			// MP3 本体上限 100 MiB，额外 1 MiB 留给 multipart 边界和字段头。
			requestLimit = 101 << 20
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, requestLimit)
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
	if h.TikTok != nil {
		h.TikTok.Register(api)
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
	api.DELETE("/audio-novels/:id/audio", h.AudioNovel.RemoveAudio)
	api.PATCH("/audio-novels/:id/status", h.AudioNovel.SetEnabled)
	api.PATCH("/audio-novels/:id/featured", h.AudioNovel.SetFeatured)
	api.POST("/audio-novels/preview", h.AudioNovel.Preview)
	api.POST("/audio-novel-covers", h.AudioNovel.UploadCover)
	api.POST("/audio-novel-audio", h.AudioNovel.UploadAudio)
	// 语音投放链接独立于普通短链和文字小说链接，避免三个前端项目的数据混在一起。
	api.GET("/audio-novel-links", h.AudioNovel.ListDistributionLinks)
	api.POST("/audio-novel-links", h.AudioNovel.CreateDistributionLink)
	api.PATCH("/audio-novel-links/:id", h.AudioNovel.UpdateDistributionLink)
	api.DELETE("/audio-novel-links/:id", h.AudioNovel.DeleteDistributionLink)
	// Audio campaign statistics keep playback funnels isolated by link and bound content.
	api.POST("/audio-novel-links/:id/stats", h.AudioNovel.DistributionStats)
	api.GET("/novels", h.Novel.ListAdmin)
	api.POST("/novels", h.Novel.CreateAdmin)
	api.GET("/novels/:id", h.Novel.GetAdmin)
	api.PATCH("/novels/:id", h.Novel.UpdateAdmin)
	// 封面上传后独立保存，列表刷新即可读取最新 cover_path。
	api.PATCH("/novels/:id/cover", h.Novel.UpdateCover)
	api.DELETE("/novels/:id", h.Novel.DeleteAdmin)
	api.PATCH("/novels/:id/status", h.Novel.SetEnabled)
	api.PATCH("/novels/:id/featured", h.Novel.SetFeatured)
	api.GET("/novels/:id/chapters", h.Novel.ListChaptersAdmin)
	api.POST("/novels/:id/chapters", h.Novel.CreateChapterAdmin)
	api.PATCH("/novels/:id/chapters/:chapterId", h.Novel.UpdateChapterAdmin)
	api.DELETE("/novels/:id/chapters/:chapterId", h.Novel.DeleteChapterAdmin)
	api.POST("/novels/preview", h.Novel.Preview)
	api.POST("/novel-covers", h.Novel.UploadCover)
	api.GET("/novels/:id/translations", h.Novel.ListTranslations)
	api.POST("/novels/:id/translations", h.Novel.GenerateTranslations)
	api.PATCH("/novels/:id/translations/:locale/status", h.Novel.SetTranslationEnabled)
	// 小说投放链接使用独立接口，避免普通短链接列表混入其他前端项目的数据。
	api.GET("/novel-links", h.Novel.ListDistributionLinks)
	api.POST("/novel-links", h.Novel.CreateDistributionLink)
	api.PATCH("/novel-links/:id", h.Novel.UpdateDistributionLink)
	api.DELETE("/novel-links/:id", h.Novel.DeleteDistributionLink)
	api.POST("/novel-links/:id/stats", h.Novel.DistributionStats)
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
	for prefix, root := range map[string]string{"/admin-assets/": filepath.Join(core.Config.FrontendDir, "admin-assets"), "/landing-assets/": filepath.Join(core.Config.LandingDir, "landing-assets"), "/audio-novel-assets/": filepath.Join(core.Config.AudioNovelDir, "audio-novel-assets"), "/novel-assets/": filepath.Join(core.Config.NovelDir, "novel-assets"), "/assets/": filepath.Join(core.Config.FrontendDir, "assets")} {
		r.GET(prefix+"*filepath", staticfiles.Handler(root))
		r.HEAD(prefix+"*filepath", staticfiles.Handler(root))
	}
	// Uploaded covers remain independent from frontend build artifacts and use persistent storage.
	r.GET("/audio-novel-uploads/*filepath", staticfiles.Handler(core.Config.AudioNovelUploadDir))
	r.HEAD("/audio-novel-uploads/*filepath", staticfiles.Handler(core.Config.AudioNovelUploadDir))
	// MP3 使用独立持久目录，http.ServeContent 原生处理播放器需要的 Range 请求。
	r.GET("/audio-novel-audio/*filepath", staticfiles.Handler(core.Config.AudioNovelAudioDir))
	r.HEAD("/audio-novel-audio/*filepath", staticfiles.Handler(core.Config.AudioNovelAudioDir))
	r.GET("/audio-novel-api/:code/home", h.AudioNovel.PublicHome)
	r.GET("/audio-novel-api/:code/stories", h.AudioNovel.PublicList)
	r.GET("/audio-novel-api/:code/stories/:slug", h.AudioNovel.PublicStory)
	r.GET("/audio-novel-api/:code/audio", h.AudioNovel.PublicAudioList)
	r.GET("/audio-novel-api/:code/audio/:slug", h.AudioNovel.PublicAudioStory)
	r.GET("/novel-uploads/*filepath", staticfiles.Handler(core.Config.NovelUploadDir))
	r.HEAD("/novel-uploads/*filepath", staticfiles.Handler(core.Config.NovelUploadDir))
	r.GET("/novel-api/:code/home", h.Novel.PublicHome)
	r.GET("/novel-api/:code/stories", h.Novel.PublicList)
	r.GET("/novel-api/:code/stories/:slug", h.Novel.PublicStory)
	r.GET("/novel-api/:code/stories/:slug/chapters/:number", h.Novel.PublicChapter)
	// 免费小说路由必须位于通用短码之前，避免 novel 被误识别为一个广告短码。
	// Every Vue history route must return the H5 bootstrap on a direct visit or browser refresh.
	for _, path := range []string{"/novel/:code", "/novel/:code/search", "/novel/:code/stories", "/novel/:code/stories/:slug", "/novel/:code/stories/:slug/chapters/:chapter"} {
		r.GET(path, h.Tracking.Novel)
		r.HEAD(path, h.Tracking.Novel)
	}
	r.POST("/novel/:code/view", h.Novel.View)
	r.POST("/novel/:code/start-reading", h.Novel.StartReading)
	r.POST("/novel/:code/time-spent", h.Novel.TimeSpent)
	r.POST("/novel/:code/reading-time", h.Novel.ReadingTime)
	// Audio novel routes stay before the generic short-code route so the product prefix is never treated as a code.
	r.GET("/audio-novel/:code", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code", h.Tracking.AudioNovel)
	r.GET("/audio-novel/:code/audio", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code/audio", h.Tracking.AudioNovel)
	r.GET("/audio-novel/:code/audio/:slug", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code/audio/:slug", h.Tracking.AudioNovel)
	r.GET("/audio-novel/:code/stories", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code/stories", h.Tracking.AudioNovel)
	r.GET("/audio-novel/:code/stories/:slug", h.Tracking.AudioNovel)
	r.HEAD("/audio-novel/:code/stories/:slug", h.Tracking.AudioNovel)
	r.POST("/audio-novel/:code/contact", h.AudioNovel.Contact)
	r.POST("/audio-novel/:code/view", h.AudioNovel.View)
	// TimeSpent reuses the signed visit while remaining scoped to the audio novel surface.
	r.POST("/audio-novel/:code/time-spent", h.AudioNovel.TimeSpent)
	// Playback tickets stay in JSON request bodies; Beacon may use text/plain JSON on pagehide.
	r.POST("/audio-novel/:code/start-listening", h.AudioNovel.StartListening)
	r.POST("/audio-novel/:code/playback-time", h.AudioNovel.PlaybackTime)
	r.POST("/audio-novel/:code/complete", h.AudioNovel.Complete)
	// Foreground-visible time is independent from playback and never triggers advertising events.
	r.POST("/audio-novel/:code/visible-time", h.AudioNovel.VisibleTime)
	r.POST("/:code/contact", h.Landing.Contact)
	r.POST("/:code/view", h.Landing.View)
	r.POST("/:code/time-spent", h.Landing.TimeSpent)
	r.GET("/:code", h.Tracking.Redirect)
	r.HEAD("/:code", h.Tracking.Redirect)
	return r, nil
}
