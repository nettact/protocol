// Package telemetry defines the batched data packet an agent uploads to the
// server, matching the shape in the architecture doc §5.1.
package telemetry

import "time"

// Packet is one batched, idempotent telemetry upload. The server dedups on
// (AgentID, Sequence); see architecture §3.3 / §5.1.
type Packet struct {
	SchemaVersion         int             `json:"schema_version"`
	AgentID               string          `json:"agent_id"`
	SiteID                string          `json:"site_id"`
	Sequence              uint64          `json:"sequence"`
	SentAt                time.Time       `json:"sent_at"`
	Metrics               []Metric        `json:"metrics,omitempty"`
	Events                []Event         `json:"events,omitempty"`
	InventoryDelta        []InventoryItem `json:"inventory_delta,omitempty"`
	ReportedConfigVersion int             `json:"reported_config_version"`

	// HostSnapshot carries an ephemeral, on-demand process/connection view when
	// the server requested one via config.SnapshotRequest. It is never stored;
	// the server keeps only the latest per agent in memory. Nil in normal packets.
	HostSnapshot *HostSnapshot `json:"host_snapshot,omitempty"`
}
