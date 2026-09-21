package app

import (
	"strings"
	"testing"
)

func TestConfiguredAdminPassword(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("APP_SECRET", strings.Repeat("s", 32))
	t.Setenv("PUBLIC_BASE_URL", "https://example.com")
	for _, tc := range []struct {
		name, password string
		valid          bool
	}{{"explicit admin", "admin", true}, {"empty", "", false}, {"bcrypt limit", strings.Repeat("a", 73), false}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ADMIN_PASSWORD", tc.password)
			c, e := LoadConfig()
			if (e == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, e)
			}
			if e == nil && c.AdminPassword != tc.password {
				t.Fatal("password changed")
			}
		})
	}
}

func TestAudioNovelDirectoriesUseDedicatedEnvironmentVariables(t *testing.T) {
	// The audio novel app must not fall back to the retired novel configuration names.
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("APP_SECRET", strings.Repeat("s", 32))
	t.Setenv("ADMIN_PASSWORD", "test-password")
	t.Setenv("AUDIO_NOVEL_DIR", "/tmp/audio-novel")
	t.Setenv("AUDIO_NOVEL_UPLOAD_DIR", "/tmp/audio-novel-uploads")
	t.Setenv("NOVEL_DIR", "/tmp/retired-novel")
	t.Setenv("NOVEL_UPLOAD_DIR", "/tmp/retired-novel-uploads")

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if c.AudioNovelDir != "/tmp/audio-novel" {
		t.Fatalf("AudioNovelDir = %q", c.AudioNovelDir)
	}
	if c.AudioNovelUploadDir != "/tmp/audio-novel-uploads" {
		t.Fatalf("AudioNovelUploadDir = %q", c.AudioNovelUploadDir)
	}
}
