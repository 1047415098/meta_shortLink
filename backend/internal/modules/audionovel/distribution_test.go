package audionovel

import "testing"

func distributionID(value int64) *int64 { return &value }

func TestAudioDistributionInputUsesDynamicExclusivePlatformBinding(t *testing.T) {
	input := DistributionInput{
		Name:               "Buyer A",
		Code:               "audio-a",
		AudioNovelID:       12,
		Enabled:            true,
		AdPlatform:         "meta",
		MetaConnectionID:   distributionID(3),
		MetaPixelID:        distributionID(4),
		AttributionMode:    "bound",
		Channel:            "forged",
		CampaignID:         "forged-campaign",
		AdsetID:            "forged-adset",
		AdID:               "forged-ad",
		TimeSpentThreshold: 1,
	}
	NormalizeDistributionInput(&input)
	if input.AttributionMode != "dynamic" || input.Channel != "" || input.CampaignID != "" || input.AdsetID != "" || input.AdID != "" {
		t.Fatalf("hidden attribution fields were not cleared: %+v", input)
	}
	if err := ValidateDistributionInput(input, true); err != nil {
		t.Fatalf("valid Meta audio link rejected: %v", err)
	}

	input.AdPlatform = "tiktok"
	input.MetaConnectionID = nil
	input.MetaPixelID = nil
	input.TikTokPixelID = distributionID(5)
	NormalizeDistributionInput(&input)
	if err := ValidateDistributionInput(input, true); err != nil {
		t.Fatalf("valid TikTok audio link rejected: %v", err)
	}
}

func TestAudioDistributionInputRejectsInvalidBindingsAndThresholds(t *testing.T) {
	valid := DistributionInput{
		Name:               "Buyer A",
		Code:               "audio-a",
		AudioNovelID:       12,
		Enabled:            true,
		AdPlatform:         "meta",
		MetaConnectionID:   distributionID(3),
		MetaPixelID:        distributionID(4),
		AttributionMode:    "dynamic",
		TimeSpentThreshold: 10,
	}
	cases := []DistributionInput{}
	missingPixel := valid
	missingPixel.MetaPixelID = nil
	cases = append(cases, missingPixel)
	bothPixels := valid
	bothPixels.TikTokPixelID = distributionID(5)
	cases = append(cases, bothPixels)
	missingContent := valid
	missingContent.AudioNovelID = 0
	cases = append(cases, missingContent)
	negativeThreshold := valid
	negativeThreshold.TimeSpentThreshold = -1
	cases = append(cases, negativeThreshold)
	highThreshold := valid
	highThreshold.TimeSpentThreshold = 3601
	cases = append(cases, highThreshold)

	for index, input := range cases {
		if err := ValidateDistributionInput(input, true); err == nil {
			t.Fatalf("invalid case %d was accepted: %+v", index, input)
		}
	}
}
