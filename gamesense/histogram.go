package gamesense

import "math"

// The frame-time histogram layout.
//
// Per-second summaries cannot answer the question players actually ask. "1% low"
// means the slowest one percent of frames in a whole session, and a mean of
// per-second percentiles is not that number — it is not any number. Histograms
// solve it by being addable: sum a run's seconds bin by bin and the result is
// the distribution of every frame in the run, from which the low figures follow.
//
// Bins are log-spaced because frame times are. The interesting range spans two
// orders of magnitude (a 240 Hz frame is 4 ms, a bad stutter is 200 ms) and a
// fixed-width bin fine enough for the former is absurd for the latter. Eight
// bins per decade puts every bin within about 15% of its own center, which is
// finer than the difference anyone can feel.

// HistLayoutLog24V1 names this layout on the wire and in storage. The name is
// the compatibility contract: the edges below are frozen, and any change to them
// is a new layout name, never an edit. A reader that meets an unfamiliar layout
// must decline to interpret the counts — silently applying its own edges would
// turn a bin count into a wrong frame time with no way to notice.
const HistLayoutLog24V1 = "log24_v1"

// HistBins is the number of bins in HistLayoutLog24V1.
const HistBins = 24

// HistEdgesLog24V1 are the bin boundaries in milliseconds: bin i covers
// [HistEdgesLog24V1[i], HistEdgesLog24V1[i+1]). The range 0.5–500 ms spans 2000
// FPS down to 2 FPS, comfortably past both ends of anything a game does.
//
// These are literals rather than a loop over 0.5*10^(i/8) on purpose. They are
// stored data: buckets recorded today are read back and merged years from now,
// possibly by a different binary on a different platform, and every one of them
// must agree on where bin 12 begins. A formula is a promise that every future
// runtime rounds the same way; a table is not a promise at all.
var HistEdgesLog24V1 = [HistBins + 1]float64{
	0.5,
	0.666760716081662,
	0.8891397050194614,
	1.1856868528308277,
	1.5811388300841898,
	2.108482517142911,
	2.8117066259517456,
	3.7494710466622802,
	5.0,
	6.66760716081662,
	8.891397050194614,
	11.856868528308277,
	15.811388300841898,
	21.08482517142911,
	28.117066259517456,
	37.494710466622802,
	50.0,
	66.6760716081662,
	88.91397050194614,
	118.56868528308277,
	158.11388300841898,
	210.8482517142911,
	281.17066259517456,
	374.94710466622802,
	500.0,
}

// HistMidpointsLog24V1 are the representative frame times of each bin: the value
// a bin's frames are taken to have when computing a statistic from counts alone.
//
// Geometric rather than arithmetic centers, because the bins are geometric. The
// arithmetic middle of [50, 66.68) is 58.3 ms, which is not the value that
// splits the bin evenly in the space the bin was drawn in; the geometric middle,
// 57.7 ms, is. Derived from the frozen edge table by square root — an operation
// IEEE-754 requires to be correctly rounded, so every platform derives the same
// numbers from the same table.
var HistMidpointsLog24V1 = func() [HistBins]float64 {
	var mid [HistBins]float64
	for i := range mid {
		mid[i] = math.Sqrt(HistEdgesLog24V1[i] * HistEdgesLog24V1[i+1])
	}
	return mid
}()

// HistBucket returns the bin a frame interval belongs in, and whether the
// interval fell outside the layout's range and was clamped into an end bin.
//
// A clamped sample is still counted. Dropping it would break the invariant that
// the counts sum to the second's frame total, and a histogram that quietly omits
// the worst frames is worse than one that admits its tail is approximate — hence
// the second return value, which the caller records as QualityHistClipped so a
// reader knows not to trust the end bins of that second.
//
// ms must be finite and positive; the caller filters intervals before binning,
// because a source that reports a non-positive interval has told us something
// about itself rather than about the frame.
func HistBucket(ms float64) (bin int, clipped bool) {
	if ms >= HistEdgesLog24V1[HistBins] {
		return HistBins - 1, true
	}
	if ms < HistEdgesLog24V1[0] {
		return 0, true
	}
	for i := HistBins - 1; i > 0; i-- {
		if ms >= HistEdgesLog24V1[i] {
			return i, false
		}
	}
	return 0, false
}

// HistTotal returns the number of frames a histogram counts.
func HistTotal(counts []uint32) uint64 {
	var total uint64
	for _, c := range counts {
		total += uint64(c)
	}
	return total
}

// HistAdd accumulates src into dst bin by bin. This is the whole reason the
// histogram exists: a run's distribution is the sum of its seconds'.
//
// Mismatched lengths are ignored rather than merged, so a bucket recorded under
// a different layout cannot contaminate a total. Callers compare Layout before
// calling; this is the backstop for when someone forgets.
func HistAdd(dst []uint32, src []uint32) bool {
	if len(dst) != len(src) {
		return false
	}
	for i, c := range src {
		dst[i] += c
	}
	return true
}

// HistLowFPS returns the "N% low" figure for a merged histogram: the mean frame
// time of the slowest fraction of frames, expressed as frames per second. Pass
// 0.01 for the 1% low, 0.001 for the 0.1% low.
//
// The result is an estimate, because a histogram knows which bin a frame landed
// in and not where inside it. With this layout the error is bounded by the bin
// width — under 15%, and in practice far less, since the slow tail usually
// spreads over several bins. It reports false when the histogram holds too few
// frames for the fraction to mean anything: one slow frame out of two hundred is
// not a 1% low, it is one slow frame, and publishing it as a statistic would
// invite conclusions the data cannot support.
func HistLowFPS(counts []uint32, fraction float64) (float64, bool) {
	if len(counts) != HistBins || fraction <= 0 || fraction >= 1 {
		return 0, false
	}
	total := HistTotal(counts)
	// Require the fraction to cover at least ten frames. Below that the figure is
	// dominated by whichever single frame happened to be slowest.
	//
	// Rounded up, matching the nearest-rank convention used elsewhere: the slowest
	// 1% of 999 frames is ten of them, not nine, and truncating would both measure
	// a slightly smaller tail than asked for and — right at the boundary — fail
	// this minimum for a run that clears it.
	want := uint64(math.Ceil(float64(total) * fraction))
	if want < 10 {
		return 0, false
	}
	// Walk from the slow end, accumulating frame time until the fraction is
	// covered. The last bin is taken partially, so the answer moves smoothly as
	// frames accumulate instead of stepping whenever a bin boundary is crossed.
	var (
		taken uint64
		sum   float64
	)
	for i := HistBins - 1; i >= 0 && taken < want; i-- {
		n := uint64(counts[i])
		if n == 0 {
			continue
		}
		if n > want-taken {
			n = want - taken
		}
		sum += float64(n) * HistMidpointsLog24V1[i]
		taken += n
	}
	if taken == 0 || sum <= 0 {
		return 0, false
	}
	meanMs := sum / float64(taken)
	return 1000 / meanMs, true
}

// HistMeanFPS returns the mean frame rate a histogram implies: the reciprocal of
// its mean frame time. Reported as false for an empty histogram rather than as
// zero, which would claim the machine rendered nothing.
func HistMeanFPS(counts []uint32) (float64, bool) {
	if len(counts) != HistBins {
		return 0, false
	}
	total := HistTotal(counts)
	if total == 0 {
		return 0, false
	}
	var sum float64
	for i, c := range counts {
		sum += float64(c) * HistMidpointsLog24V1[i]
	}
	if sum <= 0 {
		return 0, false
	}
	return 1000 / (sum / float64(total)), true
}

// RoundMs rounds a millisecond value to the three decimals the wire carries.
// Half away from zero, matching math.Round — the tie rule only has to be stated
// once and kept, and this is the one Go gives for free.
func RoundMs(v float64) float64 {
	return math.Round(v*1000) / 1000
}
