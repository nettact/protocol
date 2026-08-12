package wire

import "time"

// Schema 8 capability names, declared by the agent in Hello.Capabilities and
// granted (or not) by the server. They are a separate axis from
// PermissionReport: permissions gate which collectors the agent runs, while a
// capability gates which protocol state machines the two peers may enter. A
// receiver must ignore names it does not know and must never send a control
// frame for a capability the peer did not declare.
const CapSequenceFloorV1 = "sequence_floor_v1"

// HasCapability reports whether caps contains name. Duplicate entries are
// tolerated (set semantics); the empty list declares nothing.
func HasCapability(caps []string, name string) bool {
	for _, c := range caps {
		if c == name {
			return true
		}
	}
	return false
}

// SequenceFloor is the server->agent pre-claim barrier (schema 8). Before the
// agent may claim its first sequence for this credential/epoch, the server
// pushes the epoch and the highest sequence it has durably accepted, plus a
// session id for diagnostics. The agent must durably fast-forward its
// allocator past SequenceFloor and reply SequenceFloorApplied before sending
// its first Packet; a packet sent before the applied reply is a protocol
// error on both sides.
//
// Why a dedicated frame and not a TelemetryAck field: the ack is strictly
// "exactly one confirmation per in-flight packet" — the agent deletes the
// in-flight claim on receipt. An unsolicited floor carried as an ack would be
// indistinguishable from a per-packet confirmation and could delete a claim
// nothing confirmed. A dedicated frame makes the barrier an explicit session
// phase the server can enforce by its own ordering (no packet admitted before
// the applied reply) instead of by ack bookkeeping.
type SequenceFloor struct {
	EnrollmentEpoch uint64 `json:"enrollment_epoch"`
	SequenceFloor   uint64 `json:"sequence_floor"`
	SessionID       string `json:"session_id"`
}

// SequenceFloorApplied is the agent->server reply to SequenceFloor: the floor
// is durably applied and the WAL drain is open. It is a control response, NOT
// a packet ack — the server must not treat it as telemetry admission.
type SequenceFloorApplied struct {
	EnrollmentEpoch uint64 `json:"enrollment_epoch"`
	SequenceFloor   uint64 `json:"sequence_floor"`
}

// EpochRotationChallenge is the server->agent opener of the controlled
// credential/epoch rotation. The challenge string is a server-generated
// one-time binding of agent, old epoch, expiry and server origin (the exact
// encoding is server-private; the agent treats it as opaque bytes to sign).
// The agent proves possession of the enrolled ed25519 key by signing the
// challenge and answers with EpochRotationRequest.
//
// Reason is an open enum explaining why the server demands a rotation
// (sequence_conflict: the same (agent, epoch, sequence) carried a different
// payload; epoch_mismatch: the Hello presented a stale epoch). Unknown values
// must be tolerated by the agent.
type EpochRotationChallenge struct {
	Challenge string    `json:"challenge"`
	Reason    string    `json:"reason"`
	ExpiresAt time.Time `json:"expires_at"`
}

// EpochRotationRequest is the agent->server possession proof answering a
// challenge. Signature is ed25519 over the challenge bytes with the key the
// agent enrolled with; OldEpoch states the epoch the agent believes it is
// rotating out of. The server must re-validate challenge freshness and the
// signature binding before rotating anything.
type EpochRotationRequest struct {
	Challenge string `json:"challenge"`
	OldEpoch  uint64 `json:"old_epoch"`
	Signature []byte `json:"signature"`
}

// EpochRotationResult status values. Unknown status values on the wire must be
// tolerated and treated by the agent as a denial with reason (open string enum
// convention).
const (
	RotationOK     = "ok"     // new credential + epoch issued; AgentToken/NewEpoch set
	RotationDenied = "denied" // proof rejected or policy refused; terminal for this challenge
	RotationRetry  = "retry"  // server transiently unavailable; agent may retry with a fresh challenge
)

// EpochRotationResult is the server->agent terminal of a rotation. On OK the
// agent must durably persist AgentToken and NewEpoch (and the WAL's epoch
// state) before it switches identity; on denial or retry the old credential
// stays in force. AgentToken is shown exactly once, like enrollment.
type EpochRotationResult struct {
	Status     string `json:"status"`
	NewEpoch   uint64 `json:"new_epoch,omitempty"`
	AgentToken string `json:"agent_token,omitempty"`
	Reason     string `json:"reason,omitempty"`
}

// EpochRotationChallengeRequest is the agent->server opener of an
// agent-initiated rotation: the agent detected it cannot safely proceed on its
// current epoch — the canonical case is an in-flight WAL claim whose sequence
// sits at or below the server's accepted floor, which must never be renumbered
// in place — and asks the server to challenge it. The server answers
// EpochRotationChallenge and the normal rotation flow follows.
type EpochRotationChallengeRequest struct {
	Reason string `json:"reason"`
}
