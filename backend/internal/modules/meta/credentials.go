package meta

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"time"
	"whatsapp-analytics/internal/platform/runtime"
)

func (h *Handler) registerCredentials(g *gin.RouterGroup) {
	g.GET("/credentials", h.credentials)
	g.GET("/audit", h.audit)
	g.POST("/credentials/rewrap", h.rewrap)
}
func (h *Handler) credentials(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	accounts, e := h.Service.Connections(ctx)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	pixels, e := h.Service.Pixels(ctx, 0)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	items := []gin.H{}
	names := map[int64]Connection{}
	for _, a := range accounts {
		names[a.ID] = a
	}
	for _, p := range pixels {
		a := names[p.ConnectionID]
		// Only Pixel CAPI credentials are part of the active product surface.
		items = append(items, gin.H{"kind": "capi", "connection_id": p.ConnectionID, "pixel_record_id": p.ID, "name": p.Name, "account_id": a.AccountID, "pixel_id": p.PixelID, "configured": p.HasCapiToken, "status": p.CredentialStatus, "expires_at": p.TokenExpiresAt, "validated_at": p.ValidatedAt, "last_error": p.LastError})
	}
	c.JSON(200, gin.H{"items": items, "encryption_key_id": h.Service.encryptionKeyID()})
}
func (h *Handler) audit(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	rows, e := h.Service.Core.DB.Query(ctx, "SELECT id,actor,action,detail,occurred_at FROM audit_logs WHERE action LIKE 'meta.%' ORDER BY id DESC LIMIT 100")
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id int64
		var actor, action string
		var detail json.RawMessage
		var at time.Time
		if e = rows.Scan(&id, &actor, &action, &detail, &at); e != nil {
			runtime.ServerError(c, e)
			return
		}
		items = append(items, gin.H{"id": id, "actor": actor, "action": action, "detail": detail, "created_at": at})
	}
	if e = rows.Err(); e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items})
}
func (h *Handler) rewrap(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	n, e := h.Service.Rewrap(ctx)
	if e != nil {
		runtime.Bad(c, e.Error())
		return
	}
	c.JSON(200, gin.H{"updated": n, "encryption_key_id": h.Service.encryptionKeyID()})
}
