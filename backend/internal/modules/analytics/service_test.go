package analytics

import (
	"net/url"
	"testing"
)

func TestSurfaceFilterValidation(t *testing.T) {
	for _, surface := range []string{"", "short_link", "audio_novel"} {
		filter, err := parseFilterValues(url.Values{"surface": {surface}}, "Asia/Shanghai")
		if err != nil || filter.Surface != surface {
			t.Fatalf("surface=%q filter=%+v err=%v", surface, filter, err)
		}
	}
	if _, err := parseFilterValues(url.Values{"surface": {"unknown"}}, "Asia/Shanghai"); err == nil {
		t.Fatal("invalid surface was accepted")
	}
}
