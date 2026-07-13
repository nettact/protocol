package wire

import (
	"context"
	"errors"
	"fmt"
)

// CloseCode mirrors the RFC 6455 status codes plus the NetTact 4000-range
// application codes. It replaces the transport-specific status code (previously
// coder/websocket's StatusCode) so both the WebSocket adapters and the
// in-memory Pipe classify session outcomes through the same vocabulary.
type CloseCode int

const (
	// CloseNormalClosure is a clean shutdown: the agent sent it on Ctrl+C so the
	// server marks it offline immediately instead of waiting out a timeout.
	CloseNormalClosure CloseCode = 1000
	// CloseGoingAway is the server closing every session for graceful shutdown.
	CloseGoingAway CloseCode = 1001
	// ClosePolicyViolation closes a session that violated a runtime policy, such
	// as a consumer too slow to drain its outbound queue.
	ClosePolicyViolation CloseCode = 1008
	// CloseInternalError signals an abnormal session end (write failure, decode
	// trouble) — not a clean close, so the peer knows it was not graceful.
	CloseInternalError CloseCode = 1011

	// CloseSuperseded closes an old session when the same agent connects again;
	// the replaced side must NOT reconnect (it would kick the new one in a loop).
	CloseSuperseded CloseCode = 4000
	// CloseUnsupportedSchema rejects a Hello whose schema version the server does
	// not understand; reconnecting won't help until one side is upgraded.
	CloseUnsupportedSchema CloseCode = 4001
	// CloseUnsupportedSubprotocol rejects a client that offered neither
	// nettact.v1 subprotocol, so the Frame encoding was never agreed on.
	CloseUnsupportedSubprotocol CloseCode = 4002
	// CloseProtocolError rejects a frame the protocol does not allow at that
	// point (non-Hello first frame, agent-bound frame sent by the agent, or
	// undecodable bytes).
	CloseProtocolError CloseCode = 4003
	// CloseRevoked evicts the session of an agent being deleted: its credential
	// is about to stop authenticating, so the agent must re-enroll rather than
	// reconnect.
	CloseRevoked CloseCode = 4004
)

// CloseError is returned by Conn.ReadFrame/WriteFrame/Ping once the link has
// been closed with a code (by either end). Its Code drives the supervisor's
// terminal-outcome decision, extracted via CloseStatus through %w wrapping.
type CloseError struct {
	Code   CloseCode
	Reason string
}

func (e *CloseError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("connection closed with code %d", e.Code)
	}
	return fmt.Sprintf("connection closed with code %d: %s", e.Code, e.Reason)
}

// CloseStatus extracts the close code from err (seeing through fmt.Errorf %w
// wrapping), or -1 if err carries no CloseError. It is the transport-agnostic
// replacement for websocket.CloseStatus at every classification site outside
// the WebSocket adapters.
func CloseStatus(err error) CloseCode {
	var ce *CloseError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return -1
}

// Conn is one end of an agent <-> server frame link. Frames cross it already
// decoded into wire.Frame values; any serialization is the implementation's
// concern (the WebSocket adapters marshal per the negotiated subprotocol; the
// in-memory Pipe passes values through untouched). Implementations:
//   - agent/internal/conn (client-side WebSocket adapter)
//   - server-core/agentws (server-side WebSocket adapter)
//   - Pipe below (in-process desktop link)
type Conn interface {
	// ReadFrame returns the next frame, or a *CloseError once the link closed.
	ReadFrame(ctx context.Context) (Frame, error)
	// WriteFrame sends one frame; it returns a *CloseError if the link closed.
	WriteFrame(ctx context.Context, f Frame) error
	// Ping verifies liveness. It succeeds while the link is open.
	Ping(ctx context.Context) error
	// Close ends the link with a code/reason. The first Close wins; later calls
	// are no-ops.
	Close(code CloseCode, reason string) error
}

// Dialer establishes one authenticated frame link for the given bearer token.
// The default is a WebSocket dialer built from the agent's server URL; the
// desktop injects the embedded Lite server's in-process pipe dialer instead.
type Dialer func(ctx context.Context, token string) (Conn, error)
