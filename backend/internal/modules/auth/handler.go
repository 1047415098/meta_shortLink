package auth

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	Password []byte
}

func (a *Handler) Login(c *gin.Context) {
	if a.Exceed("login:"+c.ClientIP(), 10, 15*time.Minute) {
		c.JSON(429, gin.H{"error": "尝试过于频繁，请稍后再试"})
		return
	}
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&in) != nil {
		runtime.Bad(c, "无效登录请求")
		return
	}
	e := bcrypt.CompareHashAndPassword(a.Password, []byte(in.Password))
	if e != nil || in.Username != a.Config.AdminUser {
		c.JSON(401, gin.H{"error": "用户名或密码错误"})
		return
	}
	v := runtime.Token()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if e = (Repository{DB: a.DB}).Create(ctx, runtime.Digest(v)); e != nil {
		runtime.ServerError(c, e)
		return
	}
	a.SetCookie(c, a.SessionName(), v, 43200)
	c.JSON(200, gin.H{"username": a.Config.AdminUser})
}
func (a *Handler) Require(c *gin.Context) {
	v, e := c.Cookie(a.SessionName())
	if e != nil {
		c.AbortWithStatusJSON(401, gin.H{"error": "请先登录"})
		return
	}
	var ok bool
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	ok, e = (Repository{DB: a.DB}).Valid(ctx, runtime.Digest(v))
	if e != nil {
		c.AbortWithStatusJSON(503, gin.H{"error": "服务暂不可用"})
		return
	}
	if !ok {
		c.AbortWithStatusJSON(401, gin.H{"error": "登录已过期"})
		return
	}
	c.Next()
}
func (a *Handler) Logout(c *gin.Context) {
	v, _ := c.Cookie(a.SessionName())
	e := (Repository{DB: a.DB}).Delete(c.Request.Context(), runtime.Digest(v))
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	a.SetCookie(c, a.SessionName(), "", -1)
	c.JSON(200, gin.H{"ok": true})
}
