// Package enroll defines the agent enrollment handshake (architecture §11).
// The agent generates an ed25519 keypair, proves possession by signing a nonce,
// and presents a one-time enrollment token. The server verifies both, enforces
// the agent quota, and returns a bearer token used for subsequent telemetry.
package enroll

import (
	"time"

	"github.com/nettact/protocol/capability"
)

// EnrollRequest is posted by the agent to /api/v1/enroll on first run.
type EnrollRequest struct {
	SchemaVersion   int                     `json:"schema_version"`
	EnrollmentToken string                  `json:"enrollment_token"`
	PublicKey       []byte                  `json:"public_key"` // ed25519 public key (32 bytes)
	Nonce           string                  `json:"nonce"`      // random, signed to prove key possession
	Signature       []byte                  `json:"signature"`  // ed25519 signature over Nonce bytes
	Hostname        string                  `json:"hostname"`
	Platform        string                  `json:"platform"`
	AgentVersion    string                  `json:"agent_version"`
	Capabilities    []capability.Capability `json:"capabilities"`
}

// EnrollResponse is returned once on successful enrollment. AgentToken is shown
// exactly once; the server stores only its hash.
type EnrollResponse struct {
	AgentID       string    `json:"agent_id"`
	SiteID        string    `json:"site_id"`
	AgentToken    string    `json:"agent_token"`
	ServerTime    time.Time `json:"server_time"`
	ConfigVersion int       `json:"config_version"`
}
