package klf200

import (
	"crypto/sha1" //nolint:gosec // fingerprint, not signature.
	"crypto/tls"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestVerifyPinnedFingerprint_Match(t *testing.T) {
	raw := []byte("fake DER bytes for a KLF-200 leaf certificate")
	pinned := sha1.Sum(raw) //nolint:gosec // fingerprint, not signature.
	if err := verifyPinnedFingerprint([][]byte{raw}, pinned); err != nil {
		t.Fatalf("expected match, got error: %v", err)
	}
}

func TestVerifyPinnedFingerprint_Mismatch(t *testing.T) {
	raw := []byte("some other certificate")
	var pinned [20]byte // all-zero, definitely not the SHA-1 of `raw`
	err := verifyPinnedFingerprint([][]byte{raw}, pinned)
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "does not match pinned") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestVerifyPinnedFingerprint_NoCert(t *testing.T) {
	err := verifyPinnedFingerprint(nil, veluxCertFingerprintSHA1)
	if err == nil {
		t.Fatal("expected error when peer sent no cert, got nil")
	}
	if !strings.Contains(err.Error(), "no certificate") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestConfigureTCPKeepAlive_SetsSockopt verifies SO_KEEPALIVE ends up enabled
// on the underlying socket after configureTCPKeepAlive runs against a real
// TLS-wrapped TCP connection. The tls.Client wrap needs no handshake for this
// test: NetConn() returns the unwrapped *net.TCPConn either way.
func TestConfigureTCPKeepAlive_SetsSockopt(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			defer c.Close()
			_, _ = c.Read(make([]byte, 1)) // block until client closes
		}
	}()
	nc, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	// tls.Client requires a non-nil config, but we never trigger the handshake
	// here — configureTCPKeepAlive only reads NetConn() and touches the raw
	// socket. A ServerName-only config is safe and non-insecure.
	tlsConn := tls.Client(nc, &tls.Config{ServerName: "keepalive-test.local"})
	defer tlsConn.Close()

	if err := configureTCPKeepAlive(tlsConn); err != nil {
		t.Fatalf("configureTCPKeepAlive: %v", err)
	}

	tc, ok := tlsConn.NetConn().(*net.TCPConn)
	if !ok {
		t.Fatalf("expected *net.TCPConn under tls.Conn, got %T", tlsConn.NetConn())
	}
	sc, err := tc.SyscallConn()
	if err != nil {
		t.Fatalf("SyscallConn: %v", err)
	}
	var enabled bool
	var sockoptErr error
	err = sc.Control(func(fd uintptr) {
		v, e := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE)
		sockoptErr = e
		enabled = v != 0
	})
	if err != nil {
		t.Fatalf("Control: %v", err)
	}
	if sockoptErr != nil {
		t.Fatalf("GetsockoptInt: %v", sockoptErr)
	}
	if !enabled {
		t.Fatal("SO_KEEPALIVE not set on socket after configureTCPKeepAlive")
	}
}

// TestGracefulClose_BoundedWhenPeerNeverFINs verifies that gracefulClose
// returns within roughly its budget even when the peer accepts and then never
// closes its write side, i.e. we never see the peer's FIN.
func TestGracefulClose_BoundedWhenPeerNeverFINs(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	acceptedCh := make(chan net.Conn, 1)
	go func() {
		if c, err := ln.Accept(); err == nil {
			acceptedCh <- c
		}
	}()
	nc, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tc := nc.(*net.TCPConn)
	server := <-acceptedCh
	defer server.Close() // never closes its write side before we do

	budget := 200 * time.Millisecond
	start := time.Now()
	err = gracefulClose(tc, budget)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("gracefulClose returned unexpected error: %v", err)
	}
	// Allow generous headroom for CI jitter; the point is that we do NOT wait
	// forever, not that we time exactly to the budget.
	if elapsed > budget+500*time.Millisecond {
		t.Fatalf("gracefulClose took %v, expected <= %v", elapsed, budget+500*time.Millisecond)
	}
	if elapsed < budget/4 {
		t.Fatalf("gracefulClose returned suspiciously fast (%v) — did it actually drain?", elapsed)
	}
}

// TestGracefulClose_ReturnsWhenPeerFINs verifies the fast path: when the peer
// closes cleanly, gracefulClose completes well before the drain budget.
func TestGracefulClose_ReturnsWhenPeerFINs(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	acceptedCh := make(chan net.Conn, 1)
	go func() {
		if c, err := ln.Accept(); err == nil {
			acceptedCh <- c
		}
	}()
	nc, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tc := nc.(*net.TCPConn)
	server := <-acceptedCh
	// Peer FINs immediately.
	server.Close()

	budget := 2 * time.Second
	start := time.Now()
	if err := gracefulClose(tc, budget); err != nil {
		t.Fatalf("gracefulClose returned unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > budget/2 {
		t.Fatalf("gracefulClose took %v against a FIN-ing peer; expected < %v", elapsed, budget/2)
	}
}

// TestVeluxFingerprint_Constant guards against accidental edits to the pinned
// bytes: the expected value is the SHA-1 fingerprint published in
// MiSchroe/klf-200-api PR #255 (02:8C:23:A0:...:6A).
func TestVeluxFingerprint_Constant(t *testing.T) {
	want := [20]byte{
		0x02, 0x8C, 0x23, 0xA0, 0x89, 0x2B, 0x62, 0x98,
		0xC4, 0x99, 0x00, 0x5B, 0xD2, 0xE7, 0x2E, 0x0A,
		0x70, 0x3D, 0x71, 0x6A,
	}
	if veluxCertFingerprintSHA1 != want {
		t.Fatalf("pinned fingerprint drift: got %x want %x", veluxCertFingerprintSHA1, want)
	}
}
