// Package gamesense defines the contract for game presentation data: the
// line-delimited JSON a sensor component writes on its stdout, the single
// config line the agent writes back on its stdin, and the run / second-bucket
// records the agent uploads from it.
//
// Both halves live here because they carry the same facts. A sensor observes one
// second of frame presentation and describes it; the agent groups those seconds
// into runs and forwards them unchanged. Defining the payload once is why a
// Bucket embeds the same Sample the sensor emitted instead of restating its
// fields — the two ends cannot drift apart because there is only one definition
// to change.
//
// # Unknown is not zero
//
// A sensor reports what its data source actually provides, and sources differ:
// one may know whether each frame reached the screen, another only that a frame
// was presented. Every field a source may be unable to fill is a pointer or a
// slice, absent from the JSON when unknown. Nothing here ever substitutes zero
// for a missing measurement — "the game dropped no frames" and "we cannot see
// dropped frames" are different facts, and a chart that renders them alike is
// lying. The capabilities on Hello say up front which fields to expect.
//
// The package is stdlib-only, like the other protocol type packages, so the
// sensor, the agent, the server, and their tests all import it without pulling
// anything else in.
package gamesense

import (
	"strings"
	"time"
)

// ProtoVersion is the sensor protocol this build speaks. The agent requires an
// exact match: a sensor ships with the agent rather than independently, so a
// mismatch is a broken install, not a version to negotiate.
//
// 4 added the machine-level second and the frameless-second line, and removed
// the whole-adapter telemetry and the busiest core from Sec. Both directions of
// a mismatched pair lose data silently without this bump — an older agent
// ignores lines it has no case for, and a newer one decodes a Sec whose moved
// fields simply are not there — which is exactly what the exact-match check is
// for.
const ProtoVersion = 4

// Message types. Every line is exactly one of these, discriminated by Type. A
// reader decodes Envelope first and then the matching struct: probe and hello
// both carry information about capability but in different shapes, so no single
// union struct can decode the stream.
//
// All but TypeConfig travel from the sensor to the agent; TypeConfig is the one
// line that travels the other way.
const (
	TypeProbe  = "probe"
	TypeHello  = "hello"
	TypeSec    = "sec"
	TypeStatus = "status"
	TypeConfig = "config"
	// TypeHost is the machine-level per-second line: what the WHOLE MACHINE was
	// doing during the second that just closed, as opposed to what the tracked
	// game was doing in it.
	//
	// It is a separate line rather than a block on Sec because the two have
	// different subjects and different lifetimes. A Sec belongs to one process and
	// exists only for a second that held frames; this exists for every second the
	// sensor is watching anything at all — including the alt-tabbed minute and the
	// loading screen, where the game produced nothing and the machine is precisely
	// what a reader wants to see. Folding it into Sec would make the machine's
	// record disappear exactly when it matters, and would file a fact about the
	// adapter under whichever process happened to win the second.
	TypeHost = "host"
	// TypeGap is the per-second line for a second the tracked game presented
	// nothing, carrying which of the two silences it was. Sec's counterpart for
	// the seconds Sec cannot describe.
	TypeGap = "gap"
)

// Tracking modes carried by a Config, deciding which presenting processes the
// sensor reports at all.
//
// The distinction is a privacy and volume choice, not a capability one: a site
// that has named the games it cares about does not want every other program
// that touches the screen recorded, while a site with no profiles yet would
// learn nothing from a sensor that reports nothing.
const (
	ModeAll      = "all"      // track every presenting process
	ModeProfiles = "profiles" // strict: only track processes matching a profile
)

// Profile tiers, naming how much a matched game is measured. Base is the frame
// data every profile gets; diag adds the deeper per-frame breakdowns, which cost
// more to collect and are only worth it on the games a user is diagnosing.
const (
	TierBase = "base"
	TierDiag = "diag"
)

// Sensor states carried by a status message.
//
// The states are about the sensor, not the machine: idle means nothing is
// presenting, which is the ordinary state of a desktop and not a fault. Only
// error names something the agent cannot observe for itself.
const (
	StateIdle     = "idle"
	StateTracking = "tracking"
	StateError    = "error"
)

// Reason codes for game capture, naming why it is not available. They travel on
// a probe or status line, and they are also the vocabulary the agent puts in
// permission.PermissionReport.UnsupportedReasons for the game permissions — so
// the block covers causes the AGENT observes about the sensor as much as ones
// the sensor reports about itself.
//
// Each reason exists because its remedy differs. Missing middleware is an
// install problem, an unavailable service is a "start or repair it" problem, and
// a version mismatch means the middleware and the service were upgraded apart —
// telling a user "frame capture failed" for all three would be true and useless.
//
// This block is not the whole vocabulary. The agent adds three codes of its own
// in agent/internal/gamesense — "probe_failed", "proto_mismatch" and
// "sensor_exited" — for the failures that happen around a sensor run rather than
// inside one, where no line from the sensor exists to carry a reason. They are
// not defined here because nothing in this contract emits them, but a reader of
// a permission report will see them, which is the other half of why a reader
// must fall back to generic text on an unrecognized code instead of assuming
// this list is exhaustive.
const (
	// ReasonUnsupportedOS: this platform has no frame source at all. Terminal.
	ReasonUnsupportedOS = "unsupported_os"
	// ReasonSensorMissing: no sensor component was found beside the agent — the
	// build ships none, or it was removed. Nothing about the machine is wrong, so
	// there is nothing on it to install; the fix is an agent build that carries a
	// sensor.
	ReasonSensorMissing = "sensor_missing"
	// ReasonPresentMonMissing: the PresentMon middleware library was not found in
	// any of the locations the sensor searches. The component is not installed.
	ReasonPresentMonMissing = "presentmon_missing"
	// ReasonServiceUnavailable: the middleware is present but no service answered
	// — installed and stopped, or never installed.
	ReasonServiceUnavailable = "service_unavailable"
	// ReasonVersionMismatch: the middleware and the service are different builds.
	// PresentMon refuses the pairing outright, so this is never a partial-data
	// state; the fix is to upgrade both together.
	ReasonVersionMismatch = "version_mismatch"
	// ReasonSessionLost: capture was working and the service connection dropped
	// (service stopped or restarted under a running session).
	ReasonSessionLost = "session_lost"
	// ReasonInternalError: a defect in the sensor itself. Distinct from the
	// environmental reasons above because no user action fixes it.
	ReasonInternalError = "internal_error"
	// ReasonGPUTelemetryUnavailable: capture works, but this adapter or driver
	// publishes no telemetry, so game.gpu.read cannot be supported. An ordinary
	// machine, not a fault — and nothing to install. It is the one reason here
	// that leaves frame capture entirely intact, which is why it can only ever
	// explain game.gpu.read and never the two permissions beneath it.
	ReasonGPUTelemetryUnavailable = "gpu_telemetry_unavailable"
	// ReasonNotLicensed: a Store-edition sensor found no active Microsoft Store
	// license for the app, so it refuses to collect. The machine is capable; the
	// remedy is to install (or purchase) NetTact from the Microsoft Store rather
	// than to fix anything about the capture stack.
	ReasonNotLicensed = "not_licensed"
)

// Capabilities a sensor declares on Hello, naming the optional fields this
// session will actually fill. They are advisory in one direction only: a
// capability that is absent guarantees the field will be absent, while a
// declared capability can still yield an absent field for an individual second
// (nothing presented that could carry it).
//
// Readers must tolerate unknown strings. A newer sensor paired with an older
// console should degrade to ignoring a capability, never to refusing the stream.
const (
	// CapDisplayed: frames are tracked through to the screen, so the displayed
	// and dropped counts and the displayed-frame intervals are real.
	CapDisplayed = "displayed"
	// CapFrameType: application frames are distinguished from driver-generated
	// ones. Without it, a game running frame generation reports a presented rate
	// that is roughly double what the engine actually simulated.
	CapFrameType = "frame_type"
	// CapPresentMeta: presentation mode, sync interval, tearing and graphics API
	// are observed.
	CapPresentMeta = "present_meta"
	// CapPerFrameComplete: every presented frame is observed individually, rather
	// than sampled. Statistics from a sampled source describe the samples, not
	// the second, and the distinction belongs on the record.
	CapPerFrameComplete = "per_frame_complete"
	// CapStutter: long frames are detected against a rolling baseline, so each
	// second carries how many hitches it held and how much time they cost.
	// Without it the interval statistics are all a reader has, and a p99 cannot
	// say whether one second held one bad frame or ten.
	CapStutter = "stutter"
	// CapProcCPU: the tracked game process's own CPU usage is sampled once per
	// second. Scoped to that process rather than the machine — "the box is busy"
	// and "the game is busy" lead to different fixes.
	CapProcCPU = "proc_cpu"
	// CapProcMem: the tracked game process's memory footprint is sampled once
	// per second. Separate from CapProcCPU because the two readings come from
	// different queries and one can be available while the other is not.
	CapProcMem = "proc_mem"
	// CapCPUSplit: every frame's CPU time is split into the work the game did
	// and the time it spent waiting. Frame intervals say a second was slow; the
	// split says which side was holding it up, which is the difference between a
	// finding and a number.
	CapCPUSplit = "cpu_split"
	// CapGPUSplit: every frame's GPU side is broken out — how long before the GPU
	// started, how long it ran, how long it idled — along with the present path
	// around it. Read against CapCPUSplit it names the bottleneck: a busy GPU
	// with a waiting CPU is one verdict and the mirror image is the other.
	CapGPUSplit = "gpu_split"
	// CapLatency: display latency and animation error are observed per frame, so
	// the second can say how long a frame took to reach the screen and how far
	// the game's own pacing drifted from what was shown.
	CapLatency = "latency"
	// CapProcVRAM: the game process's own dedicated video memory is read once a
	// second. Distinct from the whole-card figure on the machine-level stream for
	// the same reason CapProcCPU is distinct from the machine's CPU usage.
	CapProcVRAM = "proc_vram"
)

// There is deliberately no capability for the whole-card or whole-machine
// readings, and their absence from this list is the point.
//
// A capability is a promise about what a RUN will contain, stated once when the
// run opens and never revised. HostSample is not part of a run: it is keyed by
// the machine and the second, it is collected for seconds no run covers, and one
// second of it is read by every run that overlaps it. There is no run whose
// opening could promise it and no run whose caps could describe its absence.
//
// What replaces the promise is the schema's own rule: NULL means NOT MEASURED.
// A reader asking "does this machine report adapter utilization" answers it by
// looking for a non-null reading in the window it cares about, which is a
// stronger answer than a capability anyway — a capability says the sensor
// intended to collect it, while a value says it did.

// Sources. The identifier is stored with every run so a later comparison
// between two runs can tell apart a real change in the machine from a change in
// how it was measured.
const SourcePresentMonService = "presentmon_service"

// Quality flags on a second, recording something about how the second was
// measured rather than about the frames in it. Open-ended: a reader must ignore
// flags it does not know.
const (
	// QualityHistClipped: at least one frame interval fell outside the
	// histogram's range and was counted in an end bin. The counts still sum to
	// the frame total, but the tail is not where it appears.
	QualityHistClipped = "hist_clipped"
	// QualityConsumeBacklog: a read returned a full buffer, so frames may have
	// been produced faster than they were consumed and some may belong to a
	// neighbouring second.
	QualityConsumeBacklog = "consume_backlog"
	// QualityDiagDegraded: the sensor's per-second boundary work exceeded its
	// wall-clock budget, and diagnostic polling was stopped for the rest of the
	// run. The frame-derived blocks continue — they cost nothing extra at the
	// boundary — so a degraded second is partial, not empty.
	//
	// It is a per-second flag rather than a capability change because
	// capabilities are stated once and never revised: a run's caps describe what
	// the run set out to measure, and rewriting them mid-run would make every
	// earlier second retroactively unexplainable. The flag marks exactly the
	// seconds that lost the polled blocks, which is the honest granularity.
	QualityDiagDegraded = "diag_degraded"
	// QualityHostDegraded is QualityDiagDegraded's counterpart on the
	// machine-level stream: the sensor's boundary work exceeded its wall-clock
	// budget and the polled readings were given up for the rest of the run.
	//
	// A second flag rather than a reuse of QualityDiagDegraded because the two
	// travel on records with different subjects, and a reader holding a machine
	// second has no bucket to read the other flag off. It is also the only thing
	// that tells an all-empty host second apart from one the machine simply had
	// nothing to report for: without it, "we stopped asking" and "we asked and
	// the driver publishes nothing" would be the same row.
	QualityHostDegraded = "host_degraded"
)

// Gap reasons, naming which of the two silences a frameless second was.
//
// There are two because the remedies are opposite, and a chart that shaded both
// alike would answer neither question. Recording only "no frames" would be the
// same as recording nothing: the blank stretch is already visible, and what a
// reader cannot see is which kind it was.
const (
	// GapBackground: the tracked program did not own the focused window. The
	// player alt-tabbed, minimized, or moved to a second monitor. Nothing is
	// wrong; the blank stretch is time nobody was playing, and the figures around
	// it should not be read as though the game had stalled.
	GapBackground = "background"
	// GapNoFrames: the tracked program owned the focused window and presented
	// nothing anyway — a loading screen, a shader cache being built, a
	// pre-rendered cutscene, a paused menu. The player was sitting there waiting,
	// which is an experience worth measuring and the opposite of the above.
	GapNoFrames = "no_frames"
)

// Presentation modes, mapped from the source's own enumeration. The wire carries
// names rather than vendor ordinals so a stored value stays readable in a
// database and so a renumbering in a future release of the source cannot
// silently reinterpret recorded history.
const (
	PresentModeHardwareLegacyFlip              = "hardware_legacy_flip"
	PresentModeHardwareLegacyCopy              = "hardware_legacy_copy"
	PresentModeHardwareIndependentFlip         = "hardware_independent_flip"
	PresentModeComposedFlip                    = "composed_flip"
	PresentModeComposedCopyGPUGDI              = "composed_copy_gpu_gdi"
	PresentModeComposedCopyCPUGDI              = "composed_copy_cpu_gdi"
	PresentModeHardwareComposedIndependentFlip = "hardware_composed_independent_flip"
)

// Graphics APIs, mapped the same way and for the same reason. A source that
// cannot identify the API omits the field rather than naming it "unknown":
// there is nothing a reader would do differently with the two.
const (
	APIDXGI = "dxgi"
	APID3D9 = "d3d9"
)

// Envelope is the discriminator prefix every line begins with. Decode it first,
// then the struct its Type names.
type Envelope struct {
	Type  string `json:"type"`
	Proto int    `json:"proto"`
}

// Config is the one line that travels toward the sensor: the agent writes it to
// the sensor's stdin immediately after spawn, and writes nothing else for the
// rest of the run. Closing stdin remains the stop signal, so the sensor keeps
// reading after this line purely to see the EOF.
//
// One line rather than a stream because the sensor's configuration is fixed for
// the life of a run: when the profiles or the mode change, the agent restarts
// the sensor with a new Config. That keeps the sensor free of reconfiguration
// state, and keeps a run describable by a single set of rules — a run whose
// filtering changed midway would be two runs wearing one id.
type Config struct {
	Type  string `json:"type"`  // TypeConfig
	Proto int    `json:"proto"` // ProtoVersion
	// GPU is whether game.gpu.read is effective for this agent, i.e. whether the
	// sensor may collect adapter-level telemetry. Consumed in a later batch.
	GPU      bool            `json:"gpu"`
	Mode     string          `json:"mode"` // ModeAll | ModeProfiles
	Profiles []ConfigProfile `json:"profiles,omitempty"`
}

// ConfigProfile is one named game as the sensor needs to know it: what to match
// on, and how closely to watch it once matched. It is the pushed GameProfile
// stripped to the fields that change sensor behaviour — the display name and the
// linked monitors stay on the server, where they are read.
type ConfigProfile struct {
	ID        string   `json:"id"`
	Exe       []string `json:"exe"`                  // process names, matched case-insensitively ("cs2.exe")
	TargetFPS int      `json:"target_fps,omitempty"` // 0 = unset
	Tier      string   `json:"tier"`                 // TierBase | TierDiag
}

// Match returns the profile the named process belongs to, comparing process
// names case-insensitively because the source of a process name (the OS, a
// launcher, a user typing into the console) does not agree on case and none of
// them mean anything by it. The first matching profile wins, so a process listed
// twice resolves to the earlier profile rather than to whichever the map
// iteration happened to reach.
//
// This is the only implementation of the matching rule. The sensor uses it to
// decide what to track and which profile id to stamp on a status line, and any
// other consumer that needs to ask the same question must call it rather than
// re-derive it: two spellings of "does this process match" is exactly how a
// profile starts recording on one side of the system and not the other.
func (c Config) Match(proc string) (ConfigProfile, bool) {
	for _, p := range c.Profiles {
		for _, exe := range p.Exe {
			if strings.EqualFold(exe, proc) {
				return p, true
			}
		}
	}
	return ConfigProfile{}, false
}

// Probe is the single line a `--probe` run prints before exiting. It answers one
// question — can this machine capture frames right now — and, when the answer is
// no, says why in a code the console can act on.
type Probe struct {
	Type          string `json:"type"`
	Proto         int    `json:"proto"`
	SensorVersion string `json:"sensor_version"`
	OK            bool   `json:"ok"`
	Reason        string `json:"reason,omitempty"`
	// GPUOK reports the second, narrower question: beyond opening a frame
	// session, the probe registered a minimal GPU-telemetry query and it was
	// accepted. That is a separate answer because the two fail apart — a machine
	// whose driver exposes no adapter telemetry captures frames perfectly well —
	// and it is what gates game.gpu.read on the agent side. False alongside a
	// true OK is an ordinary machine, not a fault, so it carries no reason of its
	// own.
	GPUOK bool `json:"gpu_ok,omitempty"`
	// PMVersion is the frame source's own version, when one could be read. It is
	// diagnostic only: the OK/Reason pair is the whole decision.
	PMVersion string `json:"pm_version,omitempty"`
}

// Hello is the first line of a capture run, sent once the source is open and its
// query registered — which is why it can state the capabilities rather than
// promise them. A Hello with no source is a run that failed to start; the status
// line that follows carries the reason.
type Hello struct {
	Type          string   `json:"type"`
	Proto         int      `json:"proto"`
	SensorVersion string   `json:"sensor_version"`
	Source        string   `json:"source,omitempty"`
	PMVersion     string   `json:"pm_version,omitempty"`
	Caps          []string `json:"caps,omitempty"`
}

// Status reports a change in what the sensor is doing. It is emitted on
// transition rather than on a schedule: entering or leaving tracking, changing
// the tracked process, a changed window title, or a terminal error.
type Status struct {
	Type  string `json:"type"`
	State string `json:"state"`
	PID   *int   `json:"pid,omitempty"`
	Proc  string `json:"proc,omitempty"`
	// Title is the tracked window's title. It names the run for a human — a
	// process name identifies the program, the title usually identifies what is
	// being played inside it. Absent when no window could be read, which is
	// ordinary for a game mid-launch.
	Title string `json:"title,omitempty"`
	// ProfileID names the Config profile the tracked process matched, when one
	// did. Absent means the process matched nothing — which under ModeAll is an
	// ordinary recorded process and under ModeProfiles cannot happen. The sensor
	// decides this once, so the agent never re-runs the match against a profile
	// list it might hold a different generation of.
	ProfileID string `json:"profile_id,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

// Frames counts one second of frames by outcome.
//
// Presented is the only count every source knows: it is the number of frames the
// application handed to the system. The rest describe what happened to them, and
// are absent when the source cannot see that far. App and Generated split the
// presented frames by origin and only exist under CapFrameType; a game using
// frame generation presents roughly twice what it simulated, and the smoothness
// a player feels follows the application frames.
type Frames struct {
	Presented int  `json:"presented"`
	Displayed *int `json:"displayed,omitempty"`
	Dropped   *int `json:"dropped,omitempty"`
	App       *int `json:"app,omitempty"`
	Generated *int `json:"generated,omitempty"`
}

// FrameTimes describes the distribution of frame intervals within one second, in
// milliseconds.
//
// The intervals are the ones the APPLICATION produced. Under CapFrameType that
// excludes frames a driver or upscaler generated, which are interpolated at even
// spacing and would otherwise smooth over the hitches these figures exist to
// expose. Presented counts every frame regardless — the display did update that
// often — so the two answer different questions and are meant to be read
// together.
//
// Percentiles are nearest-rank over the second's own samples: with the handful
// of frames a second holds, interpolating between two of them would invent
// precision the sample size does not support. Values are rounded to three
// decimals, half away from zero. SD is the population standard deviation,
// because the second is the whole population being described, not a sample of a
// larger one.
//
// These are within-second statistics and must not be averaged across seconds to
// describe a run: a mean of per-second p95s is not the run's p95. Whole-run
// figures come from summing the histograms.
type FrameTimes struct {
	Avg float64 `json:"avg"`
	P50 float64 `json:"p50"`
	P95 float64 `json:"p95"`
	P99 float64 `json:"p99"`
	Max float64 `json:"max"`
	SD  float64 `json:"sd"`
}

// Histogram is the second's frame intervals binned by a named, frozen layout.
// It covers the same intervals FrameTimes summarizes — see there.
//
// It exists so whole-run figures stay honest. The 1% low a player cares about is
// a property of every frame in a session, and per-second summaries cannot
// reconstruct it; histograms can simply be added together. Layout names the bin
// edges, and a reader that does not recognize the layout must refuse to
// interpret the counts rather than assume its own.
type Histogram struct {
	Layout string   `json:"layout"`
	Counts []uint32 `json:"counts"`
}

// DispFT summarizes the intervals between frames actually reaching the screen,
// as opposed to the intervals at which the application produced them. The two
// diverge whenever frames are dropped or the display holds one longer than
// intended, and it is the displayed rhythm a player sees. Present only under
// CapDisplayed.
type DispFT struct {
	Avg float64 `json:"avg"`
	P95 float64 `json:"p95"`
}

// Present is how the second's frames reached the screen. Each field carries the
// second's dominant value; Changed records that the second was not uniform, so a
// reader knows the single value is a summary of a transition (a game entering
// fullscreen, vsync toggling) rather than a steady state.
//
// Sync and Tearing are pointers because their zero values are meaningful: sync
// interval 0 means vsync off, and tearing false is a real observation.
type Present struct {
	Mode    string `json:"mode,omitempty"`
	Sync    *int   `json:"sync,omitempty"`
	Tearing *bool  `json:"tearing,omitempty"`
	API     string `json:"api,omitempty"`
	Changed bool   `json:"changed,omitempty"`
}

// Stutter reports the long-frame events of one second, produced by the sensor's
// adaptive detector: the baseline is a rolling 30 s median of application frame
// intervals, a candidate long frame is one exceeding max(50 ms, 2.5 × baseline),
// and consecutive candidates merge into a single event attributed to the second
// the event started in.
//
// The threshold is adaptive because a fixed one measures the wrong thing: 20 ms
// is a severe hitch at 240 fps and an ordinary frame at 45. Merging matters for
// the same reason — a 400 ms freeze spanning six frames is one thing that
// happened to the player, not six.
//
// A present block with Count 0 is a real measurement: the second was watched and
// held no stutter. Absence means nothing watched, which is why neither field
// carries omitempty — a zero here must reach the wire.
type Stutter struct {
	Count    int     `json:"count"`
	ExcessMs float64 `json:"excess_ms"` // sum of (frame time − baseline) across candidate frames, ms
}

// ProcRes is the game process's own resource usage, sampled once at the second
// boundary rather than derived from the frame stream. It answers the question
// the frame data cannot: whether a bad second was the game running out of room
// or something else on the machine taking it.
//
// The inner fields are pointers because the two sources fail independently. CPU
// is a delta and needs two samples, so the first observed second of a run has
// none; memory is a level and can be read at once.
//
// The key is "proc_res" rather than the shorter "proc" because Sample is
// embedded into Sec and Bucket: "proc" is already the tracked process's name
// there, and a duplicate would be resolved away by the JSON encoder rather than
// reported, leaving a block that silently never travelled.
type ProcRes struct {
	CPUPct    *float64 `json:"cpu_pct,omitempty"`    // % of total CPU capacity (all cores), 0-100
	WSBytes   *uint64  `json:"ws_bytes,omitempty"`   // working set
	PrivBytes *uint64  `json:"priv_bytes,omitempty"` // private (committed) bytes
}

// The diag blocks below are the deeper breakdowns a TierDiag profile buys. They
// come in two kinds and share one rule.
//
// CPUSplit, GPUSplit and Latency are frame-derived: each figure aggregates the
// second's APPLICATION frames, the same population FrameTimes describes, so a
// breakdown lines up frame for frame with the interval it exists to explain.
// Their values are milliseconds, and like FrameTimes they are within-second
// statistics — averaging a run's per-second p95s does not produce the run's p95.
// ProcVRAM is the other kind: a single reading taken once at the second
// boundary, in the units its comment names.
//
// The shared rule is that a block is group-atomic — if it is present, every
// field in it was measured. That mirrors how the sensor acquires them: whole
// metric groups are registered when the session opens and are either accepted or
// not, so a half-filled block is not a state that can occur and the fields
// inside need no presence of their own. The exceptions are marked with pointers,
// and each says why it is one.

// CPUSplit divides each frame's CPU time into the work the game did and the
// time it spent waiting for something else — which is the whole question when a
// frame runs long. Paired with GPUSplit it decides the verdict: CPU busy high
// with GPU wait high is a CPU-bound frame, and the mirror is a GPU-bound one.
type CPUSplit struct {
	BusyAvg float64 `json:"busy_avg"`
	BusyP95 float64 `json:"busy_p95"`
	WaitAvg float64 `json:"wait_avg"`
	WaitP95 float64 `json:"wait_p95"`
}

// GPUSplit is the frame's GPU side, from the queue in front of it to the
// present that ends it. It is scoped to the tracked process — these come from
// the frame events, not from adapter telemetry — so a busy figure here is this
// game's work and not the card's total load. GPUTel is the other half of that
// comparison.
type GPUSplit struct {
	LatencyAvg       float64 `json:"latency_avg"` // frame start → GPU work start
	TimeAvg          float64 `json:"time_avg"`    // GPU total duration per frame
	TimeP95          float64 `json:"time_p95"`
	BusyAvg          float64 `json:"busy_avg"` // GPU active time per frame
	BusyP95          float64 `json:"busy_p95"`
	WaitAvg          float64 `json:"wait_avg"`
	InPresentAvg     float64 `json:"in_present_avg"`     // blocked inside the Present call
	RenderLatencyAvg float64 `json:"render_latency_avg"` // Present → GPU completion
}

// Latency is how long the second's frames took to become visible, and how far
// the game's own pacing drifted from what the screen showed.
//
// DisplayAvg is an estimate, and how good an estimate depends on how the frames
// reached the screen — an independent flip is measured much more tightly than a
// composed copy. Present.Mode is what says which, so the two are read together
// and a display latency shown without its present mode is a number with an
// unstated error bar.
type Latency struct {
	DisplayAvg float64 `json:"display_avg"`  // frame start → on screen (estimate; trust level per present mode)
	AnimErrAvg float64 `json:"anim_err_avg"` // |animation error| — source is signed, absolute value recorded
	AnimErrP95 float64 `json:"anim_err_p95"`
}

// GPUTel is whole-GPU telemetry polled once a second, not derived from the
// frame stream. It is deliberately the ADAPTER's figures: a card at 100% while
// the tracked game's own GPUSplit shows it idling is the signature of something
// else on the machine taking the card, and that is a conclusion neither number
// reaches alone.
//
// The inner fields are pointers, breaking the group-atomic rule above, because
// which telemetry a driver publishes varies by vendor and by metric. A card that
// reports utilization but not memory is an ordinary card, not a broken read.
//
// It lives on HostSample and nowhere else. It describes the card every process
// on the machine shares, so filing it under whichever process happened to draw
// the second — which is what it used to do — answered a machine-level question
// only for the seconds one particular game was drawing in.
type GPUTel struct {
	UtilPct *float64 `json:"util_pct,omitempty"` // whole-GPU utilization 0-100 (NOT this process)
	MemUsed *uint64  `json:"mem_used,omitempty"` // whole-GPU dedicated memory used, bytes
	MemSize *uint64  `json:"mem_size,omitempty"` // dedicated memory capacity, bytes
	// The clocks the card is actually running at, in MHz. Two of them because
	// they throttle for different reasons and independently: the core drops on
	// power or thermal limits, while memory holds its clock through most of that
	// and drops on its own. A frame rate that fell while the core clock fell with
	// it is a card that ran out of headroom; one that fell while both clocks
	// stayed up is not, and that is the fork these two decide.
	//
	// Independently nullable like the three above and for the same reason: which
	// figures a driver publishes varies by vendor and by metric.
	CoreMHz *float64 `json:"core_mhz,omitempty"`
	MemMHz  *float64 `json:"mem_mhz,omitempty"`
}

// ProcVRAM is the game process's own dedicated video memory, read once a second
// per adapter and process. It answers what GPUTel.MemUsed cannot: a full card
// says nothing about whether this game is the one filling it.
//
// Budget is a pointer because the OS does not always expose a per-process
// budget, and Used without it is still worth recording — the level is the
// measurement, the budget is the context.
type ProcVRAM struct {
	Used   uint64  `json:"used"`             // bytes committed by the game process
	Budget *uint64 `json:"budget,omitempty"` // OS budget for the process, bytes; nil when the source can't provide it
}

// Sample is one second of presentation data, and the payload both the sensor's
// sec line and the agent's uploaded bucket carry. It is emitted only for seconds
// that contained frames: an idle second produces no sample at all, because
// "nothing was rendering" and "rendering happened at zero" are different facts
// and only one of them can be plotted honestly.
//
// A frameless second is not silent, though — it produces a GapSec saying which
// kind of silence it was, and a HostSample saying what the machine was doing
// through it. The rule this comment states is about the frame data specifically,
// not about the second.
type Sample struct {
	Frames  Frames     `json:"frames"`
	FT      FrameTimes `json:"ft"`
	Hist    Histogram  `json:"ft_hist"`
	DispFT  *DispFT    `json:"disp_ft,omitempty"`
	Present *Present   `json:"present,omitempty"`
	Stutter *Stutter   `json:"stutter,omitempty"`
	ProcRes *ProcRes   `json:"proc_res,omitempty"`
	// The diag blocks. Absent on a base-tier run, and absent on a diag run's
	// seconds after QualityDiagDegraded appears for the polled ones.
	CPUSplit *CPUSplit `json:"cpu_split,omitempty"`
	GPUSplit *GPUSplit `json:"gpu_split,omitempty"`
	Latency  *Latency  `json:"lat,omitempty"`
	ProcVRAM *ProcVRAM `json:"proc_vram,omitempty"`
	Quality  []string  `json:"quality,omitempty"`
}

// Sec is the per-second line on the sensor's stdout: a Sample plus the process
// it describes and the moment the second closed.
type Sec struct {
	Type   string    `json:"type"`
	TS     time.Time `json:"ts"`
	PID    int       `json:"pid"`
	Proc   string    `json:"proc"`
	Sample           // inlined: a sec line is a sample, not a wrapper around one
}

// Run is one continuous stretch of a single game presenting frames, from the
// first second captured to the last. It is the unit a player recognizes — a
// session, a match, an evening — and the key every bucket hangs from.
//
// A run survives things that are not a new game: the process may change PID when
// a launcher hands off, and the window title may change as the player moves
// between menu and match. It ends when presentation stops, when a different
// process takes over, or when the sensor does.
type Run struct {
	ID    string `json:"id"`
	Proc  string `json:"proc"`
	Title string `json:"title,omitempty"`
	// ProfileID is the run's game, copied from the status line that opened it.
	// Absent means the process matched no profile — an "other process" run, which
	// only exists under ModeAll. It is stamped rather than resolved at read time
	// so a later profile edit cannot retroactively rewrite what a past run was.
	ProfileID string `json:"profile_id,omitempty"`
	// StartedAt and LastSeenAt bound the captured seconds. EndedAt is set only
	// once the run is known to be over, so an in-progress run is distinguishable
	// from one whose ending was never observed because the agent stopped.
	StartedAt  time.Time  `json:"started_at"`
	LastSeenAt time.Time  `json:"last_seen_at"`
	EndedAt    *time.Time `json:"ended_at,omitempty"`
	// Source and Caps record how the run was measured. Two runs of the same game
	// are only comparable when these agree, and without them a later reader
	// cannot tell a game that stopped dropping frames from a capture that stopped
	// being able to see drops.
	Source string   `json:"source,omitempty"`
	Caps   []string `json:"caps,omitempty"`
}

// Bucket is one second of a run as uploaded to the server: the sensor's Sample,
// addressed. (RunID, TS) is its identity, so a replayed or retried upload
// overwrites nothing and duplicates nothing.
type Bucket struct {
	RunID  string    `json:"run_id"`
	TS     time.Time `json:"ts"`
	Sample           // inlined for the same reason as in Sec
}

// HostCPU is the machine's processor load for one second, differenced from the
// per-core counters.
//
// The two figures come from one read and one differencing pass, so the block is
// group-atomic in the sense the diag blocks are: if the counters answered, both
// numbers exist, and if they did not, neither does. That is why the fields are
// plain floats and the presence lives on the block.
//
// They are BOTH here because either alone misleads. A single-threaded game pins
// one core at 100% while a sixteen-thread machine reports 6% busy, so TotalPct
// on its own says the machine is idle while the game is starved, and BusiestPct
// on its own says the machine is saturated while fifteen cores sit free. The GAP
// between them is the finding, and a reader can only see a gap that is reported
// as a pair.
//
// Two zeros is a genuinely idle machine and a real measurement. Only an absent
// block means the counters could not be read.
type HostCPU struct {
	TotalPct   float64 `json:"total_pct"`   // busy share of every logical core, 0-100
	BusiestPct float64 `json:"busiest_pct"` // busiest single logical core, 0-100
}

// HostCPUClock is the processor's clock for one second, in MHz.
//
// Separate from HostCPU rather than a field on it, because they come from
// different sources that fail independently: the busy percentages are
// differenced from kernel counters, and this pairs a performance counter with a
// power-management reading. Folding it in would break HostCPU's group-atomic
// rule — the one that lets its two figures be plain floats — for a value that
// can be absent on its own.
//
// CurrentMHz is the HIGHEST clock any logical core is running at, not a mean.
// Modern processors boost a small number of cores well past the all-core clock,
// and the game's own thread is very often one of them; averaging that away
// reports a processor idling at its base clock while the thread that matters is
// at 5 GHz. It is the same argument HostCPU.BusiestPct makes about utilization,
// and the two are read together.
//
// MaxMHz travels with it every second for the reason HostMem.Total does: it is
// what makes the current figure readable. 3.2 GHz is a processor coasting on one
// machine and one pinned at its ceiling on another, and nothing else in the
// record says which. It is the NOMINAL maximum rather than the boost ceiling, so
// CurrentMHz above it is ordinary — that is what boost is.
//
// Group-atomic: one reading produces both, so they arrive and vanish together.
type HostCPUClock struct {
	CurrentMHz float64 `json:"current_mhz"` // the fastest logical core right now
	MaxMHz     float64 `json:"max_mhz"`     // the processor's nominal maximum
}

// HostMem is the machine's physical memory for one second.
//
// Total travels with Used every second rather than being stated once per run,
// because it is what makes Used readable: 12 GB in use is comfortable on a 32 GB
// machine and terminal on a 16 GB one, and a reader looking at a stored second
// months later has no other way to learn which. It costs eight bytes a second
// and removes a whole class of misreading.
//
// Group-atomic for the same reason HostCPU is: one call returns both.
type HostMem struct {
	Used  uint64 `json:"used"`  // bytes of physical memory in use
	Total uint64 `json:"total"` // bytes of physical memory installed
}

// HostSample is one second of machine-level readings.
//
// Every block is optional and independent, because their sources fail apart: a
// machine whose driver publishes no adapter telemetry still reports its CPU, and
// a per-core read that comes back malformed does not stop memory being a level
// anyone can ask for. A sample in which every block is absent is not recorded at
// all unless Quality explains why — an empty row that says nothing is worse than
// no row, because a reader would have to treat it as evidence of something.
//
// # Why this is not gated on the diag tier
//
// A tier is a property of a game profile, and this is a property of the machine.
// One second can hold a base-tier process and a diag-tier one at once, so there
// is no per-second answer to "which tier is this machine second", and picking
// one would make the same reading appear and disappear with whichever window
// happened to draw. The consequence is deliberate and worth stating plainly: a
// base-tier run's detail view now shows whole-card and whole-machine curves.
// They describe the box the game ran on, which is not something the tier was
// ever choosing between.
//
// The GPU block alone is gated on game.gpu.read (and on the adapter publishing
// telemetry at all). CPU and memory need no graphics permission: the busiest
// core is a fact about the processor, and so is the rest of this.
type HostSample struct {
	CPU      *HostCPU      `json:"cpu,omitempty"`
	CPUClock *HostCPUClock `json:"cpu_clock,omitempty"`
	Mem      *HostMem      `json:"mem,omitempty"`
	GPU      *GPUTel       `json:"gpu,omitempty"`
	Quality  []string      `json:"quality,omitempty"`
}

// Empty reports whether a sample carries nothing at all — no reading and no
// explanation for their absence.
//
// Such a sample must not be emitted or stored. An all-NULL row asserts "this
// second was covered and nothing was readable", which is a claim that has to be
// earned by a quality flag; without one, a reader has to treat the row as
// evidence of something, and there is nothing behind it.
func (h HostSample) Empty() bool {
	return h.CPU == nil && h.CPUClock == nil && h.Mem == nil && h.GPU == nil && len(h.Quality) == 0
}

// HostSec is the per-second machine line on the sensor's stdout.
type HostSec struct {
	Type       string    `json:"type"` // TypeHost
	TS         time.Time `json:"ts"`
	HostSample           // inlined, for the reason Sec inlines Sample
}

// HostSecond is one machine second as uploaded to the server: the sensor's
// HostSample, addressed by time alone.
//
// There is no run id and no process id on purpose. This stream is keyed by
// (agent, second) on the server, so two runs overlapping the same second read
// the same row rather than each holding a private copy of one machine's load —
// and a run deleted from the console does not take the machine's history with
// it. A run detail view reads the window [started_at, ended_at] out of it.
type HostSecond struct {
	TS         time.Time `json:"ts"`
	HostSample           // inlined for the same reason as in HostSec
}

// GapSec is the per-second line for a second that held no frames, and why.
//
// A per-second line rather than an interval the sensor closes, because the
// interval has to hang off a RUN and the sensor knows nothing about runs — run
// ids are minted by the agent, which is also the only party that knows a session
// parked at second thirty is still the same session at second forty. An interval
// the sensor closed would have to be re-attributed on arrival anyway, so the
// split is not extra work; it is the work, in the only place that can do it.
//
// PID and Proc name the process that most recently DREW, not the tracker's root:
// they are what the agent matches its sessions on, and for a launcher pair or a
// Chromium window the two are different processes.
type GapSec struct {
	Type   string    `json:"type"` // TypeGap
	TS     time.Time `json:"ts"`   // the moment the frameless second CLOSED
	PID    int       `json:"pid"`
	Proc   string    `json:"proc"`
	Reason string    `json:"reason"` // GapBackground | GapNoFrames
}

// Gap is one continuous stretch of a run that produced no frames, as uploaded.
//
// Re-sent whenever it grows, so the server upserts by id — the same shape as Run
// and for the same reason: a stretch that is still going has to be visible
// before it ends, and an at-least-once uploader must be able to redeliver a
// stale copy without rewinding the current one.
//
// StartedAt is the moment the first frameless second BEGAN and EndedAt the
// moment the last one closed, so the interval spans the same axis a bucket's TS
// sits on. A single frameless second is StartedAt = EndedAt − 1s, matching how a
// bucket's point at TS describes the second ending there.
//
// EndedAt is never absent. An interval still accumulating reports the last
// second it has seen and reports a later one next time; a nullable end would add
// a state ("open") that no reader would render differently from "ends here for
// now", while costing every reader a branch.
//
// A gap may end AFTER its run does, and that must not be clipped. A run ends at
// its last frame; a player who minimized the game and never came back leaves
// fifty minutes of silence after it, and "did they stop playing or just alt-tab"
// is exactly the question this record exists to answer.
type Gap struct {
	ID        string    `json:"id"`
	RunID     string    `json:"run_id"`
	Reason    string    `json:"reason"` // GapBackground | GapNoFrames
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at"`
}
