package links

import (
	"regexp"
	"time"
)

type Link struct {
	MetaPixelID        *int64    `json:"meta_pixel_id"`
	MetaConnectionID   *int64    `json:"meta_connection_id"`
	AttributionMode    string    `json:"attribution_mode"`
	LandingDelay       int       `json:"landing_delay"`
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
const Columns = "id,code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,created_at,mode,landing_brand,landing_title,landing_description,landing_details,landing_delay,meta_connection_id,attribution_mode,meta_pixel_id"

var phonePattern = regexp.MustCompile(`^/[1-9][0-9]{6,14}$`)

var codePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,40}$`)
