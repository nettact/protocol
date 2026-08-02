package gamesense

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// The console computes the same figures this file does, over whatever span a
// reader has selected on a chart, and it has to compute them from histograms for
// the reason HistLowFPS gives: a mean of per-second percentiles is not any
// number. So there is a second implementation of this arithmetic, in TypeScript,
// and the copy is the thing that can drift.
//
// Both read this one fixture. Go going red means Go moved; vitest going red
// means the port moved. There is no way for the two to move together silently,
// which is the only property that makes a duplicated implementation safe.
//
// The fixture lives in web-console because that is where the copy is. The cost
// of that is honest and worth stating: a CI that checks out protocol alone will
// skip this and catch no drift. The workspace is always laid out with the
// modules side by side, which is where this actually runs.
const goldenPath = "../../web-console/src/lib/histogram.golden.json"

type goldenCase struct {
	Name    string   `json:"name"`
	Counts  []uint32 `json:"counts"`
	MeanFPS *float64 `json:"mean_fps"`
	Low1    *float64 `json:"low_1pct_fps"`
	Low01   *float64 `json:"low_0_1pct_fps"`
}

func TestHistogramGoldenMatchesFixture(t *testing.T) {
	raw, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skipf("no web-console beside this module, so there is no port to check against: %v", err)
	}
	var cases []goldenCase
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatalf("golden fixture is not readable: %v", err)
	}
	if len(cases) < 5 {
		t.Fatalf("golden fixture holds %d case(s); it is meant to cover the boundaries", len(cases))
	}

	// Compared exactly rather than within a tolerance, and deliberately. The two
	// implementations do the same operations in the same order over the same
	// IEEE-754 doubles, so they agree bit for bit — and a tolerance would hide
	// precisely the kind of drift this test exists to catch, such as a midpoint
	// derived by a different route.
	check := func(t *testing.T, name string, got float64, ok bool, want *float64) {
		t.Helper()
		if want == nil {
			if ok {
				t.Errorf("%s = %v, want it declined", name, got)
			}
			return
		}
		if !ok {
			t.Errorf("%s was declined, want %v", name, *want)
			return
		}
		if got != *want && !(math.IsNaN(got) && math.IsNaN(*want)) {
			t.Errorf("%s = %v, want %v", name, got, *want)
		}
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			if len(c.Counts) != HistBins {
				t.Fatalf("case holds %d bins, want %d", len(c.Counts), HistBins)
			}
			mean, ok := HistMeanFPS(c.Counts)
			check(t, "HistMeanFPS", mean, ok, c.MeanFPS)
			low1, ok := HistLowFPS(c.Counts, 0.01)
			check(t, "HistLowFPS(1%)", low1, ok, c.Low1)
			low01, ok := HistLowFPS(c.Counts, 0.001)
			check(t, "HistLowFPS(0.1%)", low01, ok, c.Low01)
		})
	}
}
