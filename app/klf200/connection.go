// Package klf200 implements the transport and runtime layer for talking to a
// Velux KLF200 gateway: the TLS connection and read loop (connection.go), the
// session-id generator (session.go), and the request/response correlation
// helper (event.go). It is ported from tjaehnel/pyvlx@master_vlxmqttha and
// builds on the leaf protocol package.
package klf200

import (
	"context"
	"crypto/sha1" //nolint:gosec // used only as a certificate fingerprint, not a signature.
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// DefaultPort is the TCP port the KLF200 API listens on. Ported from
// config.Config.DEFAULT_PORT.
const DefaultPort = 51200

// veluxCertFingerprintSHA1 is the SHA-1 fingerprint of the shared, self-signed
// VELUX certificate that every KLF-200 gateway presents on its TLS listener.
// The certificate itself was issued 2018-04-25 with an 8-year lifetime and
// expired 2026-07-12 09:38:26 GMT — so a normal chain/expiry check now rejects
// every device in the field. We deliberately bypass Go's built-in verification
// (InsecureSkipVerify) and instead pin this fingerprint via VerifyPeerCertificate,
// which mirrors the fix in the upstream JS lib (MiSchroe/klf-200-api PR #255):
// only the known VELUX certificate is accepted, its expiry is intentionally
// ignored, and any other cert (including a MITM attempt with a different
// self-signed cert) is rejected.
var veluxCertFingerprintSHA1 = [20]byte{
	0x02, 0x8C, 0x23, 0xA0, 0x89, 0x2B, 0x62, 0x98,
	0xC4, 0x99, 0x00, 0x5B, 0xD2, 0xE7, 0x2E, 0x0A,
	0x70, 0x3D, 0x71, 0x6A,
}

// verifyPinnedFingerprint accepts the peer's leaf certificate iff its SHA-1
// fingerprint equals pinned. Certificate expiry is intentionally not checked —
// see veluxCertFingerprintSHA1 for the story.
func verifyPinnedFingerprint(rawCerts [][]byte, pinned [20]byte) error {
	if len(rawCerts) == 0 {
		return errors.New("klf200: peer presented no certificate")
	}
	got := sha1.Sum(rawCerts[0]) //nolint:gosec // fingerprint, not signature.
	if got != pinned {
		return fmt.Errorf("klf200: peer certificate SHA-1 fingerprint %x does not match pinned VELUX fingerprint %x", got, pinned)
	}
	return nil
}

// FrameReceivedCallback is invoked for every decoded, non-nil frame read from
// the gateway. It corresponds to the callables registered with pyvlx's
// Connection.register_frame_received_cb.
//
// Callbacks are invoked from the connection's single read-loop goroutine. A
// callback MUST NOT block for long (and MUST NOT call Disconnect synchronously),
// as that stalls delivery of subsequent frames.
type FrameReceivedCallback func(frame protocol.Frame)

// callbackHandle uniquely identifies a registered callback so it can be removed
// even though funcs are not comparable.
type callbackHandle struct {
	id uint64
	cb FrameReceivedCallback
}

// Conn owns a TLS connection to a KLF200 gateway and a single read-loop
// goroutine that tokenizes the SLIP stream, decodes frames, and dispatches them
// to registered callbacks. It is the Go counterpart of pyvlx's Connection
// (plus its TCPTransport/SlipTokenizer helpers).
type Conn struct {
	host string
	port int

	mu        sync.Mutex
	tlsConn   *tls.Conn
	connected bool
	nextCBID  uint64
	callbacks []callbackHandle

	// connectionCounter counts successful connects, mirroring
	// Connection.connection_counter.
	connectionCounter int

	// life is the current connection lifecycle. Its channel is closed exactly
	// once when that connection is lost (read loop terminated) or after
	// Disconnect — the Go equivalent of connection_closed_cb. Each Connect
	// installs a fresh lifecycle so waiters and the read loop always act on
	// the channel belonging to their own connection.
	life *lifecycle
}

// lifecycle pairs a connection-lost channel with a one-shot closer so it is
// closed exactly once regardless of who observes loss first.
type lifecycle struct {
	lost chan struct{}
	once sync.Once
}

func newLifecycle() *lifecycle {
	return &lifecycle{lost: make(chan struct{})}
}

func (l *lifecycle) signalLost() {
	l.once.Do(func() { close(l.lost) })
}

// NewConn returns a Conn for the given gateway host. If port is 0, DefaultPort
// is used.
func NewConn(host string, port int) *Conn {
	if port == 0 {
		port = DefaultPort
	}
	return &Conn{
		host: host,
		port: port,
		life: newLifecycle(),
	}
}

// Connect establishes the TLS connection and starts the read loop. The KLF200
// presents a shared, self-signed VELUX certificate whose fingerprint we pin
// (see veluxCertFingerprintSHA1). InsecureSkipVerify disables Go's default
// chain/expiry check — needed both because the cert is self-signed and because
// it expired 2026-07-12 — and VerifyPeerCertificate then enforces the pin, so
// only the known VELUX certificate is ever accepted. The provided context
// bounds the dial/handshake only; once connected the read loop runs until the
// connection is lost or Disconnect is called.
func (c *Conn) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.connected {
		c.mu.Unlock()
		return errors.New("klf200: already connected")
	}
	c.mu.Unlock()

	addr := net.JoinHostPort(c.host, fmt.Sprintf("%d", c.port))
	dialer := &tls.Dialer{
		Config: &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // pinned via VerifyPeerCertificate below.
			VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				return verifyPinnedFingerprint(rawCerts, veluxCertFingerprintSHA1)
			},
			// The KLF200's TLS stack does not support TLS 1.3 and freezes on a
			// TLS 1.3 ClientHello instead of negotiating down (confirmed against
			// hardware: openssl -tls1_3 gets alert 40, Go's default 1.3 offer
			// hangs the handshake). Cap at TLS 1.2, which the gateway speaks.
			MinVersion: tls.VersionTLS10,
			MaxVersion: tls.VersionTLS12,
		},
	}
	nc, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("klf200: connect to %s: %w", addr, err)
	}
	tlsConn := nc.(*tls.Conn)

	c.mu.Lock()
	c.tlsConn = tlsConn
	c.connected = true
	c.connectionCounter++
	// Fresh lifecycle for this connection.
	life := newLifecycle()
	c.life = life
	c.mu.Unlock()

	go c.readLoop(tlsConn, life)
	return nil
}

// Connected reports whether the connection is currently established.
func (c *Conn) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// ConnectionCounter returns the number of successful connects, mirroring
// pyvlx's Connection.connection_counter.
func (c *Conn) ConnectionCounter() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connectionCounter
}

// Lost returns a channel that is closed when the current connection is lost
// (read loop terminated) or after Disconnect. It is the Go equivalent of
// pyvlx's connection_closed_cb signal. Capture it before you need to wait; a
// subsequent Connect installs a new channel.
func (c *Conn) Lost() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.life.lost
}

// RegisterFrameReceivedCB registers a callback for received frames and returns
// its handle for later unregistration. Ported from
// Connection.register_frame_received_cb.
func (c *Conn) RegisterFrameReceivedCB(cb FrameReceivedCallback) uint64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextCBID++
	id := c.nextCBID
	c.callbacks = append(c.callbacks, callbackHandle{id: id, cb: cb})
	return id
}

// UnregisterFrameReceivedCB removes a previously registered callback by handle.
// Ported from Connection.unregister_frame_received_cb.
func (c *Conn) UnregisterFrameReceivedCB(handle uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i, h := range c.callbacks {
		if h.id == handle {
			c.callbacks = append(c.callbacks[:i], c.callbacks[i+1:]...)
			return
		}
	}
}

// Write SLIP-packs a frame and sends it to the gateway. Ported from
// Connection.write.
func (c *Conn) Write(frame protocol.Frame) error {
	if frame == nil {
		return errors.New("klf200: cannot write nil frame")
	}
	raw, err := protocol.MarshalFrame(frame)
	if err != nil {
		return fmt.Errorf("klf200: marshal frame: %w", err)
	}
	c.mu.Lock()
	conn := c.tlsConn
	connected := c.connected
	c.mu.Unlock()
	if !connected || conn == nil {
		return errors.New("klf200: not connected")
	}
	if _, err := conn.Write(protocol.SlipPack(raw)); err != nil {
		return fmt.Errorf("klf200: write frame: %w", err)
	}
	return nil
}

// Disconnect performs an orderly TLS/TCP close (TLS close_notify then TCP FIN).
// This is critical: it releases the KLF200 API session slot so a later
// reconnect can succeed. It is idempotent. Ported from Connection.disconnect,
// but performs a graceful close rather than an abrupt one.
func (c *Conn) Disconnect() error {
	c.mu.Lock()
	conn := c.tlsConn
	life := c.life
	c.tlsConn = nil
	c.connected = false
	c.mu.Unlock()

	if conn == nil {
		return nil
	}
	// CloseWrite sends the TLS close_notify alert followed by a TCP FIN,
	// giving the gateway an orderly shutdown so it releases the session slot.
	var firstErr error
	if err := conn.CloseWrite(); err != nil {
		firstErr = err
	}
	if err := conn.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	life.signalLost()
	if firstErr != nil {
		return fmt.Errorf("klf200: disconnect: %w", firstErr)
	}
	return nil
}

// readLoop is the single reader goroutine. It reads bytes, feeds the SLIP
// tokenizer, decodes each complete packet with protocol.FrameFromRaw, and
// dispatches non-nil frames to registered callbacks. It mirrors
// TCPTransport.data_received + connection_lost. It exits when the connection
// is closed (by the gateway or by Disconnect) or on a read error, and then
// signals loss on its own lifecycle.
func (c *Conn) readLoop(conn *tls.Conn, life *lifecycle) {
	defer func() {
		c.mu.Lock()
		// Only tear down shared state if this is still the active connection
		// (a later Connect may have replaced it).
		if c.tlsConn == conn {
			c.connected = false
			c.tlsConn = nil
		}
		c.mu.Unlock()
		// Always close our own lifecycle channel; it is unique to this loop.
		life.signalLost()
	}()

	buf := make([]byte, 4096)
	var stream []byte
	for {
		n, err := conn.Read(buf)
		if n > 0 {
			stream = append(stream, buf[:n]...)
			// Drain all complete SLIP packets currently buffered.
			for protocol.IsSlip(stream) {
				var packet []byte
				packet, stream = protocol.GetNextSlip(stream)
				if packet == nil {
					break
				}
				frame, ferr := protocol.FrameFromRaw(packet)
				if ferr != nil {
					// Malformed frame: skip, matching pyvlx's tolerant
					// stream handling (frame_from_raw errors are logged,
					// not fatal).
					continue
				}
				if frame != nil {
					c.dispatch(frame)
				}
			}
		}
		if err != nil {
			return
		}
	}
}

// dispatch delivers a frame to a snapshot of the registered callbacks. Ported
// from Connection.frame_received_cb.
func (c *Conn) dispatch(frame protocol.Frame) {
	c.mu.Lock()
	snapshot := make([]FrameReceivedCallback, len(c.callbacks))
	for i, h := range c.callbacks {
		snapshot[i] = h.cb
	}
	c.mu.Unlock()
	for _, cb := range snapshot {
		cb(frame)
	}
}
