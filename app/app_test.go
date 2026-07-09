package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mqtt-home/velux-mqtt-gw/config"
)

// fakeKLF is a test double for the klfClient seam. It records the ordered
// sequence of lifecycle calls so a test can assert the exact reset choreography
// (clean Disconnect BEFORE reconnect/reauth/reload) without any hardware. It can
// also be made to fail connect to model a still-wedged gateway.
type fakeKLF struct {
	mu           sync.Mutex
	calls        []string
	connectErr   error // returned from Connect while set (models a wedged gateway)
	disconnected int
	connected    int
	loaded       int
}

func (f *fakeKLF) record(s string) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	f.mu.Unlock()
}

func (f *fakeKLF) Connect(_ context.Context) error {
	f.record("connect")
	f.mu.Lock()
	err := f.connectErr
	if err == nil {
		f.connected++
	}
	f.mu.Unlock()
	return err
}

func (f *fakeKLF) Disconnect() error {
	f.record("disconnect")
	f.mu.Lock()
	f.disconnected++
	f.mu.Unlock()
	return nil
}

func (f *fakeKLF) Connected() bool { return true }

func (f *fakeKLF) PasswordEnter(_ context.Context, _ string) error {
	f.record("password")
	return nil
}

func (f *fakeKLF) SetUTC(_ context.Context, _ time.Time) error {
	f.record("setutc")
	return nil
}

func (f *fakeKLF) LoadNodes(_ context.Context) error {
	f.record("load")
	f.mu.Lock()
	f.loaded++
	f.mu.Unlock()
	return nil
}

func (f *fakeKLF) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

// fakeManager is a test double for the coverManager seam. It counts Register /
// availability / CloseAll invocations so tests can assert re-registration on
// reset, offline/online transitions on wedge/recovery, and shutdown ordering.
type fakeManager struct {
	mu          sync.Mutex
	registers   int
	unavailable int
	available   int
	closeAll    int
	registerErr error
	order       []string
}

func (m *fakeManager) Register(_ context.Context) error {
	m.mu.Lock()
	m.registers++
	m.order = append(m.order, "register")
	err := m.registerErr
	m.mu.Unlock()
	return err
}

func (m *fakeManager) MarkAllUnavailable() {
	m.mu.Lock()
	m.unavailable++
	m.order = append(m.order, "unavailable")
	m.mu.Unlock()
}

func (m *fakeManager) MarkAllAvailable() {
	m.mu.Lock()
	m.available++
	m.order = append(m.order, "available")
	m.mu.Unlock()
}

func (m *fakeManager) CloseAll() {
	m.mu.Lock()
	m.closeAll++
	m.order = append(m.order, "closeall")
	m.mu.Unlock()
}

func (m *fakeManager) counts() (reg, unavail, avail, close int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.registers, m.unavailable, m.available, m.closeAll
}

// newTestApp builds an App wired entirely to fakes: no real klf200.Client, no
// real bridge.Manager, no real heartbeat. The heartbeat lifecycle is replaced
// with counters, and MQTT is represented only by the absence of any teardown
// (the fakes never touch it), so "MQTT stays up across a reset" is expressed as
// "no MQTT-affecting call happens during doReset".
func newTestApp(cfg config.Config) (*App, *fakeKLF, *fakeManager, *int) {
	klf := &fakeKLF{}
	mgr := &fakeManager{}
	hbStarts := 0
	a := &App{
		cfg:    cfg,
		client: nil, // nil => connectAndLoad skips the real house-monitor call
		mgr:    nil, // never used: Stop/CloseAll go through the cm seam
		klf:    klf,
		cm:     mgr,
		now:    time.Now,
	}
	a.startHeartbeatFn = func() { hbStarts++ }
	a.stopHeartbeatFn = func() {}
	a.stopCh = make(chan struct{})
	return a, klf, mgr, &hbStarts
}

// TestPeriodicResetCycle asserts a periodic reset performs an in-process CLEAN
// disconnect FIRST, then reconnect + reauth + reload + re-register, restarts the
// heartbeat, and never calls CloseAll (which would flap HA) — i.e. MQTT/HA are
// left untouched.
func TestPeriodicResetCycle(t *testing.T) {
	a, klf, mgr, hbStarts := newTestApp(config.Config{})
	// Zero the reconnect delay so the test does not actually sleep 2s.
	oldDelay := resetReconnectDelay
	resetReconnectDelay = 0
	defer func() { resetReconnectDelay = oldDelay }()

	if err := a.doReset(context.Background()); err != nil {
		t.Fatalf("doReset: %v", err)
	}

	got := klf.snapshot()
	// The clean disconnect must come first, then the reconnect sequence.
	want := []string{"disconnect", "connect", "password", "setutc", "load"}
	if len(got) != len(want) {
		t.Fatalf("klf calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("klf call[%d] = %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
	if klf.disconnected != 1 || klf.connected != 1 || klf.loaded != 1 {
		t.Fatalf("disc=%d conn=%d load=%d, want 1/1/1", klf.disconnected, klf.connected, klf.loaded)
	}

	reg, unavail, avail, closeAll := mgr.counts()
	if reg != 1 {
		t.Fatalf("Register calls = %d, want 1 (re-register after reset)", reg)
	}
	if closeAll != 0 {
		t.Fatalf("CloseAll during reset = %d, want 0 (HA must not flap)", closeAll)
	}
	if unavail != 0 || avail != 0 {
		t.Fatalf("availability toggled during clean reset: unavail=%d avail=%d, want 0/0", unavail, avail)
	}
	if *hbStarts != 1 {
		t.Fatalf("heartbeat restarts = %d, want 1", *hbStarts)
	}
}

// TestWedgeMarksOfflineAndRetries asserts that when lastContact is stale beyond
// interval x threshold, checkHealth publishes offline availability once, and —
// with restart-on-error — keeps attempting reconnect on subsequent ticks.
func TestWedgeMarksOfflineAndRetries(t *testing.T) {
	cfg := config.Config{}
	cfg.Restart.HealthCheckInterval = 10 // seconds
	cfg.Restart.RestartOnError = true

	a, klf, mgr, _ := newTestApp(cfg)
	resetReconnectDelay = 0
	// Make the reconnect keep failing so the gateway stays "wedged".
	klf.connectErr = errors.New("still wedged")

	// Freeze a clock far past the threshold (10s x 2 = 20s window).
	base := time.Unix(1_000_000, 0)
	a.now = func() time.Time { return base }
	// Stamp contact well in the past.
	a.lastContact.Store(base.Add(-60 * time.Second).UnixNano())

	// First tick: should trip the wedge (offline once) and attempt a reconnect
	// that fails (so it stays wedged).
	a.checkHealth(context.Background())
	if !a.wedged.Load() {
		t.Fatalf("expected wedged after first stale check")
	}
	_, unavail, avail, _ := mgr.counts()
	if unavail != 1 {
		t.Fatalf("MarkAllUnavailable calls = %d, want 1", unavail)
	}
	if avail != 0 {
		t.Fatalf("MarkAllAvailable calls = %d, want 0 while wedged", avail)
	}
	firstDisc := klf.disconnected

	// Second tick: still stale, still wedged. Must NOT re-publish offline (only
	// on the transition) but MUST keep retrying reconnect (disconnect attempted
	// again), i.e. not silently give up.
	a.checkHealth(context.Background())
	_, unavail2, _, _ := mgr.counts()
	if unavail2 != 1 {
		t.Fatalf("MarkAllUnavailable calls = %d after 2nd tick, want 1 (transition-only)", unavail2)
	}
	if klf.disconnected <= firstDisc {
		t.Fatalf("expected another reconnect attempt on 2nd tick (disc %d -> %d)", firstDisc, klf.disconnected)
	}
}

// TestWedgeRecoveryRestoresAvailability asserts that once the gateway is
// reachable again, a health-check-driven reconnect succeeds, reloads, and
// restores availability (online).
func TestWedgeRecoveryRestoresAvailability(t *testing.T) {
	cfg := config.Config{}
	cfg.Restart.HealthCheckInterval = 10
	cfg.Restart.RestartOnError = true

	a, klf, mgr, _ := newTestApp(cfg)
	resetReconnectDelay = 0
	klf.connectErr = errors.New("wedged")

	base := time.Unix(2_000_000, 0)
	a.now = func() time.Time { return base }
	a.lastContact.Store(base.Add(-60 * time.Second).UnixNano())

	// Trip the wedge.
	a.checkHealth(context.Background())
	if !a.wedged.Load() {
		t.Fatalf("expected wedged")
	}

	// Gateway recovers: connect now succeeds. doReset within checkHealth reloads
	// and stamps contact; recovery restores availability.
	klf.mu.Lock()
	klf.connectErr = nil
	klf.mu.Unlock()

	a.checkHealth(context.Background())
	if a.wedged.Load() {
		t.Fatalf("expected recovery to clear wedged")
	}
	reg, _, avail, _ := mgr.counts()
	if avail < 1 {
		t.Fatalf("MarkAllAvailable calls = %d, want >=1 after recovery", avail)
	}
	if reg < 1 {
		t.Fatalf("Register calls = %d, want >=1 (reload on recovery)", reg)
	}
	if klf.loaded < 1 {
		t.Fatalf("expected LoadNodes on recovery reconnect")
	}
}

// TestShutdownOrdering asserts Stop performs discovery cleanup + cover offline
// (CloseAll) and then a clean KLF200 disconnect, and that it never disconnects
// before closing HA — the shutdown contract. It also asserts the heartbeat is
// stopped and Stop is idempotent.
func TestShutdownOrdering(t *testing.T) {
	a, klf, mgr, _ := newTestApp(config.Config{})
	stopHBCalls := 0
	a.stopHeartbeatFn = func() { stopHBCalls++ }

	a.Stop()

	// CloseAll must happen (discovery cleanup + covers offline).
	if _, _, _, closeAll := mgr.counts(); closeAll != 1 {
		t.Fatalf("CloseAll calls = %d, want 1", closeAll)
	}
	// A clean KLF200 disconnect must happen on shutdown.
	if klf.disconnected != 1 {
		t.Fatalf("Disconnect calls = %d, want 1", klf.disconnected)
	}
	// Ordering: CloseAll (HA cleanup) BEFORE the clean disconnect. The manager
	// records "closeall"; the klf records "disconnect". Assert closeall was the
	// last manager action and disconnect the only klf action, and that CloseAll
	// preceded Disconnect by checking neither reconnect nor register ran.
	if len(klf.snapshot()) != 1 || klf.snapshot()[0] != "disconnect" {
		t.Fatalf("klf calls on shutdown = %v, want exactly [disconnect]", klf.snapshot())
	}
	if stopHBCalls < 1 {
		t.Fatalf("heartbeat stop calls = %d, want >=1", stopHBCalls)
	}

	// Idempotent: a second Stop must not double-disconnect or double-close.
	a.Stop()
	if _, _, _, closeAll := mgr.counts(); closeAll != 1 {
		t.Fatalf("CloseAll after second Stop = %d, want still 1 (idempotent)", closeAll)
	}
	if klf.disconnected != 1 {
		t.Fatalf("Disconnect after second Stop = %d, want still 1 (idempotent)", klf.disconnected)
	}
}
