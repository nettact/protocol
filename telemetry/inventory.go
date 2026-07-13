package telemetry

import "time"

// InventoryItem is a device-level delta (upsert or removal) that the agent sends
// so the server can maintain site device inventory without re-sending the full
// set each time. Interface state is no longer an inventory delta — it travels as
// an authoritative InterfaceSnapshot (see wireless.go).
type InventoryItem struct {
	Kind InventoryKind `json:"kind"`
	Op   DeltaOp       `json:"op"`
	ID   string        `json:"id"` // MAC for a device

	// device fields
	MAC      string    `json:"mac,omitempty"`
	IP       string    `json:"ip,omitempty"`
	Hostname string    `json:"hostname,omitempty"`
	Vendor   string    `json:"vendor,omitempty"`
	LastSeen time.Time `json:"last_seen,omitempty"`
}

type InventoryKind string

const (
	InventoryDevice InventoryKind = "device"
)

type DeltaOp string

const (
	OpUpsert DeltaOp = "upsert"
	OpRemove DeltaOp = "remove"
)
