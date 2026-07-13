package klf200

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// Client is the high-level facade for a KLF200 gateway. It merges pyvlx's PyVLX
// (top-level object, connection wiring, node/scene loading) and Klf200Gateway
// (gateway housekeeping calls such as password_enter, get_state). It owns the
// transport Conn, the session-id generator, and the Nodes collection, and wires
// inbound notifications to the node updater.
//
// A Client is constructed with NewClient and made live with Connect. The
// authentication handshake, live-update dispatch, and gateway housekeeping
// calls are filled in by later phases through the seams exposed here.
type Client struct {
	conn     *Conn
	sessions *SessionIDGenerator
	nodes    *Nodes
	password string
	timeout  time.Duration

	// nodeUpdaters holds the inbound-frame processors that live-update nodes
	// from house-monitor / status notifications. The live-updates author
	// registers a processor via RegisterNodeUpdater; Connect installs a single
	// connection-level callback that fans each frame out to them. Guarded by mu.
	mu           sync.Mutex
	nodeUpdaters []FrameProcessor
	updaterCBID  uint64
	updaterSet   bool

	// apiCall is the seam through which every request/response exchange runs.
	// In production it is the real transport-backed implementation
	// (defaultAPICall); tests replace it with a fake that captures the request
	// frame and feeds synthetic response frames to the handler, so command and
	// limitation frame-building can be verified without hardware.
	apiCall func(ctx context.Context, request protocol.Frame, handle FrameHandler) error
}

// FrameProcessor is invoked for every inbound frame once the client is
// connected. It is the seam the live-updates author (node_updater port) plugs
// into: their processor inspects notification frames and mutates the matching
// node, then calls the node's AfterUpdate. It corresponds to registering
// NodeUpdater.process_frame as a frame-received callback in PyVLX.__init__.
type FrameProcessor func(frame protocol.Frame)

// ClientOption configures a Client at construction. Options keep NewClient's
// signature stable while letting callers override defaults (port, timeout).
type ClientOption func(*Client)

// WithPort overrides the gateway TCP port (default DefaultPort).
func WithPort(port int) ClientOption {
	return func(c *Client) {
		if port != 0 {
			c.conn = NewConn(c.conn.host, port)
		}
	}
}

// WithTimeout overrides the default per-API-call timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *Client) {
		if d > 0 {
			c.timeout = d
		}
	}
}

// NewClient constructs a Client for the given gateway host and password. The
// password is stored and applied during Connect's authentication handshake
// (implemented by the auth wrapper phase). Ported from PyVLX.__init__ +
// Config.
func NewClient(host, password string, opts ...ClientOption) *Client {
	c := &Client{
		conn:     NewConn(host, DefaultPort),
		sessions: NewSessionIDGenerator(),
		nodes:    NewNodes(),
		password: password,
		timeout:  DefaultTimeoutSeconds * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.apiCall == nil {
		c.apiCall = c.defaultAPICall
	}
	return c
}

// Conn returns the underlying transport connection. Exposed so the auth wrapper
// and command phases can register callbacks or issue writes directly.
func (c *Client) Conn() *Conn { return c.conn }

// Sessions returns the session-id generator, used by command frames that embed
// a session id for response correlation.
func (c *Client) Sessions() *SessionIDGenerator { return c.sessions }

// Nodes returns the node collection.
func (c *Client) Nodes() *Nodes { return c.nodes }

// Password returns the stored gateway password. Exposed for the auth wrapper,
// which performs the GW_PASSWORD_ENTER handshake during Connect.
func (c *Client) Password() string { return c.password }

// Timeout returns the default per-API-call timeout.
func (c *Client) Timeout() time.Duration { return c.timeout }

// Connected reports whether the transport is currently connected.
func (c *Client) Connected() bool { return c.conn.Connected() }

// Connect establishes the transport connection and installs the inbound-frame
// dispatch that feeds registered node updaters. It corresponds to the transport
// portion of PyVLX.connect.
//
// Authentication (password_enter), version/state queries, and house-status
// monitor enabling are performed by the auth wrapper and gateway phases, which
// call APICall after Connect returns; this method exposes the connection and
// the password they need. Ported from PyVLX.connect (transport half).
func (c *Client) Connect(ctx context.Context) error {
	if err := c.conn.Connect(ctx); err != nil {
		return err
	}
	c.installUpdaterDispatch()
	return nil
}

// Disconnect tears down the transport connection. The graceful house-status
// monitor disable that must precede it (per PyVLX.disconnect) is issued by the
// gateway phase before calling this. Ported from PyVLX.disconnect (transport
// half).
func (c *Client) Disconnect() error {
	c.mu.Lock()
	if c.updaterSet {
		c.conn.UnregisterFrameReceivedCB(c.updaterCBID)
		c.updaterSet = false
	}
	c.mu.Unlock()
	return c.conn.Disconnect()
}

// RegisterNodeUpdater registers a frame processor that receives every inbound
// frame while connected. This is the seam the live-updates author uses to plug
// in the node-updater logic. Processors registered before Connect are picked up
// when the dispatch is installed; processors registered after Connect take
// effect immediately. Corresponds to
// connection.register_frame_received_cb(node_updater.process_frame).
func (c *Client) RegisterNodeUpdater(p FrameProcessor) {
	if p == nil {
		return
	}
	c.mu.Lock()
	c.nodeUpdaters = append(c.nodeUpdaters, p)
	c.mu.Unlock()
}

// installUpdaterDispatch registers a single connection-level callback that fans
// every inbound frame out to the registered node updaters. It is idempotent per
// connection. Kept as one callback (rather than one per updater) so it is
// cleanly removed on Disconnect.
func (c *Client) installUpdaterDispatch() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.updaterSet {
		return
	}
	c.updaterCBID = c.conn.RegisterFrameReceivedCB(func(frame protocol.Frame) {
		c.mu.Lock()
		processors := make([]FrameProcessor, len(c.nodeUpdaters))
		copy(processors, c.nodeUpdaters)
		c.mu.Unlock()
		for _, p := range processors {
			p(frame)
		}
	})
	c.updaterSet = true
}

// APICall runs a single request/response exchange over the client's connection,
// applying the client's default timeout if the context has no deadline. It is a
// thin wrapper over DoAPICall, giving command and gateway code a single entry
// point. Ported from the do_api_call usage throughout pyvlx.
func (c *Client) APICall(ctx context.Context, request protocol.Frame, handle FrameHandler) error {
	call := c.apiCall
	if call == nil {
		call = c.defaultAPICall
	}
	return call(ctx, request, handle)
}

// defaultAPICall is the production implementation of the APICall seam: it runs
// the exchange over the client's transport connection, applying the client's
// default timeout if the context has no deadline. Ported from the do_api_call
// usage throughout pyvlx.
func (c *Client) defaultAPICall(ctx context.Context, request protocol.Frame, handle FrameHandler) error {
	if c.conn == nil {
		return errors.New("klf200: client has no connection")
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && c.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.timeout)
		defer cancel()
	}
	return DoAPICall(ctx, c.conn, request, handle)
}

// LoadNodes loads all nodes from the gateway and rebuilds the Nodes collection.
// It delegates to LoadAllNodes (node_loading.go) which faithfully ports
// GetAllNodesInformation from api/get_all_nodes_information.py, then converts
// each notification frame via node_helper and repopulates Client.Nodes.
// Ported from Nodes._load_all_nodes / PyVLX.load_nodes.
func (c *Client) LoadNodes(ctx context.Context) error {
	collected, err := LoadAllNodes(ctx, c)
	if err != nil {
		return fmt.Errorf("klf200: load nodes: %w", err)
	}

	c.nodes.Clear()
	for _, ntf := range collected {
		node := ConvertAllNodesFrame(c, ntf)
		if node != nil {
			c.nodes.Add(node)
		}
	}
	return nil
}
