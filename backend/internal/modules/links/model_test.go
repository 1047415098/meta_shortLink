package links

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLinkJSONOmitsRemovedLegacyCampaignParameter(t *testing.T) {
	// The public link contract exposes only the canonical explicit Meta IDs.
	raw, err := json.Marshal(Link{})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "legacy_campaign_param") {
		t.Fatalf("removed legacy field remains public: %s", raw)
	}
}

func TestValidLinkTimeSpentThreshold(t *testing.T) {
	// Zero disables the event; enabled thresholds use a bounded whole-second value.
	base := Link{Code: "timer", Name: "Timer", TargetURL: "https://wa.me/13365661092", AttributionMode: "dynamic"}
	for _, threshold := range []int{0, 5, 60, 3600} {
		base.TimeSpentThreshold = threshold
		if !ValidLink(base) {
			t.Fatalf("valid TimeSpent threshold rejected: %d", threshold)
		}
	}
	for _, threshold := range []int{-1, 1, 4, 3601} {
		base.TimeSpentThreshold = threshold
		if ValidLink(base) {
			t.Fatalf("invalid TimeSpent threshold accepted: %d", threshold)
		}
	}
}
