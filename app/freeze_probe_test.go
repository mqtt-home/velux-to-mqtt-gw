package main

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/config"
)

// TestClassifyDialError exercises the four TCP-signal outcome branches
// (accepted/refused/reset/timeout) plus the unclassified-error fallthrough
// (which folds into `error`, not `timeout`).
func TestClassifyDialError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"nil error", nil, probeOutcomeAccepted},
		{"ECONNREFUSED", syscall.ECONNREFUSED, probeOutcomeRefused},
		{"ECONNRESET", syscall.ECONNRESET, probeOutcomeReset},
		{"wrapped ECONNREFUSED", &net.OpError{Op: "dial", Err: &os.SyscallError{Err: syscall.ECONNREFUSED}}, probeOutcomeRefused},
		{"wrapped ECONNRESET", &net.OpError{Op: "read", Err: &os.SyscallError{Err: syscall.ECONNRESET}}, probeOutcomeReset},
		{"context deadline exceeded", context.DeadlineExceeded, probeOutcomeTimeout},
		{"os.IsTimeout error", &timeoutErr{}, probeOutcomeTimeout},
		{"context canceled → error", context.Canceled, probeOutcomeError},
		{"EHOSTUNREACH → error", syscall.EHOSTUNREACH, probeOutcomeError},
		{"ENETUNREACH → error", syscall.ENETUNREACH, probeOutcomeError},
		{"unclassified error → error", errors.New("something else"), probeOutcomeError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyDialError(tc.err)
			if got != tc.want {
				t.Fatalf("classifyDialError(%v) = %q, want %q", tc.err, got, tc.want)
			}
		})
	}
}

// timeoutErr is a minimal net.Error whose Timeout() returns true, so
// os.IsTimeout classifies it.
type timeoutErr struct{}

func (*timeoutErr) Error() string   { return "timeout" }
func (*timeoutErr) Timeout() bool   { return true }
func (*timeoutErr) Temporary() bool { return false }

// TestFreezeProbe_AcceptedAgainstLiveListener verifies the end-to-end
// dial-and-classify path against a real listener that accepts immediately.
func TestFreezeProbe_AcceptedAgainstLiveListener(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go func() {
		if c, err := ln.Accept(); err == nil {
			_ = c.Close()
		}
	}()

	outcome, elapsed, dialErr := freezeProbe(context.Background(), ln.Addr().String())
	if outcome != probeOutcomeAccepted {
		t.Fatalf("outcome = %q, want %q", outcome, probeOutcomeAccepted)
	}
	if dialErr != nil {
		t.Fatalf("dialErr = %v, want nil", dialErr)
	}
	if elapsed > probeTimeout {
		t.Fatalf("elapsed = %v, expected << probeTimeout=%v", elapsed, probeTimeout)
	}
}

// TestFreezeProbe_RefusedAgainstUnusedPort verifies that a dial to a closed
// port classifies as `refused` (kernel replies with RST on our SYN → the
// syscall returns ECONNREFUSED).
func TestFreezeProbe_RefusedAgainstUnusedPort(t *testing.T) {
	// Grab a free port then immediately release it, so we're guaranteed nothing
	// is listening there.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	outcome, _, _ := freezeProbe(context.Background(), addr)
	if outcome != probeOutcomeRefused {
		t.Fatalf("outcome = %q, want %q", outcome, probeOutcomeRefused)
	}
}

// TestFreezeProbe_TimeoutViaTinyContext verifies the `timeout` branch by
// dialing an address that would either accept or refuse, but with a context
// timeout so short the dial cannot complete in time. This exercises the
// context.DeadlineExceeded branch of the classifier without needing a real
// blackhole route (which is not portable across CI environments).
func TestFreezeProbe_TimeoutViaTinyContext(t *testing.T) {
	// TEST-NET-1 (RFC 5737): 192.0.2.0/24 is reserved for documentation and is
	// unreachable from every real network — a well-defined blackhole.
	const blackhole = "192.0.2.1:51200"
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	outcome, elapsed, _ := freezeProbe(ctx, blackhole)
	if outcome != probeOutcomeTimeout {
		t.Fatalf("outcome = %q, want %q (elapsed=%v)", outcome, probeOutcomeTimeout, elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("timeout took %v; expected ~50ms + jitter", elapsed)
	}
}

// TestScheduleFollowUpProbes_ReturnsImmediately_ExitsOnStopCh covers the
// spec's "Recovery not delayed by probes" requirement: scheduleFollowUpProbes
// must return without waiting on any follow-up, and the spawned goroutine
// must exit promptly when the App shuts down (closing stopCh), not sit for
// the full 300 s follow-up window.
func TestScheduleFollowUpProbes_ReturnsImmediately_ExitsOnStopCh(t *testing.T) {
	a, _, _, _ := newTestApp(config.Config{})

	// scheduleFollowUpProbes must return without blocking on the follow-up
	// timers. Give it a generous jitter allowance — the test asserts "not
	// blocking on 120 s", not tight timing.
	start := time.Now()
	a.scheduleFollowUpProbes(context.Background(), "test-event", time.Now())
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Fatalf("scheduleFollowUpProbes took %v; expected near-immediate return", elapsed)
	}

	// Signal shutdown; the follow-up goroutine must observe stopCh and exit
	// well inside its 120 s / 300 s timers.
	close(a.stopCh)
	done := make(chan struct{})
	go func() {
		a.loopsWG.Wait()
		close(done)
	}()
	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("follow-up goroutine did not exit within 2s after stopCh closed")
	}
}

// TestFreezeProbe_CanceledContext_ClassifiesAsError verifies the code-reviewer
// concern: a caller-cancelled context (typical of shutdown) must NOT be
// misclassified as `timeout` (which would look like a wedge signal).
func TestFreezeProbe_CanceledContext_ClassifiesAsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before dial

	outcome, _, dialErr := freezeProbe(ctx, "192.0.2.1:51200")
	if outcome != probeOutcomeError {
		t.Fatalf("outcome = %q, want %q", outcome, probeOutcomeError)
	}
	if dialErr == nil {
		t.Fatal("expected non-nil dialErr for error outcome")
	}
}
