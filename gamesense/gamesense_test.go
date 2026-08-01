package gamesense

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestEnvelopeDiscriminatesBeforeTheTypedDecode(t *testing.T) {
	// The two capability-bearing messages disagree on the shape of what they
	// carry, which is why the reader is two-pass. Decoding either as the other
	// must not be attempted, and the envelope is what makes that decidable.
	lines := []string{
		`{"type":"probe","proto":3,"ok":true}`,
		`{"type":"hello","proto":3,"caps":["displayed"]}`,
	}
	want := []string{TypeProbe, TypeHello}
	for i, line := range lines {
		var env Envelope
		if err := json.Unmarshal([]byte(line), &env); err != nil {
			t.Fatalf("line %d: %v", i, err)
		}
		if env.Type != want[i] {
			t.Errorf("line %d: type = %q, want %q", i, env.Type, want[i])
		}
		if env.Proto != ProtoVersion {
			t.Errorf("line %d: proto = %d, want %d", i, env.Proto, ProtoVersion)
		}
	}
}

func TestProbeOmitsTheReasonWhenThereIsNone(t *testing.T) {
	b, err := json.Marshal(Probe{Type: TypeProbe, Proto: ProtoVersion, SensorVersion: "1.2.3", OK: true, PMVersion: "3.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	want := `{"type":"probe","proto":3,"sensor_version":"1.2.3","ok":true,"pm_version":"3.3.0"}`
	if got != want {
		t.Errorf("probe = %s, want %s", got, want)
	}
	// A blocked probe is the same message with the answer inverted and a code
	// attached; ok stays present because false is the answer, not a missing one.
	b, err = json.Marshal(Probe{Type: TypeProbe, Proto: ProtoVersion, SensorVersion: "1.2.3", Reason: ReasonServiceUnavailable})
	if err != nil {
		t.Fatal(err)
	}
	got = string(b)
	want = `{"type":"probe","proto":3,"sensor_version":"1.2.3","ok":false,"reason":"service_unavailable"}`
	if got != want {
		t.Errorf("blocked probe = %s, want %s", got, want)
	}
}

func TestStatusCarriesOnlyWhatItKnows(t *testing.T) {
	pid := 4242
	b, err := json.Marshal(Status{Type: TypeStatus, State: StateTracking, PID: &pid, Proc: "eldenring.exe", Title: "ELDEN RING"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"status","state":"tracking","pid":4242,"proc":"eldenring.exe","title":"ELDEN RING"}`
	if got := string(b); got != want {
		t.Errorf("tracking status = %s, want %s", got, want)
	}
	// Going idle says nothing about a process, because there no longer is one.
	b, err = json.Marshal(Status{Type: TypeStatus, State: StateIdle})
	if err != nil {
		t.Fatal(err)
	}
	want = `{"type":"status","state":"idle"}`
	if got := string(b); got != want {
		t.Errorf("idle status = %s, want %s", got, want)
	}
	// A tracked process that matched a profile carries its id; one that matched
	// nothing leaves the field out rather than naming an empty profile, because
	// "this is an unrecognized program" is a fact the reader acts on.
	b, err = json.Marshal(Status{Type: TypeStatus, State: StateTracking, PID: &pid, Proc: "cs2.exe", ProfileID: "gp_cs2"})
	if err != nil {
		t.Fatal(err)
	}
	want = `{"type":"status","state":"tracking","pid":4242,"proc":"cs2.exe","profile_id":"gp_cs2"}`
	if got := string(b); got != want {
		t.Errorf("matched status = %s, want %s", got, want)
	}
}

// The config line is the agent's only word to the sensor, so its shape is the
// whole handshake: type and proto are always present (the sensor rejects a
// mismatched proto), gpu is a decision and must survive as false rather than
// vanish, and profiles are omitted entirely when there are none.
func TestConfigStatesTheWholeRunUpFront(t *testing.T) {
	b, err := json.Marshal(Config{Type: TypeConfig, Proto: ProtoVersion, Mode: ModeAll})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"type":"config","proto":3,"gpu":false,"mode":"all"}`
	if got := string(b); got != want {
		t.Errorf("open config = %s, want %s", got, want)
	}

	in := Config{
		Type: TypeConfig, Proto: ProtoVersion, GPU: true, Mode: ModeProfiles,
		Profiles: []ConfigProfile{
			{ID: "gp_cs2", Exe: []string{"cs2.exe"}, TargetFPS: 240, Tier: TierDiag},
			{ID: "gp_er", Exe: []string{"eldenring.exe", "start_protected_game.exe"}, Tier: TierBase},
		},
	}
	b, err = json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Config
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Proto != ProtoVersion || !out.GPU || out.Mode != ModeProfiles {
		t.Errorf("config header = %+v", out)
	}
	if len(out.Profiles) != 2 || out.Profiles[0].TargetFPS != 240 || out.Profiles[1].TargetFPS != 0 {
		t.Errorf("profiles = %+v", out.Profiles)
	}
	if len(out.Profiles[1].Exe) != 2 || out.Profiles[1].Tier != TierBase {
		t.Errorf("second profile = %+v", out.Profiles[1])
	}
	// An unset target is absent, not a zero rate the sensor could act on.
	if strings.Contains(string(b), `"target_fps":0`) {
		t.Errorf("unset target_fps reached the wire: %s", b)
	}
}

func TestConfigMatch(t *testing.T) {
	cfg := Config{
		Type: TypeConfig, Proto: ProtoVersion, Mode: ModeProfiles,
		Profiles: []ConfigProfile{
			{ID: "gp_cs2", Exe: []string{"cs2.exe"}, Tier: TierDiag},
			{ID: "gp_er", Exe: []string{"eldenring.exe", "start_protected_game.exe"}, Tier: TierBase},
			// Deliberately claims a name an earlier profile already claims: two
			// profiles can name the same launcher, and the answer must not depend
			// on which one the code happens to visit first.
			{ID: "gp_dupe", Exe: []string{"CS2.EXE"}, Tier: TierBase},
		},
	}
	tests := []struct {
		name   string
		cfg    Config
		proc   string
		wantID string
		wantOK bool
	}{
		{"exact", cfg, "cs2.exe", "gp_cs2", true},
		{"upper-case process", cfg, "CS2.EXE", "gp_cs2", true},
		{"mixed-case process", cfg, "Cs2.Exe", "gp_cs2", true},
		{"mixed-case profile entry", cfg, "start_PROTECTED_game.exe", "gp_er", true},
		{"second entry of a profile", cfg, "eldenring.exe", "gp_er", true},
		{"first match wins", cfg, "cs2.EXE", "gp_cs2", true},
		{"no match", cfg, "notepad.exe", "", false},
		{"empty process", cfg, "", "", false},
		{"no profiles", Config{Type: TypeConfig, Proto: ProtoVersion, Mode: ModeAll}, "cs2.exe", "", false},
		{"profile with no names", Config{Profiles: []ConfigProfile{{ID: "gp_empty"}}}, "cs2.exe", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := tt.cfg.Match(tt.proc)
			if ok != tt.wantOK {
				t.Fatalf("Match(%q) ok = %v, want %v", tt.proc, ok, tt.wantOK)
			}
			if got.ID != tt.wantID {
				t.Errorf("Match(%q) id = %q, want %q", tt.proc, got.ID, tt.wantID)
			}
			if !ok && !reflect.DeepEqual(got, ConfigProfile{}) {
				t.Errorf("Match(%q) returned %+v alongside a false", tt.proc, got)
			}
		})
	}
}

// A source that cannot see a fact must leave the field out. This is the single
// most load-bearing rule in the format: a zero here would be read as a
// measurement, and "no frames were dropped" is a very different claim from "we
// cannot tell whether frames were dropped".
func TestUnknownCountsAreAbsentAndZeroCountsAreNot(t *testing.T) {
	base := Sample{
		Frames: Frames{Presented: 142},
		FT:     FrameTimes{Avg: 6.94, P50: 6.8, P95: 8.1, P99: 11.2, Max: 23, SD: 1.4},
		Hist:   Histogram{Layout: HistLayoutLog24V1, Counts: make([]uint32, HistBins)},
	}
	b, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"displayed", "dropped", "app", "generated", "disp_ft", "present", "quality"} {
		if strings.Contains(string(b), `"`+key+`"`) {
			t.Errorf("sample without %s capability still emitted %q: %s", key, key, b)
		}
	}

	zero := 0
	withZero := base
	withZero.Frames.Dropped = &zero
	b, err = json.Marshal(withZero)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"dropped":0`) {
		t.Errorf("a measured zero must survive the wire: %s", b)
	}
}

// Present has the same problem one level down: sync interval 0 means vsync off
// and tearing false is an observation, so neither can use the zero value to mean
// "unset".
func TestPresentKeepsMeaningfulZeroes(t *testing.T) {
	sync := 0
	tearing := false
	b, err := json.Marshal(Present{Mode: PresentModeHardwareIndependentFlip, Sync: &sync, Tearing: &tearing, API: APIDXGI})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"mode":"hardware_independent_flip","sync":0,"tearing":false,"api":"dxgi"}`
	if got := string(b); got != want {
		t.Errorf("present = %s, want %s", got, want)
	}
}

// The sec line and the uploaded bucket must carry byte-identical payloads: they
// are the same second, and the whole point of embedding Sample is that nobody
// has to keep two field lists in agreement.
func TestSecAndBucketShareOneSampleShape(t *testing.T) {
	ts := time.Date(2026, 8, 1, 12, 0, 4, 0, time.UTC)
	displayed, dropped := 140, 2
	sample := Sample{
		Frames: Frames{Presented: 142, Displayed: &displayed, Dropped: &dropped},
		FT:     FrameTimes{Avg: 6.94, P50: 6.8, P95: 8.1, P99: 11.2, Max: 23, SD: 1.4},
		Hist:   Histogram{Layout: HistLayoutLog24V1, Counts: make([]uint32, HistBins)},
	}
	secJSON, err := json.Marshal(Sec{Type: TypeSec, TS: ts, PID: 4242, Proc: "eldenring.exe", Sample: sample})
	if err != nil {
		t.Fatal(err)
	}
	bucketJSON, err := json.Marshal(Bucket{RunID: "run-1", TS: ts, Sample: sample})
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`"frames":{"presented":142,"displayed":140,"dropped":2}`, `"ft":{"avg":6.94`, `"ft_hist":{"layout":"log24_v1"`} {
		if !strings.Contains(string(secJSON), fragment) {
			t.Errorf("sec missing %s: %s", fragment, secJSON)
		}
		if !strings.Contains(string(bucketJSON), fragment) {
			t.Errorf("bucket missing %s: %s", fragment, bucketJSON)
		}
	}
	// Inlined, not nested: a reader must not have to unwrap a "sample" object.
	if strings.Contains(string(secJSON), `"Sample"`) || strings.Contains(string(secJSON), `"sample"`) {
		t.Errorf("sample leaked as a nested object: %s", secJSON)
	}
}

func TestSecRoundTrips(t *testing.T) {
	ts := time.Date(2026, 8, 1, 12, 0, 4, 123000000, time.UTC)
	displayed, dropped, app, generated := 140, 2, 71, 71
	sync := 1
	tearing := true
	counts := make([]uint32, HistBins)
	counts[12] = 140
	counts[16] = 2
	in := Sec{
		Type: TypeSec, TS: ts, PID: 4242, Proc: "eldenring.exe",
		Sample: Sample{
			Frames:  Frames{Presented: 142, Displayed: &displayed, Dropped: &dropped, App: &app, Generated: &generated},
			FT:      FrameTimes{Avg: 6.94, P50: 6.8, P95: 8.1, P99: 11.2, Max: 23, SD: 1.4},
			Hist:    Histogram{Layout: HistLayoutLog24V1, Counts: counts},
			DispFT:  &DispFT{Avg: 7.1, P95: 8.4},
			Present: &Present{Mode: PresentModeComposedFlip, Sync: &sync, Tearing: &tearing, API: APIDXGI, Changed: true},
			Quality: []string{QualityHistClipped},
		},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Sec
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !out.TS.Equal(in.TS) {
		t.Errorf("ts = %v, want %v", out.TS, in.TS)
	}
	if out.Proc != in.Proc || out.PID != in.PID {
		t.Errorf("process = %d/%q, want %d/%q", out.PID, out.Proc, in.PID, in.Proc)
	}
	if *out.Frames.Displayed != displayed || *out.Frames.Generated != generated {
		t.Errorf("frames = %+v", out.Frames)
	}
	if out.FT != in.FT {
		t.Errorf("ft = %+v, want %+v", out.FT, in.FT)
	}
	if out.Hist.Layout != HistLayoutLog24V1 || len(out.Hist.Counts) != HistBins || out.Hist.Counts[12] != 140 {
		t.Errorf("hist = %+v", out.Hist)
	}
	if out.Present == nil || *out.Present.Sync != sync || !out.Present.Changed {
		t.Errorf("present = %+v", out.Present)
	}
	if len(out.Quality) != 1 || out.Quality[0] != QualityHistClipped {
		t.Errorf("quality = %v", out.Quality)
	}
}

func TestRunLeavesEndedAtUnsetWhileRunning(t *testing.T) {
	start := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	b, err := json.Marshal(Run{ID: "run-1", Proc: "eldenring.exe", Title: "ELDEN RING", ProfileID: "gp_er", StartedAt: start, LastSeenAt: start.Add(time.Minute), Source: SourcePresentMonService, Caps: []string{CapDisplayed}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "ended_at") {
		t.Errorf("a live run must not carry an end: %s", b)
	}
	if !strings.Contains(string(b), `"profile_id":"gp_er"`) {
		t.Errorf("a profiled run must carry its profile: %s", b)
	}
	end := start.Add(2 * time.Minute)
	b, err = json.Marshal(Run{ID: "run-1", Proc: "eldenring.exe", StartedAt: start, LastSeenAt: end, EndedAt: &end})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"ended_at":"2026-08-01T12:02:00Z"`) {
		t.Errorf("a finished run must carry its end: %s", b)
	}
	// An unmatched process is recorded without a profile, not with an empty one.
	if strings.Contains(string(b), "profile_id") {
		t.Errorf("an unmatched run must not name a profile: %s", b)
	}
}
