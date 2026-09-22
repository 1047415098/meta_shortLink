package novel

import (
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
