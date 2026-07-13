package klf200

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// DefaultHeartbeatInterval is the default interval between heartbeat pulses,
// matching pyvlx Heartbeat's default timeout_in_seconds of 60.
const DefaultHeartbeatInterval = 60 * time.Second

// Heartbeat sends periodic GW_GET_STATE_REQ frames to keep the gateway
// session alive and, for any Blind nodes in the collection, a
// GW_STATUS_REQUEST_NTF to refresh the FP3 / orientation value (which House
// Monitor delivers incorrectly). Ported from heartbeat.py.
//
// Construct with NewHeartbeat; start the background goroutine with Start; stop
// it with Stop. Stop blocks until the goroutine has exited.
type Heartbeat struct {
	client   *Client
	interval time.Duration

	mu      sync.Mutex
	stopCh  chan struct{}
	doneCh  chan struct{}
	running bool

	// FailureCh receives non-nil errors when a pulse fails. The channel is
	// buffered (capacity 1); if the consumer is slow, old failures are
	// silently dropped so the heartbeat loop never blocks. Callers that do
	// not care about failures may ignore this channel.
	FailureCh chan error
}

// HeartbeatOption configures a Heartbeat at construction.
type HeartbeatOption func(*Heartbeat)

// WithHeartbeatInterval overrides the pulse interval (default 60 s).
func WithHeartbeatInterval(d time.Duration) HeartbeatOption {
	return func(h *Heartbeat) {
		if d > 0 {
			h.interval = d
		}
	}
}

// NewHeartbeat constructs a Heartbeat for client. Call Start to begin pulsing.
// Ported from Heartbeat.__init__ (heartbeat.py).
func NewHeartbeat(client *Client, opts ...HeartbeatOption) *Heartbeat {
	h := &Heartbeat{
		client:    client,
		interval:  DefaultHeartbeatInterval,
		FailureCh: make(chan error, 1),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Start launches the heartbeat goroutine. It is a no-op if already running.
// Ported from Heartbeat.start (heartbeat.py).
func (h *Heartbeat) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.running {
		return
	}
	h.stopCh = make(chan struct{})
	h.doneCh = make(chan struct{})
	h.running = true
	go h.loop()
}

// Stop signals the heartbeat goroutine to stop and blocks until it exits.
// It is a no-op if not running. Ported from Heartbeat.stop (heartbeat.py).
func (h *Heartbeat) Stop() {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return
	}
	close(h.stopCh)
	doneCh := h.doneCh
	h.mu.Unlock()
	<-doneCh
}

// loop is the heartbeat goroutine body: wait interval, then pulse; repeat until
// stopped. Ported from Heartbeat.loop (heartbeat.py).
func (h *Heartbeat) loop() {
	defer func() {
		h.mu.Lock()
		h.running = false
		close(h.doneCh)
		h.mu.Unlock()
	}()

	timer := time.NewTimer(h.interval)
	defer timer.Stop()

	for {
		select {
		case <-h.stopCh:
			return
		case <-timer.C:
			if err := h.pulse(); err != nil {
				// Surface heartbeat failure, dropping old unread errors.
				select {
				case h.FailureCh <- err:
				default:
					// Consumer not reading; discard to prevent blocking.
				}
			}
			timer.Reset(h.interval)
		}
	}
}

// pulse sends GW_GET_STATE_REQ to keep the session alive, then for each Blind
// node sends a GW_STATUS_REQUEST_NTF to refresh the FP3 orientation parameter
// (House Monitor delivers wrong values for FP3). Ported from Heartbeat.pulse
// (heartbeat.py).
func (h *Heartbeat) pulse() error {
	ctx, cancel := context.WithTimeout(context.Background(), h.client.Timeout())
	defer cancel()

	if _, err := h.client.GetState(ctx); err != nil {
		return fmt.Errorf("klf200: heartbeat: get state: %w", err)
	}

	for _, node := range h.client.Nodes().All() {
		if _, isBlind := node.(BlindNode); !isBlind {
			continue
		}
		if err := h.statusRequest(ctx, node.NodeID()); err != nil {
			// Best-effort: log-worthy but non-fatal — continue with remaining nodes.
			_ = err
		}
	}
	return nil
}

// statusRequest sends GW_STATUS_REQUEST_REQ for nodeID requesting current-position
// status for all functional parameters, and waits for GW_STATUS_REQUEST_NTF.
// The node_updater will pick up the resulting notification and refresh FP3.
// Ported from StatusRequest in api/status_request.py (called from Heartbeat.pulse).
func (h *Heartbeat) statusRequest(ctx context.Context, nodeID uint16) error {
	sid := h.client.Sessions().NewSessionID()
	req := &protocol.FrameStatusRequestRequest{
		SessionID:  sid,
		NodeIDs:    []uint8{uint8(nodeID)},
		StatusType: protocol.StatusTypeRequestCurrentPosition,
		FPI1:       0xFE, // all functional parameters
		FPI2:       0x00,
	}
	err := h.client.APICall(ctx, req, func(frame protocol.Frame) bool {
		switch f := frame.(type) {
		case *protocol.FrameStatusRequestConfirmation:
			if f.SessionID == sid {
				// Confirmation acknowledged — keep waiting for the NTF.
				return false
			}
		case *protocol.FrameStatusRequestNotification:
			if f.SessionID == sid {
				return true
			}
		}
		return false
	})
	if err != nil {
		return fmt.Errorf("status request node %d: %w", nodeID, err)
	}
	return nil
}
