package novel

import (
	"bytes"
	"testing"
)

func TestNovelMetadataReplacesGenericTags(t *testing.T) {
	page := []byte(`<html><head><title>Generic</title><meta name="description" content="generic" /></head></html>`)
	got := novelMetadata(page)
	if bytes.Contains(got, []byte("Generic")) || !bytes.Contains(got, []byte("Free Stories")) {
		t.Fatalf("metadata = %s", got)
	}
}
