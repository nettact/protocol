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
	// The GPU answer is separate and narrower: frames can be captured on a machine
	// whose driver publishes no adapter telemetry, so the first probe above — ok
	// without gpu_ok — is an ordinary machine and not a degraded one. It says so by
	// omitting the field rather than by carrying a false that reads like a fault.
	b, err = json.Marshal(Probe{Type: TypeProbe, Proto: ProtoVersion, SensorVersion: "1.2.3", OK: true, GPUOK: true, PMVersion: "3.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	got = string(b)
	want = `{"type":"probe","proto":3,"sensor_version":"1.2.3","ok":true,"gpu_ok":true,"pm_version":"3.3.0"}`
	if got != want {
		t.Errorf("gpu-capable probe = %s, want %s", got, want)
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
	for _, key := range []string{"displayed", "dropped", "app", "generated", "disp_ft", "present", "stutter", "proc", "quality"} {
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

// A watched second that held no hitch is a finding, and the most common one: a
// run is mostly smooth seconds, and a chart that cannot draw them has no
// baseline to show the bad ones against. Neither field carries omitempty, so the
// block's own presence is the entire distinction between "no stutter" and "no
// stutter detector".
func TestQuietSecondStaysAMeasurement(t *testing.T) {
	b, err := json.Marshal(Stutter{})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"count":0,"excess_ms":0}`
	if got := string(b); got != want {
		t.Errorf("quiet second = %s, want %s", got, want)
	}
	// And it survives being carried: a sample with the block still has it after
	// a round trip, where a sample without one still has none.
	sample := Sample{
		Frames:  Frames{Presented: 142},
		FT:      FrameTimes{Avg: 6.94, P50: 6.8, P95: 8.1, P99: 11.2, Max: 23, SD: 1.4},
		Hist:    Histogram{Layout: HistLayoutLog24V1, Counts: make([]uint32, HistBins)},
		Stutter: &Stutter{},
	}
	b, err = json.Marshal(sample)
	if err != nil {
		t.Fatal(err)
	}
	var out Sample
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Stutter == nil || *out.Stutter != (Stutter{}) {
		t.Errorf("quiet second lost in transit: %+v (%s)", out.Stutter, b)
	}
	sample.Stutter = nil
	b, err = json.Marshal(sample)
	if err != nil {
		t.Fatal(err)
	}
	out = Sample{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Stutter != nil {
		t.Errorf("an unwatched second acquired a stutter block: %+v (%s)", out.Stutter, b)
	}
}

// The process's readings come from queries that fail independently: CPU is a
// delta and the first observed second has nothing to subtract from, while memory
// is a level readable at once. The half that is missing stays absent rather than
// arriving as a zero that would draw a running game using no CPU at all.
func TestProcResCarriesOnlyTheHalfItHas(t *testing.T) {
	var ws, priv uint64 = 1 << 30, 1<<30 + 1<<28
	b, err := json.Marshal(ProcRes{WSBytes: &ws, PrivBytes: &priv})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ws_bytes":1073741824,"priv_bytes":1342177280}`
	if got := string(b); got != want {
		t.Errorf("memory-only proc = %s, want %s", got, want)
	}
	var out ProcRes
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.CPUPct != nil {
		t.Errorf("cpu appeared from nowhere: %v", *out.CPUPct)
	}
	if out.WSBytes == nil || *out.WSBytes != ws || out.PrivBytes == nil || *out.PrivBytes != priv {
		t.Errorf("memory = %+v", out)
	}
	// An idle-but-measured process is the mirror case: 0% is an observation and
	// must not be mistaken for the unmeasured CPU above.
	idle := 0.0
	b, err = json.Marshal(ProcRes{CPUPct: &idle})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"cpu_pct":0}`; got != want {
		t.Errorf("idle proc = %s, want %s", got, want)
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

// Inlining has a cost the compiler will not charge for: a Sample key that
// collides with one of its wrappers is not an error, it is a silent drop. The
// encoder resolves the shallower field and the deeper one simply never travels,
// so a sensor would fill a block that nothing downstream ever sees.
//
// The live collision is the process: Sec names the tracked process "proc", so
// the resource block cannot. Every wrapper key is checked, not just that one,
// because the next field added to Sample will not come with this reminder.
func TestSampleKeysDoNotCollideWithTheirWrappers(t *testing.T) {
	cpu := 42.5
	var ws uint64 = 1 << 30
	util, core := 96.5, 88.0
	var vram uint64 = 6 << 30
	sample := Sample{
		Frames:  Frames{Presented: 142},
		FT:      FrameTimes{Avg: 6.94},
		Hist:    Histogram{Layout: HistLayoutLog24V1, Counts: make([]uint32, HistBins)},
		Stutter: &Stutter{Count: 1, ExcessMs: 61.5},
		ProcRes: &ProcRes{CPUPct: &cpu, WSBytes: &ws},
		// The diag blocks are the newest arrivals and so the likeliest to have
		// picked a key one of the wrappers already owns.
		CPUSplit:       &CPUSplit{BusyAvg: 4.1, BusyP95: 5.9, WaitAvg: 2.8, WaitP95: 3.4},
		GPUSplit:       &GPUSplit{LatencyAvg: 1.2, TimeAvg: 6.1, TimeP95: 7.7, BusyAvg: 5.8, BusyP95: 7.2, WaitAvg: 0.3, InPresentAvg: 0.9, RenderLatencyAvg: 5.2},
		Latency:        &Latency{DisplayAvg: 21.4, AnimErrAvg: 1.1, AnimErrP95: 3.6},
		GPUTel:         &GPUTel{UtilPct: &util},
		ProcVRAM:       &ProcVRAM{Used: vram},
		BusiestCorePct: &core,
	}
	sampleKeys := make(map[string]struct{})
	for _, key := range ownJSONKeys(Sample{}) {
		sampleKeys[key] = struct{}{}
	}
	for _, wrapper := range []any{Sec{}, Bucket{}} {
		for _, key := range ownJSONKeys(wrapper) {
			if _, ok := sampleKeys[key]; ok {
				t.Errorf("%T's %q key is also a Sample key: one of them will never reach the wire", wrapper, key)
			}
		}
	}

	// And the whole sample does survive the inlining it shares a namespace with.
	var out Sec
	b, err := json.Marshal(Sec{Type: TypeSec, TS: time.Now(), PID: 4242, Proc: "cs2.exe", Sample: sample})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Proc != "cs2.exe" {
		t.Errorf("process name = %q, want cs2.exe", out.Proc)
	}
	if out.ProcRes == nil || out.ProcRes.CPUPct == nil || *out.ProcRes.CPUPct != cpu {
		t.Errorf("resource block did not survive the sec line: %+v (%s)", out.ProcRes, b)
	}
	if out.Stutter == nil || out.Stutter.Count != 1 {
		t.Errorf("stutter did not survive the sec line: %+v (%s)", out.Stutter, b)
	}
	if out.CPUSplit == nil || *out.CPUSplit != *sample.CPUSplit {
		t.Errorf("cpu split did not survive the sec line: %+v (%s)", out.CPUSplit, b)
	}
	if out.GPUSplit == nil || *out.GPUSplit != *sample.GPUSplit {
		t.Errorf("gpu split did not survive the sec line: %+v (%s)", out.GPUSplit, b)
	}
	if out.Latency == nil || *out.Latency != *sample.Latency {
		t.Errorf("latency did not survive the sec line: %+v (%s)", out.Latency, b)
	}
	if out.GPUTel == nil || out.GPUTel.UtilPct == nil || *out.GPUTel.UtilPct != util {
		t.Errorf("gpu telemetry did not survive the sec line: %+v (%s)", out.GPUTel, b)
	}
	if out.ProcVRAM == nil || out.ProcVRAM.Used != vram {
		t.Errorf("process vram did not survive the sec line: %+v (%s)", out.ProcVRAM, b)
	}
	if out.BusiestCorePct == nil || *out.BusiestCorePct != core {
		t.Errorf("busiest core did not survive the sec line: %+v (%s)", out.BusiestCorePct, b)
	}
}

// ownJSONKeys returns the JSON names v's own fields claim, skipping embedded
// ones — those are the other side of the comparison, not part of it. Marshalling
// would not do: a wrapper's output already contains the inlined sample's keys,
// which is the very flattening under test.
func ownJSONKeys(v any) []string {
	rt := reflect.TypeOf(v)
	var keys []string
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Anonymous {
			continue
		}
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" {
			name = f.Name
		}
		keys = append(keys, name)
	}
	return keys
}

func TestSecRoundTrips(t *testing.T) {
	ts := time.Date(2026, 8, 1, 12, 0, 4, 123000000, time.UTC)
	displayed, dropped, app, generated := 140, 2, 71, 71
	sync := 1
	tearing := true
	cpu := 42.5
	var ws, priv uint64 = 1 << 30, 1<<30 + 1<<28
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
			Stutter: &Stutter{Count: 2, ExcessMs: 118.4},
			ProcRes: &ProcRes{CPUPct: &cpu, WSBytes: &ws, PrivBytes: &priv},
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
	if out.Stutter == nil || *out.Stutter != *in.Stutter {
		t.Errorf("stutter = %+v, want %+v", out.Stutter, in.Stutter)
	}
	if out.ProcRes == nil || *out.ProcRes.CPUPct != cpu || *out.ProcRes.WSBytes != ws || *out.ProcRes.PrivBytes != priv {
		t.Errorf("proc_res = %+v", out.ProcRes)
	}
	if len(out.Quality) != 1 || out.Quality[0] != QualityHistClipped {
		t.Errorf("quality = %v", out.Quality)
	}
}

// The frame-derived diag blocks are group-atomic: the sensor registers whole
// metric groups when the session opens, so a block either arrives measured in
// full or does not arrive at all. That is what lets their fields carry no
// presence — and it is also what an omitempty added out of habit would break,
// silently turning a frame that waited no time at all into one nobody measured.
func TestDiagBlocksCarryEveryFieldIncludingZero(t *testing.T) {
	for _, block := range []any{CPUSplit{}, GPUSplit{}, Latency{}} {
		b, err := json.Marshal(block)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]json.RawMessage
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		want := ownJSONKeys(block)
		if len(got) != len(want) {
			t.Errorf("%T at zero has %d keys, want %d (%v): %s", block, len(got), len(want), want, b)
		}
		for _, key := range want {
			if _, ok := got[key]; !ok {
				t.Errorf("%T dropped %q at zero: %s", block, key, b)
			}
		}
	}
}

// Whole-GPU telemetry is the exception to the group-atomic rule, because which
// figures a driver publishes differs by vendor and by metric. A card that
// reports utilization and no memory is an ordinary card, so the memory fields
// stay absent rather than arriving as a zero that would draw an empty framebuffer.
func TestGPUTelReportsOnlyWhatTheDriverPublishes(t *testing.T) {
	util := 96.5
	b, err := json.Marshal(GPUTel{UtilPct: &util})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"util_pct":96.5}`; got != want {
		t.Errorf("utilization-only card = %s, want %s", got, want)
	}
	var out GPUTel
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.MemUsed != nil || out.MemSize != nil {
		t.Errorf("memory appeared from a card that never reported it: %+v", out)
	}
	// And the mirror: a card sitting idle under a paused game reports 0%, which is
	// a measurement and must not be mistaken for the unpublished figures above.
	idle := 0.0
	b, err = json.Marshal(GPUTel{UtilPct: &idle})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"util_pct":0}`; got != want {
		t.Errorf("idle card = %s, want %s", got, want)
	}
}

// A process's VRAM level is worth recording with or without the OS budget that
// would contextualize it: the level is the measurement, the budget is the
// yardstick, and a source that cannot supply the second must not cost us the
// first. Used carries no presence for the same reason — inside a block that
// exists, zero bytes committed is an observation.
func TestProcVRAMKeepsTheLevelWithoutTheBudget(t *testing.T) {
	var used uint64 = 6 << 30
	b, err := json.Marshal(ProcVRAM{Used: used})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"used":6442450944}`; got != want {
		t.Errorf("budget-less vram = %s, want %s", got, want)
	}
	var out ProcVRAM
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Budget != nil {
		t.Errorf("budget appeared from nowhere: %v", *out.Budget)
	}
	var budget uint64 = 8 << 30
	b, err = json.Marshal(ProcVRAM{Used: used, Budget: &budget})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(b), `{"used":6442450944,"budget":8589934592}`; got != want {
		t.Errorf("budgeted vram = %s, want %s", got, want)
	}
}

// Degradation is a property of a second, not of a run. When the sensor's
// boundary work overruns its budget it stops polling for the rest of the run,
// but the frame-derived breakdowns cost nothing extra at the boundary and keep
// arriving. So a degraded second is partial, and it says which half it lost by
// dropping exactly the polled blocks and flagging itself — not by revising the
// run's capabilities, which would make every earlier second unexplainable.
func TestDegradedSecondKeepsWhatTheFramesStillProvide(t *testing.T) {
	sample := Sample{
		Frames:   Frames{Presented: 142},
		FT:       FrameTimes{Avg: 6.94, P50: 6.8, P95: 8.1, P99: 11.2, Max: 23, SD: 1.4},
		Hist:     Histogram{Layout: HistLayoutLog24V1, Counts: make([]uint32, HistBins)},
		CPUSplit: &CPUSplit{BusyAvg: 4.1, BusyP95: 5.9, WaitAvg: 2.8, WaitP95: 3.4},
		GPUSplit: &GPUSplit{LatencyAvg: 1.2, TimeAvg: 6.1, TimeP95: 7.7, BusyAvg: 5.8, BusyP95: 7.2, WaitAvg: 0.3, InPresentAvg: 0.9, RenderLatencyAvg: 5.2},
		Latency:  &Latency{DisplayAvg: 21.4, AnimErrAvg: 1.1, AnimErrP95: 3.6},
		Quality:  []string{QualityDiagDegraded},
	}
	b, err := json.Marshal(sample)
	if err != nil {
		t.Fatal(err)
	}
	var out Sample
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"gpu_tel", "proc_vram", "busiest_core_pct"} {
		if strings.Contains(string(b), key) {
			t.Errorf("a degraded second still carries the polled %q: %s", key, b)
		}
	}
	if out.CPUSplit == nil || out.GPUSplit == nil || out.Latency == nil {
		t.Errorf("degradation cost the frame-derived blocks: %s", b)
	}
	if len(out.Quality) != 1 || out.Quality[0] != QualityDiagDegraded {
		t.Errorf("quality = %v, want the degradation flag", out.Quality)
	}
}

// Capabilities are matched as strings by every consumer, so two of them sharing
// a value would not fail to compile — it would make the pair indistinguishable,
// and a console would light up a block the sensor never fills.
func TestCapabilitiesAreDistinct(t *testing.T) {
	seen := make(map[string]struct{})
	for _, cap := range []string{
		CapDisplayed, CapFrameType, CapPresentMeta, CapPerFrameComplete, CapStutter,
		CapProcCPU, CapProcMem,
		CapCPUSplit, CapGPUSplit, CapLatency, CapGPUTel, CapProcVRAM, CapBusiestCore,
	} {
		if _, dup := seen[cap]; dup {
			t.Errorf("capability %q is declared twice", cap)
		}
		seen[cap] = struct{}{}
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
