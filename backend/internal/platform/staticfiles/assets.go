package staticfiles

import (
	"crypto/sha256"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var versionedAsset = regexp.MustCompile(`^[^/]+-[A-Za-z0-9_-]{8,}\.(js|css)$`)

// Static public files only: HTML and API responses retain the global no-store policy.
func Handler(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := strings.TrimPrefix(c.Param("filepath"), "/")
		if name == "" || strings.Contains(name, "\\") || !filepath.IsLocal(name) || filepath.ToSlash(filepath.Clean(name)) != name {
			c.Status(404)
			return
		}
		file, err := os.Open(filepath.Join(root, name))
		if err != nil {
			c.Status(404)
			return
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			c.Status(404)
			return
		}
		ext := filepath.Ext(name)
		if ext == ".js" || ext == ".css" {
			c.Header("Vary", "Accept-Encoding")
			for _, encoding := range acceptedCompressions(c.GetHeader("Accept-Encoding")) {
				compressed, err := os.Open(filepath.Join(root, name+"."+encoding.suffix))
				if err != nil {
					continue
				}
				stat, err := compressed.Stat()
				if err != nil || !stat.Mode().IsRegular() {
					compressed.Close()
					continue
				}
				defer compressed.Close()
				file = compressed
				info = stat
				c.Header("Content-Encoding", encoding.name)
				break
			}
		}
		if versionedAsset.MatchString(name) {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "public, max-age=0, must-revalidate")
			hash := sha256.New()
			if _, err := io.Copy(hash, file); err != nil {
				c.Header("Cache-Control", "no-store")
				c.Status(500)
				return
			}
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				c.Header("Cache-Control", "no-store")
				c.Status(500)
				return
			}
			c.Header("ETag", fmt.Sprintf("\"%x\"", hash.Sum(nil)))
		}
		if contentType := mime.TypeByExtension(ext); contentType != "" {
			c.Header("Content-Type", contentType)
		}
		http.ServeContent(c.Writer, c.Request, name, info.ModTime(), file)
	}
}

type compression struct {
	name, suffix string
	q            float64
}

func acceptedCompressions(header string) []compression {
	quality := map[string]float64{}
	for _, entry := range strings.Split(header, ",") {
		parts := strings.Split(strings.TrimSpace(entry), ";")
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		q := 1.0
		for _, parameter := range parts[1:] {
			pair := strings.SplitN(strings.TrimSpace(parameter), "=", 2)
			if len(pair) == 2 && strings.EqualFold(pair[0], "q") {
				var err error
				q, err = strconv.ParseFloat(pair[1], 64)
				if err != nil || q < 0 || q > 1 {
					q = 0
				}
			}
		}
		quality[name] = q
	}
	out := []compression{}
	for _, encoding := range []compression{{"br", "br", 0}, {"gzip", "gz", 0}} {
		q, ok := quality[encoding.name]
		if !ok {
			q = quality["*"]
		}
		if q > 0 {
			encoding.q = q
			out = append(out, encoding)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].q > out[j].q })
	return out
}
