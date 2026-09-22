package config

import "testing"

func TestNovelPathsUseIndependentDefaultsAndOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("APP_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ADMIN_PASSWORD", "test-password")
	t.Setenv("NOVEL_DIR", "")
	t.Setenv("NOVEL_UPLOAD_DIR", "")

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
	if c.NovelDir != "../novel-h5/dist" || c.NovelUploadDir != "../data/novel-uploads" {
		t.Fatalf("novel defaults = %q, %q", c.NovelDir, c.NovelUploadDir)
	}

	t.Setenv("NOVEL_DIR", "/tmp/novel-ui")
	t.Setenv("NOVEL_UPLOAD_DIR", "/tmp/novel-covers")
	c, err = LoadConfig()
	if err != nil {
		t.Fatalf("load overrides: %v", err)
	}
	if c.NovelDir != "/tmp/novel-ui" || c.NovelUploadDir != "/tmp/novel-covers" {
		t.Fatalf("novel overrides = %q, %q", c.NovelDir, c.NovelUploadDir)
	}
}

func TestAudioNovelAudioDirUsesIndependentDefaultAndOverride(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test")
	t.Setenv("APP_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ADMIN_PASSWORD", "test-password")
	t.Setenv("AUDIO_NOVEL_AUDIO_DIR", "")

	c, err := LoadConfig()
	if err != nil {
		t.Fatalf("load default: %v", err)
	}
	if c.AudioNovelAudioDir != "../data/audio-novel-audio" {
		t.Fatalf("audio directory default = %q", c.AudioNovelAudioDir)
	}

	t.Setenv("AUDIO_NOVEL_AUDIO_DIR", "/tmp/audio-novel-audio")
	c, err = LoadConfig()
	if err != nil {
		t.Fatalf("load override: %v", err)
	}
	if c.AudioNovelAudioDir != "/tmp/audio-novel-audio" {
		t.Fatalf("audio directory override = %q", c.AudioNovelAudioDir)
	}
}
