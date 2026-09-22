package bootstrap

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oschwald/geoip2-golang"
	"golang.org/x/crypto/bcrypt"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/jobs"
	"whatsapp-analytics/internal/modules/adspend"
	"whatsapp-analytics/internal/modules/analytics"
	"whatsapp-analytics/internal/modules/audionovel"
	"whatsapp-analytics/internal/modules/auth"
	"whatsapp-analytics/internal/modules/landing"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/modules/novel"
	"whatsapp-analytics/internal/modules/requestlogs"
	"whatsapp-analytics/internal/modules/tracking"
	"whatsapp-analytics/internal/platform/database"
	"whatsapp-analytics/internal/platform/runtime"
	httptransport "whatsapp-analytics/internal/transport/http"
)

type App struct {
	*runtime.Core
	Router *gin.Engine
	Meta   *meta.Service
}

func New(c config.Config, db *pgxpool.Pool) (*App, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, db); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(c.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	core := runtime.New(c, db)
	if c.GeoDB != "" {
		core.Geo, err = geoip2.Open(c.GeoDB)
		if err != nil {
			return nil, err
		}
	}
	landingHandler := &landing.Handler{Core: core}
	metaService := meta.New(core)
	landingHandler.Meta = metaService
	// The standalone audio novel app reuses the shared link, tracking and Meta services.
	audioNovelHandler := &audionovel.Handler{Core: core, Meta: metaService}
	novelHandler := &novel.Handler{Core: core, Meta: metaService}
	trackingHandler := &tracking.Handler{Core: core, Landing: landingHandler, AudioNovelPage: audioNovelHandler, NovelPage: novelHandler}
	router, err := httptransport.New(core, httptransport.Handlers{
		Auth:       &auth.Handler{Core: core, Password: hash},
		Links:      &links.Handler{Core: core},
		Analytics:  &analytics.Handler{Core: core},
		Spend:      &adspend.Handler{Core: core},
		Logs:       &requestlogs.Handler{Core: core},
		Landing:    landingHandler,
		AudioNovel: audioNovelHandler,
		Novel:      novelHandler,
		Tracking:   trackingHandler,
		Meta:       &meta.Handler{Service: metaService},
	})
	if err != nil {
		if core.Geo != nil {
			core.Geo.Close()
		}
		return nil, err
	}
	return &App{Core: core, Router: router, Meta: metaService}, nil
}
func (a *App) Close() {
	a.Meta.Close()
	if a.Geo != nil {
		a.Geo.Close()
	}
}
func (a *App) Retention(ctx context.Context) { jobs.Retention(ctx, a.DB, a.Config.RetentionDays) }
