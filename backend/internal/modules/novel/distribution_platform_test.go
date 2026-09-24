package novel

import (
	"strings"
	"testing"
)

func int64Pointer(value int64) *int64 { return &value }

func TestDistributionPlatformRequiresExactlyOnePixel(t *testing.T) {
	metaID, connectionID, tiktokID := int64Pointer(11), int64Pointer(12), int64Pointer(13)
	tests := []struct {
		name      string
		input     DistributionInput
		wantError string
	}{
		{"valid Meta", DistributionInput{Name: "Meta", NovelID: 1, AdPlatform: "meta", MetaConnectionID: connectionID, MetaPixelID: metaID, AttributionMode: "dynamic", TimeSpentThreshold: 10}, ""},
		{"valid TikTok", DistributionInput{Name: "TikTok", NovelID: 1, AdPlatform: "tiktok", TikTokPixelID: tiktokID, AttributionMode: "dynamic", TimeSpentThreshold: 10}, ""},
		{"Meta requires Pixel", DistributionInput{Name: "Meta", NovelID: 1, AdPlatform: "meta", AttributionMode: "dynamic", TimeSpentThreshold: 10}, "请选择 Meta Pixel"},
		{"TikTok requires Pixel", DistributionInput{Name: "TikTok", NovelID: 1, AdPlatform: "tiktok", AttributionMode: "dynamic", TimeSpentThreshold: 10}, "请选择 TikTok Pixel"},
		{"Meta rejects TikTok", DistributionInput{Name: "Meta", NovelID: 1, AdPlatform: "meta", MetaConnectionID: connectionID, MetaPixelID: metaID, TikTokPixelID: tiktokID, AttributionMode: "dynamic", TimeSpentThreshold: 10}, "只能绑定一个广告平台"},
		{"TikTok rejects Meta", DistributionInput{Name: "TikTok", NovelID: 1, AdPlatform: "tiktok", MetaConnectionID: connectionID, MetaPixelID: metaID, TikTokPixelID: tiktokID, AttributionMode: "dynamic", TimeSpentThreshold: 10}, "只能绑定一个广告平台"},
		{"unknown platform", DistributionInput{Name: "Other", NovelID: 1, AdPlatform: "other", AttributionMode: "dynamic", TimeSpentThreshold: 10}, "广告平台无效"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateDistributionInput(testCase.input, false)
			if testCase.wantError == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if testCase.wantError != "" && (err == nil || !strings.Contains(err.Error(), testCase.wantError)) {
				t.Fatalf("error=%v, want %q", err, testCase.wantError)
			}
		})
	}
}

func TestTikTokTemplateUsesOfficialDynamicMacros(t *testing.T) {
	want := "https://example.com/novel/wife-a?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__"
	if got := TikTokTemplate("https://example.com/", "wife-a"); got != want {
		t.Fatalf("template=%q", got)
	}
}
