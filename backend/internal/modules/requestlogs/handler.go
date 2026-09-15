package requestlogs

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"whatsapp-analytics/internal/modules/analytics"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct{ *runtime.Core }

func (a *Handler) List(c *gin.Context) {
	f, e := analytics.ParseFilter(c, a.Config.Timezone)
	if e != nil {
		runtime.Bad(c, e.Error())
		return
	}
	page, e := strconv.Atoi(c.DefaultQuery("page", "1"))
	if e != nil || page < 1 || page > 100000 {
		runtime.Bad(c, "页码无效")
		return
	}
	status, e := strconv.Atoi(c.DefaultQuery("status", "0"))
	if e != nil || (status != 0 && (status < 100 || status > 599)) {
		runtime.Bad(c, "状态码无效")
		return
	}
	path := c.Query("path")
	method := c.Query("method")
	if len(path) > 2048 || len(method) > 16 {
		runtime.Bad(c, "筛选条件过长")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	where := ` WHERE is_visitor=true AND path NOT IN ('/favicon.ico','/robots.txt') AND occurred_at >= $1 AND occurred_at < $2 AND ($3::text='' OR strpos(path,$3)>0) AND ($4::text='' OR method=$4) AND ($5::int=0 OR status=$5) `
	args := []any{f.Start, f.End, path, method, status}
	var total int
	if e = a.DB.QueryRow(ctx, "SELECT count(*) FROM request_logs"+where, args...).Scan(&total); e != nil {
		runtime.ServerError(c, e)
		return
	}
	rows, e := a.DB.Query(ctx, "SELECT "+logCols+" FROM request_logs"+where+"ORDER BY occurred_at DESC,id DESC LIMIT 50 OFFSET $6", append(args, (page-1)*50)...)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	defer rows.Close()
	items := []RequestLog{}
	for rows.Next() {
		var v RequestLog
		if e = rows.Scan(&v.ID, &v.OccurredAt, &v.Method, &v.Path, &v.ClientIP, &v.Status, &v.DurationMS, &v.RequestBytes, &v.ResponseBytes, &v.Trigger); e != nil {
			runtime.ServerError(c, e)
			return
		}
		items = append(items, v)
	}
	if rows.Err() != nil {
		runtime.ServerError(c, rows.Err())
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "retention_days": 7, "body_limit": logBodyLimit})
}

func (a *Handler) Detail(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	var v RequestLog
	// 密文只由已鉴权的详情接口读取，列表结构不包含任何可解密的令牌材料。
	var queryTokenCipher string
	e := a.DB.QueryRow(ctx, "SELECT "+logCols+",request_headers,query,request_body,response_headers,response_body,query_token_cipher FROM request_logs WHERE id=$1 AND is_visitor=true AND path NOT IN ('/favicon.ico','/robots.txt')", c.Param("id")).Scan(&v.ID, &v.OccurredAt, &v.Method, &v.Path, &v.ClientIP, &v.Status, &v.DurationMS, &v.RequestBytes, &v.ResponseBytes, &v.Trigger, &v.RequestHeaders, &v.Query, &v.RequestBody, &v.ResponseHeaders, &v.ResponseBody, &queryTokenCipher)
	if e == pgx.ErrNoRows {
		c.Status(http.StatusNotFound)
		return
	}
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	var query map[string]any
	queryErr := json.Unmarshal(v.Query, &query)
	for key := range query {
		if strings.EqualFold(key, "token") {
			v.QueryTokenState = "not_saved"
		}
	}
	if queryTokenCipher != "" {
		v.QueryTokenState = "unavailable"
		tokens, err := a.openQueryToken(v.ID, queryTokenCipher)
		if err == nil && queryErr == nil && query != nil {
			// 先记录查看操作再返回明文；审计记录只保存请求编号，不记录 token。
			if err = a.Audit(ctx, "request_log.token_view", gin.H{"request_id": v.ID}); err != nil {
				runtime.ServerError(c, err)
				return
			}
			for key, values := range tokens {
				query[key] = values
			}
			v.Query = logJSON(query)
			v.QueryTokenState = "available"
		}
	}
	c.JSON(200, v)
}
