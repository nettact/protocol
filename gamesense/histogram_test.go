package gamesense

import (
	"math"
	"testing"
)

func TestHistEdgesAreFrozenAndOrdered(t *testing.T) {
	if len(HistEdgesLog24V1) != HistBins+1 {
		t.Fatalf("edges = %d, want %d", len(HistEdgesLog24V1), HistBins+1)
	}
	for i := 1; i < len(HistEdgesLog24V1); i++ {
		if HistEdgesLog24V1[i] <= HistEdgesLog24V1[i-1] {
			t.Errorf("edge %d (%v) does not exceed edge %d (%v)", i, HistEdgesLog24V1[i], i-1, HistEdgesLog24V1[i-1])
		}
	}
	// The literals are the stored contract, so check them against the geometry
	// they were drawn from rather than trusting that a transcription is right.
	// A drift here means recorded history would be reinterpreted.
	for i, edge := range HistEdgesLog24V1 {
		want := 0.5 * math.Pow(10, float64(i)/8)
		if math.Abs(edge-want)/want > 1e-12 {
			t.Errorf("edge %d = %v, want ~%v", i, edge, want)
		}
	}
	if HistEdgesLog24V1[0] != 0.5 || HistEdgesLog24V1[HistBins] != 500 {
		t.Errorf("range = [%v, %v], want [0.5, 500]", HistEdgesLog24V1[0], HistEdgesLog24V1[HistBins])
	}
}

func TestHistMidpointsSitInsideTheirBins(t *testing.T) {
	for i, mid := range HistMidpointsLog24V1 {
		lo, hi := HistEdgesLog24V1[i], HistEdgesLog24V1[i+1]
		if mid <= lo || mid >= hi {
			t.Errorf("midpoint %d = %v, outside [%v, %v)", i, mid, lo, hi)
		}
	}
}

func TestHistBucketPlacesFrameTimesWhereTheyBelong(t *testing.T) {
	cases := []struct {
		ms      float64
		bin     int
		clipped bool
		about   string
	}{
		{16.67, 12, false, "60 FPS"},
		{6.94, 9, false, "144 FPS"},
		{4.17, 7, false, "240 FPS"},
		{8.33, 9, false, "120 FPS"},
		{33.3, 14, false, "30 FPS"},
		{200, 20, false, "a bad stutter"},
		// Boundaries belong to the bin they open, never the one they close.
		{0.5, 0, false, "the first edge"},
		{5.0, 8, false, "an interior edge"},
		{50.0, 16, false, "another interior edge"},
		{499.9, 23, false, "just inside the top"},
		// Outside the range the value is still counted, but flagged.
		{0.4, 0, true, "faster than the layout can express"},
		{500, 23, true, "at the top edge"},
		{5000, 23, true, "a five-second hitch"},
	}
	for _, c := range cases {
		bin, clipped := HistBucket(c.ms)
		if bin != c.bin || clipped != c.clipped {
			t.Errorf("HistBucket(%v) [%s] = (%d, %v), want (%d, %v)", c.ms, c.about, bin, clipped, c.bin, c.clipped)
		}
	}
}

func TestHistBucketCoversEveryBinExactlyOnce(t *testing.T) {
	// Every bin must be reachable, or the layout has a gap nothing can land in.
	seen := make([]bool, HistBins)
	for i := range seen {
		mid := HistMidpointsLog24V1[i]
		bin, clipped := HistBucket(mid)
		if clipped {
			t.Errorf("midpoint of bin %d reported as clipped", i)
		}
		if seen[bin] {
			t.Errorf("bin %d claimed twice", bin)
		}
		seen[bin] = true
		if bin != i {
			t.Errorf("midpoint of bin %d landed in bin %d", i, bin)
		}
	}
}

func TestHistAddRefusesAMismatchedLayout(t *testing.T) {
	dst := make([]uint32, HistBins)
	dst[3] = 5
	if HistAdd(dst, make([]uint32, HistBins-1)) {
		t.Error("merged a histogram of the wrong length")
	}
	if dst[3] != 5 {
		t.Error("a refused merge must leave the destination untouched")
	}
	src := make([]uint32, HistBins)
	src[3] = 7
	src[9] = 2
	if !HistAdd(dst, src) {
		t.Fatal("a matching merge must succeed")
	}
	if dst[3] != 12 || dst[9] != 2 {
		t.Errorf("merged = %v", dst)
	}
	if HistTotal(dst) != 14 {
		t.Errorf("total = %d, want 14", HistTotal(dst))
	}
}

func TestHistLowFPSDescribesTheSlowTail(t *testing.T) {
	// A run that mostly holds 60 FPS with a hundred frames far slower: the mean
	// is comfortable, the 1% low is not, and that gap is the whole reason players
	// ask for the figure.
	counts := make([]uint32, HistBins)
	counts[12] = 9900 // centered ~18 ms
	counts[19] = 100  // centered ~137 ms

	mean, ok := HistMeanFPS(counts)
	if !ok {
		t.Fatal("mean unavailable")
	}
	if mean < 50 || mean > 62 {
		t.Errorf("mean FPS = %v, want roughly 55-60", mean)
	}
	low, ok := HistLowFPS(counts, 0.01)
	if !ok {
		t.Fatal("1%% low unavailable")
	}
	// The slowest 1% is exactly the 100 slow frames, whose bin centers on ~137 ms.
	if low < 6.5 || low > 8 {
		t.Errorf("1%% low = %v, want roughly 7.3", low)
	}
	if low >= mean {
		t.Error("the slow tail must not be faster than the mean")
	}
}

func TestHistLowFPSDeclinesWhenTheSampleIsTooSmall(t *testing.T) {
	// One slow frame out of two hundred is one slow frame, not a 1% low. A
	// figure computed from it would look like a statistic and behave like noise.
	counts := make([]uint32, HistBins)
	counts[12] = 199
	counts[20] = 1
	if _, ok := HistLowFPS(counts, 0.01); ok {
		t.Error("published a 1%% low from two frames' worth of tail")
	}
	// Ten times the frames, the same shape: now the fraction covers enough.
	counts[12] = 1990
	counts[20] = 10
	if _, ok := HistLowFPS(counts, 0.01); !ok {
		t.Error("declined a 1%% low that the sample supports")
	}
	// Right at the boundary: 1% of 999 frames is ten of them once rounded up, so
	// this clears the minimum. Truncating to nine would refuse a run that
	// qualifies, and every run one frame short of a round thousand is this one.
	counts[12] = 989
	counts[20] = 10
	if HistTotal(counts) != 999 {
		t.Fatalf("total = %d, want 999", HistTotal(counts))
	}
	if _, ok := HistLowFPS(counts, 0.01); !ok {
		t.Error("declined a 1%% low for 999 frames, where the tail is ten frames")
	}
	if _, ok := HistLowFPS(make([]uint32, HistBins), 0.01); ok {
		t.Error("published a low figure for an empty histogram")
	}
	if _, ok := HistLowFPS(counts, 0); ok {
		t.Error("accepted a zero fraction")
	}
	if _, ok := HistLowFPS(make([]uint32, 5), 0.01); ok {
		t.Error("accepted a histogram of the wrong length")
	}
}

func TestHistMeanFPSReportsNothingRatherThanZero(t *testing.T) {
	if _, ok := HistMeanFPS(make([]uint32, HistBins)); ok {
		t.Error("an empty histogram must not claim a frame rate")
	}
}

func TestRoundMsRoundsHalfAwayFromZero(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{6.9444, 6.944},
		{6.9445, 6.945},
		{0.0005, 0.001},
		{16.6666, 16.667},
		{0, 0},
	}
	for _, c := range cases {
		if got := RoundMs(c.in); got != c.want {
			t.Errorf("RoundMs(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
