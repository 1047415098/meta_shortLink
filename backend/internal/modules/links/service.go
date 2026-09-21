package links

import (
	"net/url"
	"strings"
)

func ValidTarget(s string) bool {
	u, e := url.Parse(s)
	if e != nil || u.Scheme != "https" || u.Host != "wa.me" || u.User != nil || u.Fragment != "" || !phonePattern.MatchString(u.Path) || len(s) > 2048 {
		return false
	}
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return false
	}
	for k, v := range q {
		if k != "text" || len(v) != 1 {
			return false
		}
	}
	return true
}

func ValidLink(l Link) bool {
	if l.MetaConnectionID != nil && *l.MetaConnectionID <= 0 {
		return false
	}
	if l.MetaPixelID != nil && *l.MetaPixelID <= 0 {
		return false
	}
	// Attribution is a required stored decision; empty values can no longer
	// fall through to the historical fixed-binding behavior.
	if l.AttributionMode != "bound" && l.AttributionMode != "dynamic" {
		return false
	}
	if l.LandingDelay < 0 || l.LandingDelay > 300 {
		return false
	}
	// Zero is the explicit off state; enabled timers use a practical bounded range.
	if l.TimeSpentThreshold != 0 && (l.TimeSpentThreshold < 5 || l.TimeSpentThreshold > 3600) {
		return false
	}
	if l.Mode != "" && l.Mode != "redirect" && l.Mode != "landing" {
		return false
	}
	if len(l.LandingBrand) > 120 || len(l.LandingTitle) > 240 || len(l.LandingDescription) > 2400 || len(l.LandingDetails) > 6000 {
		return false
	}
	if l.Mode == "landing" && (strings.TrimSpace(l.LandingBrand) == "" || strings.TrimSpace(l.LandingTitle) == "" || strings.TrimSpace(l.LandingDescription) == "") {
		return false
	}
	return codePattern.MatchString(l.Code) && l.Code != "api" && l.Code != "assets" && l.Code != "healthz" && l.Code != "admin" && l.Code != "admin-assets" && l.Code != "landing-assets" && ValidTarget(l.TargetURL) && len(strings.TrimSpace(l.Name)) > 0 && len(l.Name) <= 120 && len(l.AdID) <= 120 && len(l.CampaignID) <= 120 && len(l.AdsetID) <= 120 && len(l.Channel) <= 60
}

// HasMetaBinding identifies the complete account/Pixel pair required for new
// links while legacy rows without a pair remain available for emergency pause.
func HasMetaBinding(l Link) bool {
	return l.MetaConnectionID != nil && *l.MetaConnectionID > 0 && l.MetaPixelID != nil && *l.MetaPixelID > 0
}
