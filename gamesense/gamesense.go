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
const ProtoVersion = 3

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
	// CapGPUTel: whole-GPU telemetry is polled once a second. It is the adapter,
	// not the process — the two together are what separates "this game is asking
	// too much of the card" from "something else on the machine is".
	CapGPUTel = "gpu_tel"
	// CapProcVRAM: the game process's own dedicated video memory is read once a
	// second. Distinct from the whole-card figure in CapGPUTel for the same
	// reason CapProcCPU is distinct from the machine's CPU usage.
	CapProcVRAM = "proc_vram"
	// CapBusiestCore: per-core utilization is read once a second and the busiest
	// logical core reported. A game bound to one thread pins a single core while
	// the machine-wide average stays comfortable, and the average is what hides
	// it.
	CapBusiestCore = "busiest_core"
)

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
// GPUTel and ProcVRAM are the other kind: single readings taken once at the
// second boundary, in the units their comments name.
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
type GPUTel struct {
	UtilPct *float64 `json:"util_pct,omitempty"` // whole-GPU utilization 0-100 (NOT this process)
	MemUsed *uint64  `json:"mem_used,omitempty"` // whole-GPU dedicated memory used, bytes
	MemSize *uint64  `json:"mem_size,omitempty"` // dedicated memory capacity, bytes
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
	GPUTel   *GPUTel   `json:"gpu_tel,omitempty"`
	ProcVRAM *ProcVRAM `json:"proc_vram,omitempty"`
	// BusiestCorePct is the busiest logical core, % 0-100. It stands alone
	// rather than joining ProcRes because it describes the machine, not the
	// process: a single-threaded game pins one core while ProcRes.CPUPct — a
	// share of all cores — reads low, and that gap is the finding.
	BusiestCorePct *float64 `json:"busiest_core_pct,omitempty"`
	Quality        []string `json:"quality,omitempty"`
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
