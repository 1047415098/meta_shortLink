package adspend

import (
	"context"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct{ *runtime.Core }

var amountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,9})(\.[0-9]{1,4})?$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

func (a *Handler) Import(c *gin.Context) {
	file, _, e := c.Request.FormFile("file")
	if e != nil {
		runtime.Bad(c, "请选择 CSV 文件（最多 2 MB）")
		return
	}
	defer file.Close()
	records, e := Parse(file)
	if e != nil {
		runtime.Bad(c, e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	if e = (Repository{DB: a.DB}).Import(ctx, records, a.Config.AdminUser); e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, gin.H{"imported": len(records)})
}
