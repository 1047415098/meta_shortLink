package meta

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceInspectionRejectsCredentialsWithoutEchoing(t *testing.T) {
	for _, key := range []string{"token", "ACCESS_TOKEN", "authorization", "Api_Key", "password", "client_secret"} {
		out := InspectSource("https://example.com/landing?campaign_id=1&adset_id=2&ad_id=3&site_source_name=ig&" + key + "=sensitive-fixture")
		raw, _ := json.Marshal(out)
		if out.Valid || len(out.Issues) == 0 || strings.Contains(string(raw), "sensitive-fixture") {
			t.Fatalf("credential was accepted or leaked for %s", key)
		}
	}
	out := InspectSource("https://example.com/landing?utm_source=facebook&campaign_id=1&adset_id=2&ad_id=3")
	if !out.Valid || out.Source != "facebook" || out.AdID != "3" {
		t.Fatal("canonical source not recognized", out)
	}
	out = InspectSource("campaign_id=1&adset_id=2&ad_id=3&utm_source=instagram&site_source_name=%7B%7Bsite_source_name%7D%7D")
	if out.Valid || out.Source != "instagram" || len(out.Issues) == 0 {
		t.Fatal("unexpanded source macro should warn and use fallback", out)
	}
}
