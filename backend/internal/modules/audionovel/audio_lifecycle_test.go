package audionovel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAudioLifecycleRejectsPathsOutsidePublicAudioPrefix(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.mp3")
	if err := os.WriteFile(outside, []byte("keep"), 0600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	for _, unsafe := range []string{outside, "/audio-novel-audio/../outside.mp3", "/novel-uploads/11111111111111111111111111111111.mp3"} {
		if err := removeAudioFile(dir, unsafe); err == nil {
			t.Fatalf("unsafe path accepted: %q", unsafe)
		}
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file was removed: %v", err)
	}
}

func TestAudioLifecycleRemovesOnlyValidatedBasename(t *testing.T) {
	dir := t.TempDir()
	name := "0123456789abcdef0123456789abcdef.mp3"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("ID3test"), 0600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := removeAudioFile(dir, "/audio-novel-audio/"+name); err != nil {
		t.Fatalf("remove valid audio: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("valid audio still exists: %v", err)
	}
}
