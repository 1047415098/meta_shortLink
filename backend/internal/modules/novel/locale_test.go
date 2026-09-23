package novel

import (
	"reflect"
	"testing"
)

func TestCountryLocaleMapsOnlySupportedMarkets(t *testing.T) {
	want := map[string]string{"ID": "id", "JP": "ja", "KR": "ko", "MY": "ms", "BN": "ms", "BR": "pt", "PT": "pt", "AO": "pt", "MZ": "pt", "PH": "fil", "TH": "th", "VN": "vi", "US": "en", "unknown": "en"}
	for country, locale := range want {
		if got := CountryLocale(country); got != locale {
			t.Fatalf("country %s = %s, want %s", country, got, locale)
		}
	}
}

func TestResolveLocaleHonorsExplicitCookieIPAndAvailability(t *testing.T) {
	available := []string{"en", "ja", "th"}
	for _, test := range []struct {
		explicit, cookie, country, want string
	}{
		{"th", "ja", "JP", "th"},
		{"", "ja", "TH", "ja"},
		{"", "", "TH", "th"},
		{"ko", "ko", "KR", "en"},
		{"xx", "xx", "JP", "ja"},
		{"", "", "US", "en"},
	} {
		if got := ResolveLocale(test.explicit, test.cookie, test.country, available); got != test.want {
			t.Fatalf("resolve %#v = %s, want %s", test, got, test.want)
		}
	}
	if got := PublicLocale("KO"); got != "en" {
		t.Fatalf("uppercase locale = %s, want en", got)
	}
	if got := SupportedLocaleCodes(); !reflect.DeepEqual(got, []string{"en", "id", "ja", "ko", "ms", "pt", "fil", "th", "vi"}) {
		t.Fatalf("supported locales = %v", got)
	}
}
