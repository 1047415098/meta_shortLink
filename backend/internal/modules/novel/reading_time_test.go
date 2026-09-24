package novel

import (
	"strings"
	"testing"
	"time"
)

func TestVisibleSecondsAreCappedByObservedSessionAndTwoHours(t *testing.T) {
	started := time.Date(2026, 9, 21, 10, 0, 0, 0, time.UTC)
	if got := capVisibleSeconds(9000, started, started.Add(40*time.Second)); got != 45 {
		t.Fatalf("forged duration cap = %d, want 45", got)
	}
	if got := capVisibleSeconds(8000, started, started.Add(3*time.Hour)); got != 7200 {
		t.Fatalf("two-hour cap = %d, want 7200", got)
	}
	if got := capVisibleSeconds(12, started, started.Add(40*time.Second)); got != 12 {
		t.Fatalf("valid duration = %d, want 12", got)
	}
}

func TestNormalizeTikTokTTPIsStrictAndIgnoresMacros(t *testing.T) {
	tests := []struct {
		name, input, want string
		wantError         bool
	}{
		{"valid", " cookie-123 ", "cookie-123", false},
		{"valid double underscore", "cookie__segment", "cookie__segment", false},
		{"blank", " ", "", false},
		{"macro", "__TTP__", "", false},
		{"curly macro", "{{ttp}}", "", false},
		{"control", "bad\nvalue", "", true},
		{"overlong", strings.Repeat("x", 513), "", true},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := normalizeTikTokTTP(testCase.input)
			if got != testCase.want || (err != nil) != testCase.wantError {
				t.Fatalf("value=%q err=%v", got, err)
			}
		})
	}
}
