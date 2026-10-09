package main

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"time"
)

// Freeze-diagnostic probe: classifies TCP-layer response of the KLF200 API port
// at the moment the health check declares the gateway wedged, and at t+120 s /
// t+300 s / after the reactive soft reboot. Log-only; never influences the
// existing recovery flow. See openspec change klf200-tcp-exhaustion-probe.

const (
	// probeTimeout bounds any single probe attempt. 5 s is enough to observe an
	// immediate accept/refuse/reset and cleanly time out a stalled SYN.
	probeTimeout = 5 * time.Second

	// followUpAt120s and followUpAt300s are the two follow-up-probe delays,
	// measured from the initial probe start. The 120 s point catches a
	// TIME_WAIT recycling window; the 300 s point covers the full W5500
	// TIME_WAIT plus headroom. Persistent `timeout` past 300 s is consistent
	// with CLOSE_WAIT/ESTABLISHED-stuck sockets, a W5500 chip hang, or an EFM32
	// task deadlock — the outcome consistent with the observed "only mains
	// power-cycle helps" behavior.
	followUpAt120s = 120 * time.Second
	followUpAt300s = 300 * time.Second
)

// Probe outcome codes, kept short and stable so log-line consumers can rely on
// them. Do not change without updating the openspec spec and the log schema.
const (
	probeOutcomeAccepted = "accepted"
	probeOutcomeRefused  = "refused"
	probeOutcomeReset    = "reset"
	probeOutcomeTimeout  = "timeout"
	// probeOutcomeError covers everything that is not a clean TCP-layer signal:
	// context cancellation (we are shutting down, not a wedge indicator),
	// EHOSTUNREACH / ENETUNREACH (routing / physical-layer problem, not a
	// KLF200 issue), and other unclassified errors. The caller emits the raw
	// err text in the `probe_err=` log field so this class stays diagnostic
	// rather than a bucket of silent misclassification.
	probeOutcomeError = "error"
)

// Probe stages, used as the `stage` log key so a `grep event_id=<id>` yields
// all data points for a single freeze event in order.
const (
	probeStageInitial        = "initial"
	probeStageFollowUp120s   = "follow_up_120s"
	probeStageFollowUp300s   = "follow_up_300s"
	probeStagePostSoftReboot = "post_soft_reboot"
)

// freezeProbe performs one plain-TCP connect (no TLS, no auth) to addr with a
// hard 5 s cap and classifies the outcome. Returns the classified outcome and
// the wall-clock duration to that outcome.
//
// Classifier:
//   - err == nil                     → "accepted" (probe conn is closed immediately)
//   - errors.Is(err, ECONNREFUSED)   → "refused"
//   - errors.Is(err, ECONNRESET)     → "reset"
//   - os.IsTimeout(err) OR ctx timed → "timeout"
//   - anything else                  → "error" (unclassified: context cancel,
//                                       EHOSTUNREACH/ENETUNREACH, permission/DNS
//                                       failures — the raw err is surfaced via
//                                       the `probe_err=` log field, not folded
//                                       into the TCP-stall "timeout" signal)
// freezeProbe returns the outcome, elapsed dial time, and — for the
// probeOutcomeError case — the raw dial error so the caller can surface it
// via the `probe_err=` log field. Timing is captured across the whole dial,
// so a timeout returns ~probeTimeout as elapsed.
func freezeProbe(ctx context.Context, addr string) (outcome string, elapsed time.Duration, dialErr error) {
	start := time.Now()
	dialCtx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	// Rely on the context deadline alone — a separate net.Dialer{Timeout}
	// would only obscure the timing semantics and could mask a shorter caller
	// deadline.
	var d net.Dialer
	conn, err := d.DialContext(dialCtx, "tcp", addr)
	elapsed = time.Since(start)
	if err == nil {
		_ = conn.Close()
	}
	outcome = classifyDialError(err)
	if outcome == probeOutcomeError {
		dialErr = err
	}
	return outcome, elapsed, dialErr
}

// classifyDialError is the pure classifier separated out so the mapping of
// syscall/context errors → probe-outcome codes can be unit-tested exhaustively
// (including the RST-on-connect case, which is not straightforward to trigger
// through a real listener from a Go test). context.Canceled and any other
// unclassified error rolls into probeOutcomeError so the caller can log the
// raw text and stay diagnostic; only genuine deadline expirations classify as
// probeOutcomeTimeout (the "TCP-layer stall" signal this probe was designed
// to detect).
func classifyDialError(err error) string {
	if err == nil {
		return probeOutcomeAccepted
	}
	if errors.Is(err, syscall.ECONNREFUSED) {
		return probeOutcomeRefused
	}
	if errors.Is(err, syscall.ECONNRESET) {
		return probeOutcomeReset
	}
	// Deadline expired = TCP-layer stall; a net.Error with Timeout()==true is
	// the classical shape here (via os.IsTimeout), and errors.Is against
	// context.DeadlineExceeded catches the ctx-driven variant.
	if os.IsTimeout(err) || errors.Is(err, context.DeadlineExceeded) {
		return probeOutcomeTimeout
	}
	// Everything else — context.Canceled (we are shutting down), EHOSTUNREACH
	// / ENETUNREACH (routing / physical), permission errors, DNS failures —
	// is preserved as probeOutcomeError so the caller can log the raw text.
	return probeOutcomeError
}
