package landing

import (
	"encoding/json"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/platform/runtime"
)

// RenderDirect emits only the requested top.location handoff. The tracking
// handler records the incoming visit before this response is written.
func (a *Handler) RenderDirect(c *gin.Context, l links.Link) {
	target, err := json.Marshal(l.TargetURL)
	if err != nil {
		landingError(c, err)
		return
	}
	nonce := runtime.Token()
	// Keep the inline script protected by a per-response nonce while avoiding
	// the removed intermediate page, buttons, app protocol and action beacons.
	page := []byte("<script language=\"javascript\" nonce=\"" + nonce + "\">\ntop.location = " + string(target) + ";\n</script>")
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; base-uri 'none'; frame-ancestors 'none'")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Content-Type", "text/html; charset=utf-8")
	if c.Request.Method == "HEAD" {
		c.Status(200)
		return
	}
	c.Data(200, "text/html; charset=utf-8", page)
}
