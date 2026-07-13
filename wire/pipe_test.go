package wire

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func helloFrame(v int) Frame    { return Frame{Hello: &Hello{ReportedConfigVersion: v}} }
func ackFrame(seq uint64) Frame { return Frame{Ack: &Ack{HighestSequence: seq}} }

// TestPipeFIFOBothDirections verifies frames flow both ways in order.
func TestPipeFIFOBothDirections(t *testing.T) {
	a, b := Pipe()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := a.WriteFrame(ctx, helloFrame(i)); err != nil {
			t.Fatalf("a write %d: %v", i, err)
		}
	}
	for i := 0; i < 3; i++ {
		f, err := b.ReadFrame(ctx)
		if err != nil {
			t.Fatalf("b read %d: %v", i, err)
		}
		if f.Hello == nil || f.Hello.ReportedConfigVersion != i {
			t.Fatalf("b read %d: got %+v", i, f)
		}
	}

	for i := 0; i < 3; i++ {
		if err := b.WriteFrame(ctx, ackFrame(uint64(i))); err != nil {
			t.Fatalf("b write %d: %v", i, err)
		}
	}
	for i := 0; i < 3; i++ {
		f, err := a.ReadFrame(ctx)
		if err != nil {
			t.Fatalf("a read %d: %v", i, err)
		}
		if f.Ack == nil || f.Ack.HighestSequence != uint64(i) {
			t.Fatalf("a read %d: got %+v", i, f)
		}
	}
}

// TestPipeCloseSurfacesOnPeer checks the code/reason reaches the peer on both
// ReadFrame and WriteFrame, through %w wrapping.
func TestPipeCloseSurfacesOnPeer(t *testing.T) {
	a, b := Pipe()
	ctx := context.Background()

	if err := a.Close(CloseRevoked, "gone"); err != nil {
		t.Fatalf("close: %v", err)
	}

	_, rerr := b.ReadFrame(ctx)
	if got := CloseStatus(rerr); got != CloseRevoked {
		t.Fatalf("read after close: CloseStatus=%d err=%v", got, rerr)
	}
	werr := b.WriteFrame(ctx, helloFrame(0))
	if got := CloseStatus(werr); got != CloseRevoked {
		t.Fatalf("write after close: CloseStatus=%d err=%v", got, werr)
	}
	// The closing end sees the close too.
	if _, err := a.ReadFrame(ctx); CloseStatus(err) != CloseRevoked {
		t.Fatalf("closer read: %v", err)
	}

	// Reason must survive.
	var ce *CloseError
	if !errors.As(rerr, &ce) || ce.Reason != "gone" {
		t.Fatalf("reason lost: %v", rerr)
	}
	// Wrapping must remain transparent to CloseStatus.
	wrapped := fmt.Errorf("session: %w", rerr)
	if CloseStatus(wrapped) != CloseRevoked {
		t.Fatalf("CloseStatus through wrap failed: %v", wrapped)
	}
}

// TestPipeDrainBeforeClose ensures frames written before Close are still read
// before the close error surfaces.
func TestPipeDrainBeforeClose(t *testing.T) {
	a, b := Pipe()
	ctx := context.Background()

	if err := a.WriteFrame(ctx, ackFrame(7)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := a.Close(CloseNormalClosure, "bye"); err != nil {
		t.Fatalf("close: %v", err)
	}

	f, err := b.ReadFrame(ctx)
	if err != nil {
		t.Fatalf("expected buffered frame, got err: %v", err)
	}
	if f.Ack == nil || f.Ack.HighestSequence != 7 {
		t.Fatalf("wrong frame: %+v", f)
	}
	if _, err := b.ReadFrame(ctx); CloseStatus(err) != CloseNormalClosure {
		t.Fatalf("expected close after drain, got: %v", err)
	}
}

// TestPipeCtxCancel verifies a blocked read/write unblocks on ctx cancel.
func TestPipeCtxCancel(t *testing.T) {
	a, _ := Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := a.ReadFrame(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read: expected deadline, got %v", err)
	}

	// Fill the send buffer, then a further write blocks and must honor ctx.
	bg := context.Background()
	for i := 0; i < pipeBuffer; i++ {
		if err := a.WriteFrame(bg, helloFrame(i)); err != nil {
			t.Fatalf("fill %d: %v", i, err)
		}
	}
	ctx2, cancel2 := context.WithTimeout(bg, 20*time.Millisecond)
	defer cancel2()
	if err := a.WriteFrame(ctx2, helloFrame(99)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("write: expected deadline, got %v", err)
	}
}

// TestPipeDoubleCloseNoop confirms the first Close wins and later ones are safe.
func TestPipeDoubleCloseNoop(t *testing.T) {
	a, b := Pipe()
	if err := a.Close(CloseSuperseded, "first"); err != nil {
		t.Fatalf("close 1: %v", err)
	}
	if err := b.Close(CloseRevoked, "second"); err != nil {
		t.Fatalf("close 2: %v", err)
	}
	if got := CloseStatus(mustReadErr(t, a)); got != CloseSuperseded {
		t.Fatalf("first close should win, got %d", got)
	}
}

// TestPipePing checks Ping open vs closed.
func TestPipePing(t *testing.T) {
	a, b := Pipe()
	if err := a.Ping(context.Background()); err != nil {
		t.Fatalf("ping open: %v", err)
	}
	_ = b.Close(CloseGoingAway, "")
	if got := CloseStatus(a.Ping(context.Background())); got != CloseGoingAway {
		t.Fatalf("ping closed: %d", got)
	}
}

// TestPipeRejectsBadFrame verifies WriteFrame enforces the exactly-one-variant
// invariant.
func TestPipeRejectsBadFrame(t *testing.T) {
	a, _ := Pipe()
	ctx := context.Background()
	if err := a.WriteFrame(ctx, Frame{}); !errors.Is(err, ErrFrameVariant) {
		t.Fatalf("zero-variant: %v", err)
	}
	two := Frame{Hello: &Hello{}, Ack: &Ack{}}
	if err := a.WriteFrame(ctx, two); !errors.Is(err, ErrFrameVariant) {
		t.Fatalf("two-variant: %v", err)
	}
}

func mustReadErr(t *testing.T, c Conn) error {
	t.Helper()
	_, err := c.ReadFrame(context.Background())
	if err == nil {
		t.Fatal("expected read error")
	}
	return err
}
