package config

import (
	"testing"
	"time"
)

func TestBudgetWindow(t *testing.T) {
	arrival := time.Date(2026, 7, 26, 22, 14, 56, 0, time.UTC)
	for _, tt := range []struct {
		name       string
		budgetMs   int
		now        time.Time
		wantOK     bool
		wantWindow time.Duration // remaining window at now, checked only when wantOK
	}{
		// Every instant here is the receiver's own: no sender timestamp reaches this
		// function, which is exactly why clock skew cannot shorten the window.
		{"full window at arrival", 10_000, arrival, true, 10 * time.Second},
		{"partially spent before handling", 10_000, arrival.Add(4 * time.Second), true, 6 * time.Second},

		// Spent windows must not start work: a non-positive budget, and a positive
		// budget whose window elapsed between arrival and handling.
		{"zero budget", 0, arrival, false, 0},
		{"negative budget", -1, arrival, false, 0},
		{"elapsed exactly", 10_000, arrival.Add(10 * time.Second), false, 0},
		{"elapsed by handling time", 1_000, arrival.Add(2 * time.Second), false, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			deadline, ok := BudgetWindow(tt.budgetMs, arrival, tt.now)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (deadline %v)", ok, tt.wantOK, deadline)
			}
			if !ok {
				if !deadline.IsZero() {
					t.Errorf("deadline = %v on a spent window, want zero", deadline)
				}
				return
			}
			if got := deadline.Sub(tt.now); got != tt.wantWindow {
				t.Errorf("remaining window = %v, want %v", got, tt.wantWindow)
			}
			// The deadline is anchored at arrival, never at now — handling late must
			// shorten the window rather than restart it.
			if want := arrival.Add(time.Duration(tt.budgetMs) * time.Millisecond); !deadline.Equal(want) {
				t.Errorf("deadline = %v, want %v (anchored at arrival)", deadline, want)
			}
		})
	}
}
