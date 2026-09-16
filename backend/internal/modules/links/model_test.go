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
