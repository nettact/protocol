package config

import (
	"testing"
	"time"
)

func TestProbeScheduleContract(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		params   ProbeParams
		interval time.Duration
		cycle    time.Duration
	}{
		{"icmp defaults", "icmp", ProbeParams{}, 10 * time.Second, 5800 * time.Millisecond},
		{"gateway shape", "gateway", ProbeParams{PacketCount: 3, TimeoutMs: 2_000}, 10 * time.Second, 6400 * time.Millisecond},
		{"dns defaults", "dns", ProbeParams{}, 30 * time.Second, 3 * time.Second},
		{"http defaults", "http", ProbeParams{}, 30 * time.Second, 10 * time.Second},
		{"tcp defaults", "tcp", ProbeParams{}, 30 * time.Second, 5 * time.Second},
		{"nat defaults", "nat", ProbeParams{}, 30 * time.Minute, 25 * time.Second},
		{"explicit", "http", ProbeParams{IntervalSeconds: 7, TimeoutMs: 1_250}, 7 * time.Second, 1250 * time.Millisecond},
		{"global cycle", "icmp", ProbeParams{PacketCount: 3, TimeoutMs: 500, GlobalTimeoutMs: 9_000}, 10 * time.Second, 9 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EffectiveInterval(tt.kind, tt.params); got != tt.interval {
				t.Fatalf("EffectiveInterval = %v, want %v", got, tt.interval)
			}
			if got := CycleDeadline(tt.kind, tt.params); got != tt.cycle {
				t.Fatalf("CycleDeadline = %v, want %v", got, tt.cycle)
			}
		})
	}

	staleTests := []struct {
		name     string
		interval time.Duration
		cycle    time.Duration
		upload   time.Duration
		want     time.Duration
	}{
		// upload ≤ 0 falls back to DefaultUploadInterval (5s → +2×5s = +10s).
		{"interval-dominant, upload fallback", 10 * time.Second, 5800 * time.Millisecond, 0, 40 * time.Second},
		{"cycle-dominant, upload fallback", 10 * time.Second, 25 * time.Second, 0, 70 * time.Second},
		// An explicit upload interval replaces the default in the +2×upload term.
		{"explicit upload", 10 * time.Second, 5800 * time.Millisecond, 8 * time.Second, 46 * time.Second},
		{"negative upload uses default", 10 * time.Second, 5800 * time.Millisecond, -1, 40 * time.Second},
		// The three configured tiers at their default cycle, default 5s upload:
		// icmp 10s  -> max(30s, 21.6s)=30s  + 10s = 40s
		// http 30s  -> max(90s, 50s)=90s    + 10s = 100s
		// nat 30min -> max(90m, 30m50s)=90m + 10s = 90m10s
		{"icmp tier 10s", DefaultICMPInterval, 5800 * time.Millisecond, DefaultUploadInterval, 40 * time.Second},
		{"regular tier 30s", DefaultHTTPInterval, DefaultHTTPTimeout, DefaultUploadInterval, 100 * time.Second},
		{"nat tier 30min", DefaultNATInterval, DefaultNATCycleDeadline, DefaultUploadInterval, 90*time.Minute + 10*time.Second},
	}
	for _, tt := range staleTests {
		if got := StaleAfter(tt.interval, tt.cycle, tt.upload); got != tt.want {
			t.Fatalf("StaleAfter %s = %v, want %v", tt.name, got, tt.want)
		}
	}
}
