package audionovel

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"whatsapp-analytics/internal/platform/runtime"
)

const maxCoverBytes = 5 << 20

func coverExtension(data []byte) (string, error) {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", errors.New("只支持 JPEG、PNG 或 WebP 图片")
	}
}

func (a *Handler) UploadCover(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		runtime.Bad(c, "请选择封面文件")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxCoverBytes+1))
	if err != nil {
		runtime.Bad(c, "读取封面失败")
		return
	}
	if len(data) > maxCoverBytes {
		runtime.Bad(c, "封面不能超过 5MB")
		return
	}
	ext, err := coverExtension(data)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	if err = os.MkdirAll(a.Config.AudioNovelUploadDir, 0750); err != nil {
		runtime.ServerError(c, err)
		return
	}
	temporary, err := os.CreateTemp(a.Config.AudioNovelUploadDir, ".upload-*")
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	token := make([]byte, 16)
	if _, err = rand.Read(token); err != nil {
		runtime.ServerError(c, err)
		return
	}
	name := hex.EncodeToString(token) + ext
	if err = os.Rename(temporaryName, filepath.Join(a.Config.AudioNovelUploadDir, name)); err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"path": "/audio-novel-uploads/" + name})
}
