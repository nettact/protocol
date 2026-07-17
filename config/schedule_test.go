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
		{"global cycle", "icmp", ProbeParams{Retries: 2, TimeoutMs: 500, GlobalTimeoutMs: 9_000}, 10 * time.Second, 9 * time.Second},
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

	if got := StaleAfter(10*time.Second, 5800*time.Millisecond); got != 30*time.Second {
		t.Fatalf("StaleAfter interval-dominant = %v, want 30s", got)
	}
	if got := StaleAfter(10*time.Second, 25*time.Second); got != 60*time.Second {
		t.Fatalf("StaleAfter cycle-dominant = %v, want 60s", got)
	}
}
