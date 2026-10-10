package cover

import (
	"errors"
	"strings"
	"time"

	"whatsapp-analytics/internal/modules/links"
)

const ProductType = "cover"

// LinkInput contains only campaign settings; the cover project has no novel content binding.
type LinkInput struct {
	Name               string `json:"name"`
	Code               string `json:"code,omitempty"`
	Enabled            bool   `json:"enabled"`
	MetaConnectionID   *int64 `json:"meta_connection_id"`
	MetaPixelID        *int64 `json:"meta_pixel_id"`
	TikTokPixelID      *int64 `json:"tiktok_pixel_id"`
	AdPlatform         string `json:"ad_platform"`
	AttributionMode    string `json:"attribution_mode"`
	TimeSpentThreshold int    `json:"time_spent_threshold"`
}

type Link struct {
	ID                 int64      `json:"id"`
	Code               string     `json:"code"`
	Name               string     `json:"name"`
	Enabled            bool       `json:"enabled"`
	ProductType        string     `json:"product_type"`
	MetaConnectionID   *int64     `json:"meta_connection_id"`
	MetaPixelID        *int64     `json:"meta_pixel_id"`
	TikTokPixelID      *int64     `json:"tiktok_pixel_id,omitempty"`
	TikTokPixelCode    string     `json:"tiktok_pixel_code,omitempty"`
	TikTokPixelName    string     `json:"tiktok_pixel_name,omitempty"`
	AdPlatform         string     `json:"ad_platform"`
	AttributionMode    string     `json:"attribution_mode"`
	TimeSpentThreshold int        `json:"time_spent_threshold"`
	VisitCount         int64      `json:"visit_count"`
	CreatedAt          time.Time  `json:"created_at"`
	FirstVisitedAt     *time.Time `json:"first_visited_at,omitempty"`
	PublicURL          string     `json:"public_url,omitempty"`
	TikTokTemplateURL  string     `json:"tiktok_template_url,omitempty"`
}

func normalizeInput(input *LinkInput) {
	if input.AdPlatform == "" {
		input.AdPlatform = "meta"
	}
	// Cover links always use dynamic advertising parameters from the incoming URL.
	input.AttributionMode = "dynamic"
}

func validateInput(input LinkInput, requireCode bool) error {
	if strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 120 {
		return errors.New("链接名称不能为空且不能超过 120 个字符")
	}
	if requireCode && input.Code != "" && !links.ValidCode(input.Code) {
		return errors.New("短码只能包含 3–40 位字母、数字、下划线或横线")
	}
	if input.TimeSpentThreshold != 0 && (input.TimeSpentThreshold < 5 || input.TimeSpentThreshold > 3600) {
		return errors.New("停留事件阈值应为 5–3600 秒，或设为 0")
	}
	switch input.AdPlatform {
	case "meta":
		binding := links.Link{MetaConnectionID: input.MetaConnectionID, MetaPixelID: input.MetaPixelID}
		if !links.HasMetaBinding(binding) || input.TikTokPixelID != nil {
			return errors.New("请选择一个可用的 Meta Pixel")
		}
	case "tiktok":
		if input.TikTokPixelID == nil || *input.TikTokPixelID < 1 || input.MetaConnectionID != nil || input.MetaPixelID != nil {
			return errors.New("请选择一个可用的 TikTok Pixel")
		}
	default:
		return errors.New("广告平台无效")
	}
	return nil
}
