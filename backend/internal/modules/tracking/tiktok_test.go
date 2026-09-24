package tracking

import "testing"

func TestResolvedAttributionValueRejectsMacrosControlsAndOversize(t *testing.T) {
	tests := []struct {
		input string
		limit int
		want  string
	}{
		{" campaign-1 ", 512, "campaign-1"},
		{"click__segment", 512, "click__segment"},
		{"__CAMPAIGN_ID__", 512, ""},
		{"{{campaign.id}}", 512, ""},
		{"bad\nvalue", 512, ""},
		{"123456", 5, ""},
		{"", 512, ""},
	}
	for _, testCase := range tests {
		if got := resolvedAttributionValue(testCase.input, testCase.limit); got != testCase.want {
			t.Fatalf("resolvedAttributionValue(%q,%d)=%q want %q", testCase.input, testCase.limit, got, testCase.want)
		}
	}
}

func TestTikTokPlatformSourceIsCanonical(t *testing.T) {
	if got := platformSource("TikTok"); got != "tiktok" {
		t.Fatalf("source=%q", got)
	}
}
