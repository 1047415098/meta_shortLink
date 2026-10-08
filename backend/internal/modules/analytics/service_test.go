package analytics

import (
	"net/url"
	"testing"
	"time"
)

func TestSurfaceFilterValidation(t *testing.T) {
	for _, surface := range []string{"", "short_link", "audio_novel", "novel"} {
		filter, err := parseFilterValues(url.Values{"surface": {surface}}, "Etc/GMT+8")
		if err != nil || filter.Surface != surface {
			t.Fatalf("surface=%q filter=%+v err=%v", surface, filter, err)
		}
	}
	if _, err := parseFilterValues(url.Values{"surface": {"unknown"}}, "Etc/GMT+8"); err == nil {
		t.Fatal("invalid surface was accepted")
	}
}

func TestParseFilterDefaultsToCurrentReportDay(t *testing.T) {
	filter, err := parseFilterValues(url.Values{}, "Etc/GMT+8")
	if err != nil {
		t.Fatal(err)
	}
	// Empty report queries must be a same-day report, not an implicit seven-day range.
	if got, want := filter.Start.Format("2006-01-02"), time.Now().In(time.FixedZone("UTC-8", -8*60*60)).Format("2006-01-02"); got != want {
		t.Fatalf("default start = %q, want %q", got, want)
	}
	if got, want := filter.End, filter.Start.AddDate(0, 0, 1); !got.Equal(want) {
		t.Fatalf("default end = %s, want next day %s", got, want)
	}
}
