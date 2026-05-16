package player

import (
	"testing"
	"time"
)

func TestDefaultAggregateSeason(t *testing.T) {
	orig := time.Now
	defer func() { time.Now = orig }()

	time.Now = func() time.Time {
		return time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	}
	if got := DefaultAggregateSeason("nfl"); got != 2026 {
		t.Fatalf("nfl May: got %d want 2026", got)
	}

	time.Now = func() time.Time {
		return time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	}
	if got := DefaultAggregateSeason("nfl"); got != 2025 {
		t.Fatalf("nfl Feb: got %d want 2025", got)
	}
	if got := DefaultAggregateSeason("nba"); got != 2026 {
		t.Fatalf("nba Feb: got %d want 2026", got)
	}
}
