package klf200

import (
	"context"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// capturedCall records one request that passed through the fake APICall seam.
type capturedCall struct {
	request protocol.Frame
}

// fakeAPI is a test double for the Client.apiCall seam. It captures every
// request frame and, for each call, replays a list of synthetic response frames
// through the handler (in order) until the handler reports completion. This lets
// command / limitation / gateway helpers be exercised without a real gateway:
// the test asserts on the captured request frame (frame-building) and controls
// the confirmation/notification flow the helper waits for.
type fakeAPI struct {
	calls []capturedCall
	// responder returns the frames to feed the handler for the given request.
	// If nil, no frames are fed (the handler is never called) which is useful
	// when the caller only cares about the request frame and the helper treats
	// an empty exchange as success-by-default paths are not taken.
	responder func(request protocol.Frame) []protocol.Frame
	// err, if non-nil, is returned from APICall without invoking the handler.
	err error
}

// install wires the fake into a client's apiCall seam.
func (f *fakeAPI) install(c *Client) {
	c.apiCall = f.call
}

// call is the func plugged into Client.apiCall.
func (f *fakeAPI) call(_ context.Context, request protocol.Frame, handle FrameHandler) error {
	f.calls = append(f.calls, capturedCall{request: request})
	if f.err != nil {
		return f.err
	}
	if f.responder == nil {
		return nil
	}
	for _, frame := range f.responder(request) {
		if handle(frame) {
			return nil
		}
	}
	return nil
}

// last returns the most recently captured request frame.
func (f *fakeAPI) last() protocol.Frame {
	if len(f.calls) == 0 {
		return nil
	}
	return f.calls[len(f.calls)-1].request
}

// newTestClient builds a Client with the fake API installed and no real
// connection. The session-id generator and node collection are real.
func newTestClient(f *fakeAPI) *Client {
	c := NewClient("test-host", "test-password")
	f.install(c)
	return c
}
