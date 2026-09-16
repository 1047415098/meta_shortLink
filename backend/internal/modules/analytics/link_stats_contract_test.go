package analytics

import (
	"strings"
	"testing"
)

func TestLinkStatsRequireExplicitAdID(t *testing.T) {
	// Canonical Meta traffic must identify an ad through ad_id; utm_content is
	// retained only as descriptive UTM data and cannot satisfy attribution.
	if strings.Contains(linkStatsResolvedAdID, "utm_content") {
		t.Fatal("link statistics still use the removed utm_content fallback")
	}
	if !strings.Contains(linkStatsResolvedAdID, "e.ad_id") {
		t.Fatal("link statistics no longer resolve the explicit ad_id")
	}
}
