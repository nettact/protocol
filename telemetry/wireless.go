package telemetry

import "time"

// This file defines the authoritative per-collection-round view of the agent's
// network interfaces and the current Wi-Fi status of each wireless adapter
// (architecture §4 wireless layer). It replaces the old per-interface inventory
// delta: the agent sends one full InterfaceSnapshot every round, so the server
// reconciles the complete set (including zero interfaces) by construction — no
// process-local previous-set, no removal deltas, no restart drift.
//
// Categorical Wi-Fi status (SSID, band, channel, connection state) lives here as
// current-state, NOT as metric labels (labels are not persisted). Numeric Wi-Fi
// history rides the ordinary metric time series (see metric.go wifi.* kinds).

// WiFiLinkState is the per-adapter connection verdict. Open string enum (project
// style) so unknown values decode rather than fail.
type WiFiLinkState string

const (
	// WiFiConnected: the adapter is associated with a network. SSID/band/channel
	// carry the current connection; numeric link details may still be per-field
	// absent when the driver/OS does not expose them.
	WiFiConnected WiFiLinkState = "connected"
	// WiFiDisconnected: the adapter exists and was read fine but is not associated
	// (sleep, airplane mode, radio off, disabled, transient drop). All categorical
	// fields are empty by construction — this is how stale details are cleared.
	WiFiDisconnected WiFiLinkState = "disconnected"
	// WiFiUnreadable: this specific adapter's status could not be read (driver or
	// permission error) even though the hardware is present. Categorical fields
	// empty; the collection as a whole may still be "ok".
	WiFiUnreadable WiFiLinkState = "unreadable"
)

// WiFiReason qualifies an unreadable state, or a connected adapter whose SSID was
// specifically withheld by OS privacy policy (macOS location permission).
type WiFiReason string

const (
	WiFiReasonPermission WiFiReason = "permission"
	WiFiReasonDriver     WiFiReason = "driver"
)

// WiFiBand is the operating band of the current connection. Empty when unknown
// or disconnected.
type WiFiBand string

const (
	WiFiBand24 WiFiBand = "2.4"
	WiFiBand5  WiFiBand = "5"
	WiFiBand6  WiFiBand = "6"
)

// WiFiCollectionState is the collection-level Wi-Fi verdict for one round. It
// distinguishes "no adapter" from "cannot read the Wi-Fi subsystem" without
// synthesizing fake adapters:
//   - WiFiCollectionOK with zero wireless interface rows ⇒ genuinely no adapter.
//   - WiFiCollectionUnreadable + WiFiReason ⇒ the whole subsystem failed (dlopen
//     / WlanOpenHandle / nl80211 EPERM at the family or enumeration level).
type WiFiCollectionState string

const (
	WiFiCollectionOK         WiFiCollectionState = "ok"
	WiFiCollectionUnreadable WiFiCollectionState = "unreadable"
)

// WiFiInfo is one wireless adapter's current connection status. Populated only
// for wireless interface rows (InterfaceState.WiFi != nil). SSID/Band/Channel are
// set only when State == WiFiConnected; a disconnected or unreadable sample
// carries them empty by construction.
type WiFiInfo struct {
	State   WiFiLinkState `json:"state"`
	Reason  WiFiReason    `json:"reason,omitempty"`
	SSID    string        `json:"ssid,omitempty"`
	Band    WiFiBand      `json:"band,omitempty"`
	Channel int           `json:"channel,omitempty"`
}

// InterfaceState is one network interface in a snapshot. WiFi is nil on wired
// rows. IsWireless marks known Wi-Fi hardware even when its status is unreadable
// (so "unreadable" wireless hardware never masquerades as a wired interface).
type InterfaceState struct {
	Name       string    `json:"name"`
	Addrs      []string  `json:"addrs,omitempty"`
	Gateway    string    `json:"gateway,omitempty"`
	DNS        []string  `json:"dns,omitempty"`
	Up         bool      `json:"up"`
	IsWireless bool      `json:"is_wireless,omitempty"`
	WiFi       *WiFiInfo `json:"wifi,omitempty"`
}

// InterfaceSnapshot is the authoritative full set of the agent's network
// interfaces for one collection round, plus the collection-level Wi-Fi verdict.
// The server replaces the agent's interface rows with exactly this set (a
// zero-interface snapshot clears them all). Freshness is always keyed to
// SampledAt (collector wall-clock UTC), never ingest time — a WAL replay can
// land hours later.
//
// Interfaces is intentionally not omitempty: a zero-interface round must still
// transmit the (empty) set so the server clears stale rows.
type InterfaceSnapshot struct {
	SampledAt    time.Time           `json:"sampled_at"`
	WiFiState    WiFiCollectionState `json:"wifi_state"`
	WiFiReason   WiFiReason          `json:"wifi_reason,omitempty"`
	DefaultRoute *SnapshotRoute      `json:"default_route,omitempty"`
	Interfaces   []InterfaceState    `json:"interfaces"`
}
