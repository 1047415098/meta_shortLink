package audionovel

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/platform/runtime"
)

const maxAudioBytes = 100 << 20

func isMP3Header(data []byte) bool {
	// MP3 可以用 ID3 标签开头，也可以直接以 MPEG 音频帧同步字开头。
	return len(data) >= 3 && string(data[:3]) == "ID3" || len(data) >= 2 && data[0] == 0xff && data[1]&0xe0 == 0xe0
}

func audioFilePart(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FormName() == "file" && part.FileName() != "" {
			return part, nil
		}
		part.Close()
	}
}

func (a *Handler) UploadAudio(c *gin.Context) {
	reader, err := c.Request.MultipartReader()
	if err != nil {
		runtime.Bad(c, "请选择 MP3 文件")
		return
	}
	part, err := audioFilePart(reader)
	if err != nil {
		runtime.Bad(c, "请选择 MP3 文件")
		return
	}
	defer part.Close()

	header := make([]byte, 3)
	headerSize, readErr := io.ReadFull(part, header)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		runtime.Bad(c, "读取音频失败")
		return
	}
	header = header[:headerSize]
	if !isMP3Header(header) {
		runtime.Bad(c, "只支持有效的 MP3 文件")
		return
	}
	if err = os.MkdirAll(a.Config.AudioNovelAudioDir, 0750); err != nil {
		runtime.ServerError(c, err)
		return
	}
	temporary, err := os.CreateTemp(a.Config.AudioNovelAudioDir, ".upload-*")
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	size, err := temporary.Write(header)
	if err == nil {
		var copied int64
		copied, err = io.Copy(temporary, io.LimitReader(part, maxAudioBytes+1-int64(size)))
		size += int(copied)
	}
	if err == nil && int64(size) > maxAudioBytes {
		err = errors.New("音频不能超过 100MB")
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		if err.Error() == "音频不能超过 100MB" {
			runtime.Bad(c, err.Error())
		} else {
			runtime.ServerError(c, err)
		}
		return
	}

	token := make([]byte, 16)
	if _, err = rand.Read(token); err != nil {
		runtime.ServerError(c, err)
		return
	}
	name := hex.EncodeToString(token) + ".mp3"
	// 同一文件系统内原子改名，避免静态路由读到尚未写完的 MP3。
	if err = os.Rename(temporaryName, filepath.Join(a.Config.AudioNovelAudioDir, name)); err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"path": "/audio-novel-audio/" + name, "size_bytes": size})
}
