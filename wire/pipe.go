package wire

import (
	"context"
	"sync"
	"sync/atomic"
)

// pipeBuffer is the per-direction channel depth. It only needs to absorb the
// small burst between the peer's writes and this end's reads; the session
// protocol above the transport (single writer, awaited acks) keeps the actual
// in-flight count tiny.
const pipeBuffer = 8

// Pipe returns two connected in-memory Conn ends for the desktop's bundled
// agent and embedded server. Frames cross by value with no serialization, so
// the sender MUST treat a sent Frame's pointed-to payloads as immutable — both
// sides already read received frames without mutating them. A Close on either
// end delivers the same CloseError to both.
func Pipe() (agentEnd, serverEnd Conn) {
	a2s := make(chan Frame, pipeBuffer)
	s2a := make(chan Frame, pipeBuffer)
	shared := &pipeShared{done: make(chan struct{})}
	agentEnd = &pipeConn{send: a2s, recv: s2a, shared: shared}
	serverEnd = &pipeConn{send: s2a, recv: a2s, shared: shared}
	return agentEnd, serverEnd
}

// pipeShared holds the close state common to both ends: whichever side closes
// first records the CloseError and closes done, and both ends then observe it.
type pipeShared struct {
	once     sync.Once
	done     chan struct{}
	closeErr atomic.Pointer[CloseError]
}

// pipeConn is one end of a Pipe. send/recv are the two directional channels
// (this end's send is the peer's recv); shared is the same on both ends.
type pipeConn struct {
	send   chan<- Frame
	recv   <-chan Frame
	shared *pipeShared
}

func (p *pipeConn) WriteFrame(ctx context.Context, f Frame) error {
	if err := f.Validate(); err != nil {
		return err
	}
	// Check for an already-closed link before blocking on send, so a write to a
	// closed pipe reports the close rather than parking until ctx expires.
	select {
	case <-p.shared.done:
		return p.closeError()
	default:
	}
	select {
	case p.send <- f:
		return nil
	case <-p.shared.done:
		return p.closeError()
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *pipeConn) ReadFrame(ctx context.Context) (Frame, error) {
	// Drain preference: hand back any frame the peer already queued before the
	// close, mirroring the WebSocket ordered close where an ack sent just before
	// the close frame is still delivered. This is the only ordering that matters
	// — the agent awaits its ack before closing.
	select {
	case f := <-p.recv:
		return f, nil
	default:
	}
	select {
	case f := <-p.recv:
		return f, nil
	case <-p.shared.done:
		// A frame may have been queued between the check above and the peer's
		// Close (both channels then ready, and select picks at random). Re-check
		// recv non-blocking so a frame written just before close is delivered
		// before the terminal close error, preserving drain-before-close.
		select {
		case f := <-p.recv:
			return f, nil
		default:
			return Frame{}, p.closeError()
		}
	case <-ctx.Done():
		return Frame{}, ctx.Err()
	}
}

func (p *pipeConn) Ping(ctx context.Context) error {
	select {
	case <-p.shared.done:
		return p.closeError()
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (p *pipeConn) Close(code CloseCode, reason string) error {
	p.shared.once.Do(func() {
		p.shared.closeErr.Store(&CloseError{Code: code, Reason: reason})
		close(p.shared.done)
	})
	return nil
}

// closeError returns the recorded close error. done is closed only after
// closeErr is stored, so any caller that observed done sees a non-nil value.
func (p *pipeConn) closeError() error {
	if e := p.shared.closeErr.Load(); e != nil {
		return e
	}
	return &CloseError{Code: CloseInternalError, Reason: "pipe closed"}
}
