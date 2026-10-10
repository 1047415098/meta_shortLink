package analytics

import (
	"net/url"
	"testing"
	"time"
)

func TestSurfaceFilterValidation(t *testing.T) {
	// Shared visit tables cover every independently tracked frontend surface.
	for _, surface := range []string{"", "short_link", "audio_novel", "novel", "cover"} {
		filter, err := parseFilterValues(url.Values{"surface": {surface}}, "Etc/GMT+8")
		if err != nil || filter.Surface != surface {
			t.Fatalf("surface=%q filter=%+v err=%v", surface, filter, err)
		}
	}
	if _, err := parseFilterValues(url.Values{"surface": {"unknown"}}, "Etc/GMT+8"); err == nil {
		t.Fatal("invalid surface was accepted")
	}
}

func TestProjectVisitFilterValidation(t *testing.T) {
	// Project visit tabs and attribution fields share the same validated JSON filter.
	values := url.Values{
		"traffic_scope": {"valid"}, "campaign_id": {"campaign-1"},
		"adgroup_id": {"group-1"}, "creative_id": {"creative-1"},
		"ad_id_v2": {"ad-v2"}, "event_status": {"accepted"},
	}
	filter, err := parseFilterValues(values, "Etc/GMT+8")
	if err != nil {
		t.Fatal(err)
	}
	if filter.TrafficScope != "valid" || filter.CampaignID != "campaign-1" || filter.AdgroupID != "group-1" || filter.CreativeID != "creative-1" || filter.AdIDV2 != "ad-v2" || filter.EventStatus != "accepted" {
		t.Fatalf("unexpected project visit filter: %+v", filter)
	}
	for _, scope := range []string{"all", "valid", "abnormal"} {
		if _, err = parseFilterValues(url.Values{"traffic_scope": {scope}}, "Etc/GMT+8"); err != nil {
			t.Fatalf("scope %q rejected: %v", scope, err)
		}
	}
	if _, err = parseFilterValues(url.Values{"traffic_scope": {"unknown"}}, "Etc/GMT+8"); err == nil {
		t.Fatal("invalid traffic scope was accepted")
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
