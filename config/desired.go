// Package config defines the DesiredState the server pushes down to an agent
// via the telemetry ack. Monitoring targets are configured centrally in Lite
// and delivered to agents on the agent's own outbound request — the agent
// never listens, and users don't edit agent config files (low friction).
package config

// DesiredState is the monitoring configuration for one agent. The agent applies
// it and echoes ConfigVersion back in reported_config_version so the server can
// tell when it is up to date.
type DesiredState struct {
	ConfigVersion int           `json:"config_version"`
	ProbeTargets  []ProbeTarget `json:"probe_targets"`
	Intervals     Intervals     `json:"intervals"`
}

// ProbeTarget is one monitoring target pushed to the agent.
type ProbeTarget struct {
	Kind   string `json:"kind"`   // "icmp" (M2); "dns" / "http" added in M3
	Target string `json:"target"` // "1.1.1.1", "example.com", "https://…"
	Tier   string `json:"tier"`   // "base" | "regular"
}

// Intervals controls the agent scheduler tiers (seconds).
type Intervals struct {
	BaseSeconds    int `json:"base_seconds"`
	RegularSeconds int `json:"regular_seconds"`
}
