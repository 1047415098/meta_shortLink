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

// statusFilter 兼容已缓存前端发送的空字符串，同时接受新的数字状态码。
type statusFilter int

func (s *statusFilter) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" || raw == `""` {
		*s = 0
		return nil
	}
	if strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`) {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		raw = strings.TrimSpace(value)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return err
	}
	*s = statusFilter(value)
	return nil
}

type listInput struct {
	analytics.FilterInput
	Path   string       `json:"path"`
	Method string       `json:"method"`
	Status statusFilter `json:"status"`
}

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

// ListJSON keeps operator log filters in the JSON request body.
func (a *Handler) ListJSON(c *gin.Context) {
	var input listInput
	if err := c.ShouldBindJSON(&input); err != nil {
		runtime.Bad(c, "日志筛选条件格式无效")
		return
	}
	f, _, err := analytics.ParseFilterInput(input.FilterInput, a.Config.Timezone)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	a.list(c, f, input.Page, input.Path, input.Method, int(input.Status))
}

func (a *Handler) list(c *gin.Context, f analytics.Filter, page int, path, method string, status int) {
	if page == 0 {
		page = 1
	}
	if page < 1 || page > 100000 {
		runtime.Bad(c, "页码无效")
		return
	}
	if status != 0 && (status < 100 || status > 599) {
		runtime.Bad(c, "状态码无效")
		return
	}
	if len(path) > 2048 || len(method) > 16 {
		runtime.Bad(c, "筛选条件过长")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	where := ` WHERE is_visitor=true AND path NOT IN ('/favicon.ico','/robots.txt') AND occurred_at >= $1 AND occurred_at < $2 AND ($3::text='' OR strpos(path,$3)>0) AND ($4::text='' OR method=$4) AND ($5::int=0 OR status=$5) `
	args := []any{f.Start, f.End, path, method, status}
	var total int
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM request_logs"+where, args...).Scan(&total); err != nil {
		runtime.ServerError(c, err)
		return
	}
	rows, err := a.DB.Query(ctx, "SELECT "+logCols+" FROM request_logs"+where+"ORDER BY occurred_at DESC,id DESC LIMIT 50 OFFSET $6", append(args, (page-1)*50)...)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	defer rows.Close()
	items := []RequestLog{}
	for rows.Next() {
		var item RequestLog
		if err := rows.Scan(&item.ID, &item.OccurredAt, &item.Method, &item.Path, &item.ClientIP, &item.Status, &item.DurationMS, &item.RequestBytes, &item.ResponseBytes, &item.Trigger); err != nil {
			runtime.ServerError(c, err)
			return
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		runtime.ServerError(c, err)
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
