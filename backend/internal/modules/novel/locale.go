package novel

import "strings"

const (
	LanguageCookieName = "novel_lang"
	LanguageHeaderName = "X-Novel-Language"
)

func SupportedLocaleCodes() []string {
	locales := []string{"en"}
	for _, option := range TargetLocales {
		locales = append(locales, option.Code)
	}
	return locales
}

func PublicLocale(locale string) string {
	if locale == "en" {
		return locale
	}
	if _, ok := TranslationType(locale); ok {
		return locale
	}
	return "en"
}

func CountryLocale(country string) string {
	country = strings.ToUpper(strings.TrimSpace(country))
	switch country {
	case "ID":
		return "id"
	case "JP":
		return "ja"
	case "KR":
		return "ko"
	case "MY", "BN":
		return "ms"
	case "BR", "PT", "AO", "MZ", "CV", "GW", "ST":
		return "pt"
	// 西班牙及主要西语市场共享 es；若小说未发布西语译本，ResolveLocale 会安全回退英文。
	case "ES", "MX", "AR", "CO", "CL", "PE", "VE", "EC", "GT", "CU", "BO", "DO", "HN", "PY", "SV", "NI", "CR", "PA", "UY", "GQ", "PR":
		return "es"
	case "PH":
		return "fil"
	case "TH":
		return "th"
	case "VN":
		return "vi"
	default:
		return "en"
	}
}

func ResolveLocale(explicit, remembered, country string, available []string) string {
	allowed := map[string]bool{}
	for _, locale := range available {
		allowed[locale] = true
	}
	for _, candidate := range []string{explicit, remembered, CountryLocale(country), "en"} {
		if candidate == "en" || PublicLocale(candidate) == candidate {
			if allowed[candidate] {
				return candidate
			}
		}
	}
	return "en"
}
