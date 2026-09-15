package links

import (
	"regexp"
	"time"
)

type Link struct {
	MetaPixelID         *int64     `json:"meta_pixel_id"`
	MetaConnectionID    *int64     `json:"meta_connection_id"`
	AttributionMode     string     `json:"attribution_mode"`
	LegacyCampaignParam bool       `json:"legacy_campaign_param"`
	LandingDelay        int        `json:"landing_delay"`
	Mode                string     `json:"mode"`
	LandingBrand        string     `json:"landing_brand"`
	LandingTitle        string     `json:"landing_title"`
	LandingDescription  string     `json:"landing_description"`
	LandingDetails      string     `json:"landing_details"`
	ExpiresAt           *time.Time `json:"expires_at"`
	ID                  int64      `json:"id"`
	Code                string     `json:"code"`
	Name                string     `json:"name"`
	TargetURL           string     `json:"target_url"`
	Enabled             bool       `json:"enabled"`
	CampaignID          string     `json:"campaign_id"`
	AdsetID             string     `json:"adset_id"`
	AdID                string     `json:"ad_id"`
	Channel             string     `json:"channel"`
	CreatedAt           time.Time  `json:"created_at"`
}

const Columns = "id,code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,created_at,expires_at,mode,landing_brand,landing_title,landing_description,landing_details,landing_delay,meta_connection_id,attribution_mode,legacy_campaign_param,meta_pixel_id"

var phonePattern = regexp.MustCompile(`^/[1-9][0-9]{6,14}$`)

var codePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,40}$`)
