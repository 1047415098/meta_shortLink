package novel

import (
	"testing"
	"time"
)

func stringPointer(value string) *string { return &value }

func TestDistributionStatsFilterAcceptsTikTokAttributionFields(t *testing.T) {
	filter, err := parseDistributionStatsFilter(distributionStatsRequest{
		TZ:          stringPointer("UTC"),
		CampaignID:  "campaign-1",
		AdgroupID:   "group-1",
		CreativeID:  "creative-1",
		AdIDV2:      "ad-1",
		EventStatus: "accepted",
		Page:        2,
	}, "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if filter.CampaignID != "campaign-1" || filter.AdgroupID != "group-1" || filter.CreativeID != "creative-1" ||
		filter.AdIDV2 != "ad-1" || filter.EventStatus != "accepted" || filter.Page != 2 {
		t.Fatalf("unexpected TikTok stats filter: %+v", filter)
	}
}

func TestDistributionStatsFilterDefaultsToCurrentReportDay(t *testing.T) {
	filter, err := parseDistributionStatsFilter(distributionStatsRequest{Page: 1}, "UTC")
	if err != nil {
		t.Fatal(err)
	}
	// The first visit to a link report is intentionally scoped to today's report day.
	if got, want := filter.Start.Format("2006-01-02"), time.Now().UTC().Format("2006-01-02"); got != want {
		t.Fatalf("default start = %q, want %q", got, want)
	}
	if got, want := filter.End, filter.Start.AddDate(0, 0, 1); !got.Equal(want) {
		t.Fatalf("default end = %s, want next day %s", got, want)
	}
}

func TestDistributionStatsFilterRejectsInvalidTikTokEventStatus(t *testing.T) {
	_, err := parseDistributionStatsFilter(distributionStatsRequest{
		TZ:          stringPointer("UTC"),
		EventStatus: "attributed",
		Page:        1,
	}, "Asia/Shanghai")
	if err == nil {
		t.Fatal("expected invalid TikTok event status to be rejected")
	}
}

func TestMaskTikTokIDNeverReturnsTheCompleteIdentifier(t *testing.T) {
	if got := maskTikTokID("12345678"); got != "********" {
		t.Fatalf("short identifier mask = %q", got)
	}
	if got := maskTikTokID("1234567890abcdef"); got != "1234…cdef" {
		t.Fatalf("long identifier mask = %q", got)
	}
}
