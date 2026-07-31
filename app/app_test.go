package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/config"
)

// fakeKLF is a test double for the klfClient seam. It records the ordered
// sequence of lifecycle calls so a test can assert exact choreography
// (e.g. reboot BEFORE reconnect) without any hardware. Various knobs make
// Connect and Reboot fail on demand to model wedged gateways and reboot
// failures.
type fakeKLF struct {
	mu         sync.Mutex
	calls      []string
	connectErr error // returned from Connect while set
	// connectErrsUntil, when > 0, decrements on every Connect call and returns
	// connectErr until it hits zero (models a gateway that takes N attempts to
	// come back after a reboot).
	connectErrsUntil int
	rebootErr        error
	disconnected     int
	connected        int
	loaded           int
	rebooted         int
}

func (f *fakeKLF) record(s string) {
	f.mu.Lock()
	f.calls = append(f.calls, s)
	f.mu.Unlock()
}

func (f *fakeKLF) Connect(_ context.Context) error {
	f.record("connect")
	f.mu.Lock()
	var err error
	if f.connectErrsUntil > 0 {
		f.connectErrsUntil--
		err = f.connectErr
		if err == nil {
			err = errors.New("simulated post-reboot delay")
		}
	} else if f.connectErr != nil {
		err = f.connectErr
	}
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

func (f *fakeKLF) Reboot(_ context.Context) error {
	f.record("reboot")
	f.mu.Lock()
	f.rebooted++
	err := f.rebootErr
	f.mu.Unlock()
	return err
}

func (f *fakeKLF) snapshot() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.calls))
	copy(out, f.calls)
	return out
}

// fakeManager is a test double for the coverManager seam.
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

// withFastReboot swaps the reboot-backoff schedule to zero-duration slots so
// tests do not sleep. Returns a restore function. attempts is how many attempts
// the schedule should permit.
func withFastReboot(t *testing.T, attempts int) {
	t.Helper()
	old := rebootBackoffSchedule
	sched := make([]time.Duration, attempts)
	rebootBackoffSchedule = sched
	t.Cleanup(func() { rebootBackoffSchedule = old })
}

// newTestApp builds an App wired entirely to fakes.
func newTestApp(cfg config.Config) (*App, *fakeKLF, *fakeManager, *int) {
	klf := &fakeKLF{}
	mgr := &fakeManager{}
	hbStarts := 0
	a := &App{
		cfg:    cfg,
		client: nil, // nil => connectAndLoad skips the real house-monitor call
		mgr:    nil,
		klf:    klf,
		cm:     mgr,
		now:    time.Now,
	}
	a.startHeartbeatFn = func() { hbStarts++ }
	a.stopHeartbeatFn = func() {}
	a.stopCh = make(chan struct{})
	return a, klf, mgr, &hbStarts
}

// TestDoReconnect asserts the reconnect primitive performs a clean disconnect
// FIRST, then the reconnect + reauth + reload sequence, restarts the
// heartbeat, and never calls CloseAll (which would flap HA).
func TestDoReconnect(t *testing.T) {
	a, klf, mgr, hbStarts := newTestApp(config.Config{})

	if err := a.doReconnect(context.Background()); err != nil {
		t.Fatalf("doReconnect: %v", err)
	}

	got := klf.snapshot()
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
		t.Fatalf("Register calls = %d, want 1", reg)
	}
	if closeAll != 0 || unavail != 0 || avail != 0 {
		t.Fatalf("HA-flapping calls unexpected: closeAll=%d unavail=%d avail=%d", closeAll, unavail, avail)
	}
	if *hbStarts != 1 {
		t.Fatalf("heartbeat restarts = %d, want 1", *hbStarts)
	}
}

// TestDoRebootHappyPath asserts a reboot sends GW_REBOOT_REQ, then reconnects
// on the first backoff attempt and restarts the heartbeat.
func TestDoRebootHappyPath(t *testing.T) {
	a, klf, mgr, hbStarts := newTestApp(config.Config{})
	withFastReboot(t, 3)

	if err := a.doReboot(context.Background()); err != nil {
		t.Fatalf("doReboot: %v", err)
	}

	if klf.rebooted != 1 {
		t.Fatalf("reboot calls = %d, want 1", klf.rebooted)
	}
	got := klf.snapshot()
	// Reboot first, then a local disconnect to clean up state, then the
	// reconnect sequence.
	want := []string{"reboot", "disconnect", "connect", "password", "setutc", "load"}
	if len(got) != len(want) {
		t.Fatalf("klf calls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("klf call[%d] = %q, want %q (full %v)", i, got[i], want[i], got)
		}
	}
	if reg, _, _, closeAll := mgr.counts(); reg != 1 || closeAll != 0 {
		t.Fatalf("Register=%d closeAll=%d, want 1/0", reg, closeAll)
	}
	if *hbStarts != 1 {
		t.Fatalf("heartbeat restarts = %d, want 1", *hbStarts)
	}
}

// TestDoRebootRetriesUntilSuccess asserts the backoff loop retries a failing
// reconnect and eventually succeeds when the gateway comes back.
func TestDoRebootRetriesUntilSuccess(t *testing.T) {
	a, klf, _, hbStarts := newTestApp(config.Config{})
	withFastReboot(t, 5)
	// First 3 reconnect attempts fail, 4th succeeds.
	klf.connectErrsUntil = 3

	if err := a.doReboot(context.Background()); err != nil {
		t.Fatalf("doReboot: %v", err)
	}
	if klf.connected != 1 {
		t.Fatalf("connected = %d, want 1 (only the successful one)", klf.connected)
	}
	// The fake records every Connect regardless of error, so we expect 4 total.
	connects := 0
	for _, c := range klf.snapshot() {
		if c == "connect" {
			connects++
		}
	}
	if connects != 4 {
		t.Fatalf("Connect attempts = %d, want 4", connects)
	}
	if *hbStarts != 1 {
		t.Fatalf("heartbeat restarts = %d, want 1", *hbStarts)
	}
}

// TestDoRebootExhaustsBudget asserts that if every reconnect attempt fails the
// function returns an error and does NOT restart the heartbeat.
func TestDoRebootExhaustsBudget(t *testing.T) {
	a, klf, _, hbStarts := newTestApp(config.Config{})
	withFastReboot(t, 3)
	klf.connectErr = errors.New("gateway still down")

	if err := a.doReboot(context.Background()); err == nil {
		t.Fatalf("doReboot: want error, got nil")
	}
	// All 3 attempts must have been tried.
	connects := 0
	for _, c := range klf.snapshot() {
		if c == "connect" {
			connects++
		}
	}
	if connects != 3 {
		t.Fatalf("Connect attempts = %d, want 3", connects)
	}
	if *hbStarts != 0 {
		t.Fatalf("heartbeat restarts = %d, want 0 (reboot failed)", *hbStarts)
	}
}

// TestDoRebootRebootErrorIsNonFatal asserts that if GW_REBOOT_REQ itself
// errors, the function still proceeds to the reconnect loop.
func TestDoRebootRebootErrorIsNonFatal(t *testing.T) {
	a, klf, _, hbStarts := newTestApp(config.Config{})
	withFastReboot(t, 2)
	klf.rebootErr = errors.New("session already gone")

	if err := a.doReboot(context.Background()); err != nil {
		t.Fatalf("doReboot: %v", err)
	}
	if klf.rebooted != 1 {
		t.Fatalf("reboot calls = %d, want 1 (attempted despite error)", klf.rebooted)
	}
	if klf.connected != 1 {
		t.Fatalf("connected = %d, want 1 (reconnect proceeded)", klf.connected)
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
	withFastReboot(t, 1)
	// Make the reconnect keep failing so the gateway stays "wedged".
	klf.connectErr = errors.New("still wedged")

	// Freeze a clock far past the threshold (10s x 2 = 20s window).
	base := time.Unix(1_000_000, 0)
	a.now = func() time.Time { return base }
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

// TestWedgeRecoveryTriggersReboot asserts that on wedge recovery the health
// check first reconnects (doReconnect) and then immediately reboots the
// gateway (doReboot) to clear any zombie session slot, restoring availability
// only after the reboot completes.
func TestWedgeRecoveryTriggersReboot(t *testing.T) {
	cfg := config.Config{}
	cfg.Restart.HealthCheckInterval = 10
	cfg.Restart.RestartOnError = true

	a, klf, mgr, _ := newTestApp(cfg)
	withFastReboot(t, 2)
	klf.connectErr = errors.New("wedged")

	base := time.Unix(2_000_000, 0)
	a.now = func() time.Time { return base }
	a.lastContact.Store(base.Add(-60 * time.Second).UnixNano())

	// Trip the wedge.
	a.checkHealth(context.Background())
	if !a.wedged.Load() {
		t.Fatalf("expected wedged")
	}

	// Gateway recovers: connect now succeeds.
	klf.mu.Lock()
	klf.connectErr = nil
	klf.mu.Unlock()

	a.checkHealth(context.Background())
	if a.wedged.Load() {
		t.Fatalf("expected recovery to clear wedged")
	}
	if klf.rebooted != 1 {
		t.Fatalf("reboot calls = %d, want 1 (reactive reboot after wedge recovery)", klf.rebooted)
	}
	if _, _, avail, _ := mgr.counts(); avail < 1 {
		t.Fatalf("MarkAllAvailable calls = %d, want >=1 after recovery", avail)
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

	if _, _, _, closeAll := mgr.counts(); closeAll != 1 {
		t.Fatalf("CloseAll calls = %d, want 1", closeAll)
	}
	if klf.disconnected != 1 {
		t.Fatalf("Disconnect calls = %d, want 1", klf.disconnected)
	}
	if len(klf.snapshot()) != 1 || klf.snapshot()[0] != "disconnect" {
		t.Fatalf("klf calls on shutdown = %v, want exactly [disconnect]", klf.snapshot())
	}
	if stopHBCalls < 1 {
		t.Fatalf("heartbeat stop calls = %d, want >=1", stopHBCalls)
	}

	// Idempotent.
	a.Stop()
	if _, _, _, closeAll := mgr.counts(); closeAll != 1 {
		t.Fatalf("CloseAll after second Stop = %d, want still 1 (idempotent)", closeAll)
	}
	if klf.disconnected != 1 {
		t.Fatalf("Disconnect after second Stop = %d, want still 1 (idempotent)", klf.disconnected)
	}
}

// ─── power-cycle escalation ──────────────────────────────────────────────────

// recordedPublish is one captured MQTT publish from the power-cycle seam.
type recordedPublish struct {
	topic    string
	payload  string
	retained bool
}

// withPlug configures the power-cycle feature on cfg and installs a recording
// publish seam on a. The returned func snapshots what was published.
//
// Dwell is left at 0s on purpose: ApplyDefaults would set 10, but these tests
// construct config directly, and a zero dwell makes time.After fire immediately
// so the tests do not sleep.
func withPlug(a *App, topic string) func() []recordedPublish {
	var mu sync.Mutex
	var pubs []recordedPublish
	a.cfg.Restart.PowerCycleTopic = topic
	a.cfg.Restart.PowerCycleOffPayload = "OFF"
	a.cfg.Restart.PowerCycleOnPayload = "ON"
	a.publishFn = func(t, p string, retained bool) {
		mu.Lock()
		pubs = append(pubs, recordedPublish{t, p, retained})
		mu.Unlock()
	}
	return func() []recordedPublish {
		mu.Lock()
		defer mu.Unlock()
		out := make([]recordedPublish, len(pubs))
		copy(out, pubs)
		return out
	}
}

// TestPowerCycleEscalationAfterFailedReconnect is the core of the feature: a
// gateway that will not even take a fresh session must be power-cycled, and the
// bridge must come all the way back afterwards (connect, re-register, heartbeat)
// and restore availability.
func TestPowerCycleEscalationAfterFailedReconnect(t *testing.T) {
	cfg := config.Config{}
	cfg.Restart.HealthCheckInterval = 10
	cfg.Restart.RestartOnError = true

	a, klf, mgr, hbStarts := newTestApp(cfg)
	withFastReboot(t, 2)
	pubs := withPlug(a, "hs100/velux_reset/set")

	// The gateway refuses the first connect (so doReconnect fails and we
	// escalate), then answers once power has been cut and restored. Only the
	// counter is set — also setting connectErr would feed the fake's
	// permanent-failure branch, and Connect would never succeed again.
	klf.connectErrsUntil = 1

	base := time.Unix(2_000_000, 0)
	a.now = func() time.Time { return base }
	a.lastContact.Store(base.Add(-60 * time.Second).UnixNano())

	a.checkHealth(context.Background())

	got := pubs()
	if len(got) != 2 {
		t.Fatalf("publishes = %v, want exactly off+on", got)
	}
	if got[0].payload != "OFF" || got[1].payload != "ON" {
		t.Fatalf("payload order = %q,%q, want OFF,ON", got[0].payload, got[1].payload)
	}
	for i, p := range got {
		if p.topic != "hs100/velux_reset/set" {
			t.Errorf("publish[%d] topic = %q", i, p.topic)
		}
		// A retained OFF would strand the plug off across a restart, leaving the
		// gateway dead with nobody to publish the ON.
		if p.retained {
			t.Errorf("publish[%d] (%s) was retained, must not be", i, p.payload)
		}
	}

	if *hbStarts == 0 {
		t.Errorf("heartbeat was not restarted after the power cycle")
	}
	if reg, _, avail, _ := mgr.counts(); reg == 0 || avail == 0 {
		t.Errorf("after power cycle: Register=%d MarkAllAvailable=%d, want both > 0", reg, avail)
	}
	if a.wedged.Load() {
		t.Errorf("still wedged after a successful power cycle")
	}
}

// TestPowerCycleDisabledKeepsManualBehaviour pins the opt-in guarantee: without
// a topic nothing is ever published, whatever the gateway does.
func TestPowerCycleDisabledKeepsManualBehaviour(t *testing.T) {
	cfg := config.Config{}
	cfg.Restart.HealthCheckInterval = 10
	cfg.Restart.RestartOnError = true

	a, klf, _, _ := newTestApp(cfg)
	withFastReboot(t, 1)

	published := 0
	a.publishFn = func(string, string, bool) { published++ }
	klf.connectErr = errors.New("wedged")

	base := time.Unix(2_100_000, 0)
	a.now = func() time.Time { return base }
	a.lastContact.Store(base.Add(-60 * time.Second).UnixNano())

	a.checkHealth(context.Background())

	if published != 0 {
		t.Fatalf("published %d messages with no power-cycle topic, want 0", published)
	}
	if !a.wedged.Load() {
		t.Fatalf("expected the gateway to stay wedged")
	}
}

// TestPowerCycleDebounced asserts the minimum gap: a gateway that never returns
// must not have its mains toggled on every health-check tick.
func TestPowerCycleDebounced(t *testing.T) {
	cfg := config.Config{}
	cfg.Restart.HealthCheckInterval = 10
	cfg.Restart.RestartOnError = true

	a, klf, _, _ := newTestApp(cfg)
	withFastReboot(t, 1)
	pubs := withPlug(a, "plug/set")

	// Never recovers, so every tick reaches the escalation.
	klf.connectErr = errors.New("permanently wedged")

	base := time.Unix(2_200_000, 0)
	a.now = func() time.Time { return base }
	a.lastContact.Store(base.Add(-60 * time.Second).UnixNano())

	a.checkHealth(context.Background())
	afterFirst := len(pubs())
	if afterFirst == 0 {
		t.Fatalf("first wedge did not power-cycle")
	}

	// Second tick one minute later — well inside the 15 min floor.
	a.now = func() time.Time { return base.Add(1 * time.Minute) }
	a.checkHealth(context.Background())
	if got := len(pubs()); got != afterFirst {
		t.Fatalf("publishes = %d after debounced tick, want unchanged %d", got, afterFirst)
	}

	// Past the floor it may fire again.
	a.now = func() time.Time { return base.Add(powerCycleMinGap + time.Minute) }
	a.checkHealth(context.Background())
	if got := len(pubs()); got <= afterFirst {
		t.Fatalf("publishes = %d after the debounce window, want more than %d", got, afterFirst)
	}
}

// TestPowerCycleDebouncedIsNotAFailure documents the sentinel: callers must be
// able to tell "not yet" from "it broke".
func TestPowerCycleDebouncedIsNotAFailure(t *testing.T) {
	cfg := config.Config{}
	a, _, _, _ := newTestApp(cfg)
	withFastReboot(t, 1)
	withPlug(a, "plug/set")

	base := time.Unix(2_300_000, 0)
	a.now = func() time.Time { return base }

	if err := a.doPowerCycle(context.Background()); err != nil {
		t.Fatalf("first power cycle: %v", err)
	}
	err := a.doPowerCycle(context.Background())
	if !errors.Is(err, errPowerCycleDebounced) {
		t.Fatalf("second power cycle error = %v, want errPowerCycleDebounced", err)
	}
}

// TestPowerCycleSerializedWithReboot asserts the new step joins the resetMu
// serialization, so a periodic reboot and an escalation can never drive the
// KLF200 connection at the same time.
func TestPowerCycleSerializedWithReboot(t *testing.T) {
	cfg := config.Config{}
	a, _, _, _ := newTestApp(cfg)
	withFastReboot(t, 1)
	withPlug(a, "plug/set")

	base := time.Unix(2_400_000, 0)
	a.now = func() time.Time { return base }

	var mu sync.Mutex
	inFlight, maxInFlight := 0, 0
	track := func() func() {
		mu.Lock()
		inFlight++
		if inFlight > maxInFlight {
			maxInFlight = inFlight
		}
		mu.Unlock()
		return func() {
			mu.Lock()
			inFlight--
			mu.Unlock()
		}
	}
	// Both paths funnel through stopHeartbeatFn while holding resetMu, which
	// makes it a usable probe for "how many resets are running right now".
	a.stopHeartbeatFn = func() {
		done := track()
		time.Sleep(10 * time.Millisecond)
		done()
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = a.doReboot(context.Background()) }()
	go func() { defer wg.Done(); _ = a.doPowerCycle(context.Background()) }()
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if maxInFlight > 1 {
		t.Fatalf("max concurrent reset paths = %d, want 1 (resetMu not held?)", maxInFlight)
	}
}

// TestPowerCycleRestoresPowerOnCancel guards the worst outcome of all: shutting
// down mid-cycle must never leave the gateway without power.
func TestPowerCycleRestoresPowerOnCancel(t *testing.T) {
	cfg := config.Config{}
	cfg.Restart.PowerCycleOffSeconds = 60 // long enough that cancel lands first

	a, _, _, _ := newTestApp(cfg)
	withFastReboot(t, 1)
	pubs := withPlug(a, "plug/set")
	a.cfg.Restart.PowerCycleOffSeconds = 60

	base := time.Unix(2_500_000, 0)
	a.now = func() time.Time { return base }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := a.doPowerCycle(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("doPowerCycle error = %v, want context.Canceled", err)
	}

	got := pubs()
	if len(got) < 2 || got[len(got)-1].payload != "ON" {
		t.Fatalf("publishes = %v, want the last one to restore power (ON)", got)
	}
}
