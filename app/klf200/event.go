package klf200

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// DefaultTimeoutSeconds is the default request/response timeout, matching
// pyvlx's ApiEvent default of 10 seconds.
const DefaultTimeoutSeconds = 10

// FrameHandler inspects an incoming frame during an API call and reports
// whether the call is complete. It is the Go equivalent of
// ApiEvent.handle_frame: return true when the expected (final) frame has been
// seen, false to keep waiting. Intermediate/notification frames that should be
// accumulated are captured by the handler's own closure (mirroring how pyvlx
// subclasses append to self.notification_frames).
//
// A handler is called sequentially from the connection read loop for the
// duration of one DoAPICall; it does not need to be safe for concurrent use.
type FrameHandler func(frame protocol.Frame) bool

// ApiEvent sends a request frame and waits for the matching confirmation and/or
// notification frame(s), correlating however the supplied handler decides
// (typically by session id). It is modeled on pyvlx's ApiEvent/do_api_call.
//
// Correlation by session id is the handler's responsibility: the handler
// closure compares frame session ids against the id it embedded in the request
// (see CommandSend in pyvlx), exactly as the pyvlx subclasses do.
type ApiEvent struct {
	conn    *Conn
	request protocol.Frame
	handle  FrameHandler
}

// NewApiEvent creates an ApiEvent for a single request/response exchange.
// request is the frame to send; handle decides when the exchange is complete.
func NewApiEvent(conn *Conn, request protocol.Frame, handle FrameHandler) *ApiEvent {
	return &ApiEvent{conn: conn, request: request, handle: handle}
}

// DoAPICall registers the handler, sends the request frame, and blocks until
// the handler reports completion, the context is cancelled/times out, or the
// connection is lost. It always unregisters the handler before returning.
// Ported from ApiEvent.do_api_call (Event/timeout replaced by a Go channel and
// context).
func (e *ApiEvent) DoAPICall(ctx context.Context) error {
	if e.conn == nil {
		return errors.New("klf200: ApiEvent has no connection")
	}
	if e.handle == nil {
		return errors.New("klf200: ApiEvent has no frame handler")
	}

	done := make(chan struct{})
	var doneOnce sync.Once

	// Register before sending so we cannot miss a fast response. The callback
	// runs on the read-loop goroutine; it forwards to the handler and, on
	// completion, signals done exactly once.
	handle := e.conn.RegisterFrameReceivedCB(func(frame protocol.Frame) {
		if e.handle(frame) {
			doneOnce.Do(func() { close(done) })
		}
	})
	defer e.conn.UnregisterFrameReceivedCB(handle)

	if err := e.conn.Write(e.request); err != nil {
		return fmt.Errorf("klf200: send request %s: %w", e.request.Command(), err)
	}

	// Snapshot the connection-lost signal for this attempt.
	lost := e.conn.Lost()

	select {
	case <-done:
		return nil
	case <-lost:
		return errors.New("klf200: connection lost while awaiting response")
	case <-ctx.Done():
		return fmt.Errorf("klf200: timeout awaiting response to %s: %w", e.request.Command(), ctx.Err())
	}
}

// DoAPICall is a convenience wrapper that builds and runs an ApiEvent in one
// call.
func DoAPICall(ctx context.Context, conn *Conn, request protocol.Frame, handle FrameHandler) error {
	return NewApiEvent(conn, request, handle).DoAPICall(ctx)
}
