package links

import (
	"regexp"
	"time"
)

const (
	// StartupTail defaults the final visual loading segment for every newly created novel campaign link.
	DefaultStartupTailSeconds = 5
	MinStartupTailSeconds     = 1
	MaxStartupTailSeconds     = 60
)

type Link struct {
	MetaPixelID      *int64 `json:"meta_pixel_id"`
	MetaConnectionID *int64 `json:"meta_connection_id"`
	TikTokPixelID    *int64 `json:"tiktok_pixel_id,omitempty"`
	// AdPlatform freezes which advertising system owns every visit to this link.
	AdPlatform      string `json:"ad_platform"`
	AttributionMode string `json:"attribution_mode"`
	LandingDelay    int    `json:"landing_delay"`
	// TimeSpentThreshold is shared by both public surfaces for this code; zero disables delivery.
	TimeSpentThreshold int `json:"time_spent_threshold"`
	// StartupTailSeconds only drives the free-novel H5 loading animation after its fixed first 3 seconds.
	StartupTailSeconds int `json:"startup_tail_seconds"`
	// ProductType keeps new project links isolated while legacy codes remain cross-surface compatible.
	ProductType        string    `json:"product_type"`
	NovelID            *int64    `json:"novel_id,omitempty"`
	EntryChapterID     *int64    `json:"entry_chapter_id,omitempty"`
	AudioNovelID       *int64    `json:"audio_novel_id,omitempty"`
	Mode               string    `json:"mode"`
	LandingBrand       string    `json:"landing_brand"`
	LandingTitle       string    `json:"landing_title"`
	LandingDescription string    `json:"landing_description"`
	LandingDetails     string    `json:"landing_details"`
	ID                 int64     `json:"id"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	TargetURL          string    `json:"target_url"`
	Enabled            bool      `json:"enabled"`
	CampaignID         string    `json:"campaign_id"`
	AdsetID            string    `json:"adset_id"`
	AdID               string    `json:"ad_id"`
	Channel            string    `json:"channel"`
	CreatedAt          time.Time `json:"created_at"`
}

// Columns mirrors the canonical link contract; legacy attribution flags are intentionally absent.
const Columns = "id,code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,created_at,mode,landing_brand,landing_title,landing_description,landing_details,landing_delay,meta_connection_id,attribution_mode,meta_pixel_id,time_spent_threshold,startup_tail_seconds,product_type,novel_id,ad_platform,tiktok_pixel_id,audio_novel_id,entry_chapter_id"

var phonePattern = regexp.MustCompile(`^/[1-9][0-9]{6,14}$`)

var codePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,40}$`)

// ValidCode is shared by every frontend-specific link creator.
func ValidCode(code string) bool {
	return codePattern.MatchString(code) && code != "api" && code != "assets" && code != "healthz" && code != "admin" && code != "admin-assets" && code != "landing-assets" && code != "novel" && code != "audio-novel"
}
