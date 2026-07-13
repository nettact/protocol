// Package telemetry defines the batched data packet an agent uploads to the
// server, matching the shape in the architecture doc §5.1.
package telemetry

import "time"

// Packet is one batched, idempotent telemetry upload. The server dedups on
// (AgentID, Sequence); see architecture §3.3 / §5.1.
type Packet struct {
	SchemaVersion         int                 `json:"schema_version"`
	AgentID               string              `json:"agent_id"`
	SiteID                string              `json:"site_id"`
	Sequence              uint64              `json:"sequence"`
	SentAt                time.Time           `json:"sent_at"`
	Metrics               []Metric            `json:"metrics,omitempty"`
	Events                []Event             `json:"events,omitempty"`
	InventoryDelta        []InventoryItem     `json:"inventory_delta,omitempty"`
	InterfaceSnapshots    []InterfaceSnapshot `json:"interface_snapshots,omitempty"`
	ReportedConfigVersion int                 `json:"reported_config_version"`
}
