package staticfiles

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAudioRangeDeliveryReturnsOnlyRequestedBytes(t *testing.T) {
	dir := t.TempDir()
	content := []byte(strings.Repeat("0123456789abcdef", 4))
	if err := os.WriteFile(filepath.Join(dir, "sample.mp3"), content, 0600); err != nil {
		t.Fatalf("write audio fixture: %v", err)
	}
	router := gin.New()
	router.GET("/audio-novel-audio/*filepath", Handler(dir))

	request := httptest.NewRequest(http.MethodGet, "/audio-novel-audio/sample.mp3", nil)
	request.Header.Set("Range", "bytes=0-15")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusPartialContent || response.Body.Len() != 16 {
		t.Fatalf("range response = %d/%d", response.Code, response.Body.Len())
	}
	if response.Body.String() != "0123456789abcdef" {
		t.Fatalf("range payload = %q", response.Body.String())
	}
}

func TestAssetDelivery(t *testing.T) {
	dir := t.TempDir()
	source := []byte(strings.Repeat("const message='hello';\n", 100))
	os.WriteFile(filepath.Join(dir, "index-abcdefgh.js"), source, 0600)
	os.WriteFile(filepath.Join(dir, "product.webp"), []byte("image"), 0600)
	// Content-versioned images must keep the same immutable cache contract as
	// generated JavaScript and CSS assets.
	os.MkdirAll(filepath.Join(dir, "images"), 0700)
	os.WriteFile(filepath.Join(dir, "images", "hero-abcdefgh.webp"), []byte("hero image"), 0600)
	var compressed bytes.Buffer
	w := gzip.NewWriter(&compressed)
	w.Write(source)
	w.Close()
	os.WriteFile(filepath.Join(dir, "index-abcdefgh.js.gz"), compressed.Bytes(), 0600)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-store"); c.Next() })
	r.GET("/assets/*filepath", Handler(dir))
	r.HEAD("/assets/*filepath", Handler(dir))
	get := func(method, path, encoding, modified string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Accept-Encoding", encoding)
		if modified != "" {
			req.Header.Set("If-Modified-Since", modified)
		}
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	out := get("GET", "/assets/index-abcdefgh.js", "gzip", "")
	if out.Code != 200 || out.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(out.Header().Get("Cache-Control"), "immutable") || out.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("headers: %d %v", out.Code, out.Header())
	}
	z, err := gzip.NewReader(out.Body)
	if err != nil {
		t.Fatal(err)
	}
	decoded, _ := io.ReadAll(z)
	z.Close()
	if !bytes.Equal(decoded, source) {
		t.Fatal("payload changed")
	}
	if !strings.Contains(out.Header().Get("Content-Type"), "javascript") {
		t.Fatal("incorrect MIME")
	}
	for _, encoding := range []string{"", "gzip;q=0", "br, gzip;q=0", "gzip;q=0, *;q=1"} {
		plain := get("GET", "/assets/index-abcdefgh.js", encoding, "")
		if plain.Header().Get("Content-Encoding") != "" || !bytes.Equal(plain.Body.Bytes(), source) {
			t.Fatalf("disabled encoding: %s", encoding)
		}
	}
	head := get("HEAD", "/assets/index-abcdefgh.js", "gzip", "")
	if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Encoding") != "gzip" {
		t.Fatal("HEAD failed")
	}
	picture := get("GET", "/assets/product.webp", "gzip", "")
	if picture.Header().Get("Cache-Control") != "public, max-age=0, must-revalidate" {
		t.Fatal("image caching")
	}
	versionedPicture := get("GET", "/assets/images/hero-abcdefgh.webp", "", "")
	if !strings.Contains(versionedPicture.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("versioned image caching: %v", versionedPicture.Header())
	}
	if next := get("GET", "/assets/product.webp", "", picture.Header().Get("Last-Modified")); next.Code != 304 || next.Body.Len() != 0 {
		t.Fatal("image revalidation failed")
	}
	for _, path := range []string{"/assets/missing-abcdefgh.js", "/assets/../secret", "/assets/"} {
		missing := get("GET", path, "gzip", "")
		if missing.Code != 404 || strings.Contains(missing.Header().Get("Cache-Control"), "immutable") {
			t.Fatalf("invalid cached: %s %d", path, missing.Code)
		}
	}
}

func TestCompressionNegotiation(t *testing.T) {
	for _, tc := range []struct{ header, want string }{
		{"br, gzip", "br,gzip"}, {"gzip;q=1, br;q=0.2", "gzip,br"}, {"gzip;q=0, *;q=0.5", "br"}, {"br;q=invalid, gzip;q=0", ""}, {"", ""},
	} {
		var names []string
		for _, c := range acceptedCompressions(tc.header) {
			names = append(names, c.name)
		}
		if strings.Join(names, ",") != tc.want {
			t.Fatalf("%q: %v", tc.header, names)
		}
	}
}

func TestImageReplacementInvalidatesETag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "product.webp")
	os.WriteFile(path, []byte("old image"), 0600)
	r := gin.New()
	r.GET("/assets/*filepath", Handler(dir))
	request := func(etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/assets/product.webp", nil)
		req.Header.Set("If-None-Match", etag)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, req)
		return out
	}
	original := request("")
	etag := original.Header().Get("ETag")
	if etag == "" {
		t.Fatal("missing ETag")
	}
	if request(etag).Code != 304 {
		t.Fatal("unchanged image downloaded again")
	}
	os.WriteFile(path, []byte("new image"), 0600)
	changed := request(etag)
	if changed.Code != 200 || changed.Body.String() != "new image" || changed.Header().Get("ETag") == etag {
		t.Fatal("replacement image not refreshed")
	}
}
