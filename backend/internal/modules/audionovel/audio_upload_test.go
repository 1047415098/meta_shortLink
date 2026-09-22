package audionovel

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/platform/runtime"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for index := range p {
		p[index] = 0
	}
	return len(p), nil
}

func uploadAudioRequest(t *testing.T, handler *Handler, content io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	go func() {
		part, err := multipartWriter.CreateFormFile("file", "story.mp3")
		if err == nil {
			_, err = io.Copy(part, content)
		}
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		pipeWriter.CloseWithError(err)
	}()

	router := gin.New()
	router.POST("/upload", handler.UploadAudio)
	request := httptest.NewRequest(http.MethodPost, "/upload", pipeReader)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func TestUploadAudioAcceptsMP3HeadersAndUsesRandomNames(t *testing.T) {
	dir := t.TempDir()
	handler := &Handler{Core: runtime.New(config.Config{AudioNovelAudioDir: dir}, nil)}
	paths := map[string]bool{}
	for _, payload := range [][]byte{
		append([]byte("ID3"), bytes.Repeat([]byte{1}, 32)...),
		append([]byte{0xff, 0xfb}, bytes.Repeat([]byte{2}, 32)...),
	} {
		response := uploadAudioRequest(t, handler, bytes.NewReader(payload))
		if response.Code != http.StatusOK {
			t.Fatalf("upload response = %d %s", response.Code, response.Body.String())
		}
		var result struct {
			Path      string `json:"path"`
			SizeBytes int64  `json:"size_bytes"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if !audioNovelAudioPattern.MatchString(result.Path) || result.SizeBytes != int64(len(payload)) {
			t.Fatalf("unexpected upload result: %+v", result)
		}
		if paths[result.Path] {
			t.Fatalf("duplicate random path %q", result.Path)
		}
		paths[result.Path] = true
		if _, err := os.Stat(filepath.Join(dir, filepath.Base(result.Path))); err != nil {
			t.Fatalf("uploaded file missing: %v", err)
		}
	}
}

func TestUploadAudioRejectsInvalidEmptyAndOversizedFilesWithoutTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	handler := &Handler{Core: runtime.New(config.Config{AudioNovelAudioDir: dir}, nil)}
	cases := []io.Reader{
		strings.NewReader("not an mp3"),
		strings.NewReader(""),
		io.MultiReader(strings.NewReader("ID3"), io.LimitReader(zeroReader{}, maxAudioBytes-2)),
	}
	for index, content := range cases {
		response := uploadAudioRequest(t, handler, content)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("case %d response = %d %s", index, response.Code, response.Body.String())
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read upload directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("rejected uploads left files: %v", entries)
	}
}

func TestMP3HeaderValidation(t *testing.T) {
	for _, valid := range [][]byte{{'I', 'D', '3'}, {0xff, 0xe0}, {0xff, 0xfb}} {
		if !isMP3Header(valid) {
			t.Fatalf("valid header rejected: %v", valid)
		}
	}
	for _, invalid := range [][]byte{{}, {'I', 'D'}, {0xff, 0x00}, {'R', 'I', 'F', 'F'}} {
		if isMP3Header(invalid) {
			t.Fatalf("invalid header accepted: %v", invalid)
		}
	}
}
