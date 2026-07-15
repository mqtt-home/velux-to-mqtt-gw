package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/bridge"
	"github.com/mqtt-home/velux-to-mqtt-gw/config"
	"github.com/mqtt-home/velux-to-mqtt-gw/klf200"
	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
	"github.com/philipparndt/go-logger"
)

// healthCheckFailureThreshold is the multiplier applied to the health-check
// interval before the gateway is considered wedged. Matches the Python bridge's
// HEALTH_CHECK_FAILURE_THRESHOLD (2.0).
const healthCheckFailureThreshold = 2.0

// rebootBackoffSchedule is the wait-between-attempts schedule for reconnecting
// to the gateway after a device reboot. Attempts are made after each duration
// in the slice in order; total wall-clock is the sum (~5 min). A var so tests
// can shrink it to zero-duration slots to avoid real sleeps.
var rebootBackoffSchedule = []time.Duration{
	2 * time.Second,
	5 * time.Second,
	10 * time.Second,
	30 * time.Second,
	60 * time.Second,
	90 * time.Second,
	120 * time.Second,
}

// klfClient is the seam over the concrete *klf200.Client the App drives. It
// covers the whole connect/authenticate/reload lifecycle plus the accessors the
// heartbeat and node-updater wiring need. A fake implementing this interface
// lets the reset/health logic be unit-tested without hardware.
type klfClient interface {
	Connect(ctx context.Context) error
	Disconnect() error
	Connected() bool
	PasswordEnter(ctx context.Context, password string) error
	SetUTC(ctx context.Context, t time.Time) error
	LoadNodes(ctx context.Context) error
	Reboot(ctx context.Context) error
}

// coverManager is the seam over *bridge.Manager. Register re-runs discovery +
// callbacks + initial publish for the current node set; MarkAllUnavailable and
// MarkAllAvailable drive the wedge/recovery availability transitions.
type coverManager interface {
	Register(ctx context.Context) error
	MarkAllUnavailable()
	MarkAllAvailable()
	// CloseAll marks covers offline and clears HA discovery, run on shutdown.
	CloseAll()
}

// App wires config -> klf200 client -> bridge manager and owns the background
// loops (heartbeat, health check, periodic reboot). It is the Go counterpart of
// the Python VeluxMqttHomeassistant plus its asyncio task set, but with the
// process-exit restart replaced by an in-process clean session reset.
type App struct {
	cfg config.Config

	// client and mgr are the concrete production wiring, kept so Start can build
	// the heartbeat/node-updater and re-register nodes after a reset. They are
	// also exposed through the klf/mgr seams for testability.
	client *klf200.Client
	mgr    *bridge.Manager

	// klf and mgr seams (default to client/mgr; overridable in tests).
	klf klfClient
	cm  coverManager

	heartbeat *klf200.Heartbeat
	updater   *klf200.NodeUpdater

	// startHeartbeatFn / stopHeartbeatFn are the heartbeat-lifecycle seams. They
	// default to the real klf200.Heartbeat wiring (see newApp); tests override
	// them so the reset/health logic can be exercised without a live client whose
	// pulse would dereference a real connection.
	startHeartbeatFn func()
	stopHeartbeatFn  func()

	// now is the clock seam so health checks can be unit-tested without real time.
	now func() time.Time

	// lastContact holds the UnixNano of the most recent successful KLF200 contact
	// (heartbeat confirmation or node update). Stamped atomically from multiple
	// goroutines; read by the health check.
	lastContact atomic.Int64

	// wedged tracks whether the gateway is currently considered unreachable, so
	// the health check only logs the transition once and restores on recovery.
	wedged atomic.Bool

	// resetMu serializes the reboot/reconnect paths (startup reboot, periodic
	// reboot loop, and the health-check wedge-recovery reconnect+reboot) so
	// they never race each other on the KLF200 connection.
	resetMu sync.Mutex

	mu       sync.Mutex
	stopCh   chan struct{}
	loopsWG  sync.WaitGroup
	stopOnce sync.Once
}

// newApp constructs an App from config, building the concrete klf200 client and
// bridge manager. MQTT must already be connected (main calls mqtt.Start before
// building the App) so the manager's publishes land on a live connection.
func newApp(cfg config.Config, client *klf200.Client, mgr *bridge.Manager) *App {
	a := &App{
		cfg:    cfg,
		client: client,
		mgr:    mgr,
		klf:    client,
		cm:     mgr,
		now:    time.Now,
	}
	a.startHeartbeatFn = a.startHeartbeat
	a.stopHeartbeatFn = a.stopHeartbeat
	a.updater = klf200.NewNodeUpdater(client)
	// Register the node updater and a contact stamper ONCE on the client. The
	// client keeps its processor list across reconnects (only the connection-level
	// dispatch is re-installed on each Connect), so these must not be re-registered
	// per reset or they would accumulate duplicates. Every inbound frame counts as
	// KLF200 contact, matching the Python record_klf_contact in vlxnode_callback.
	a.updater.Register()
	client.RegisterNodeUpdater(func(_ protocol.Frame) { a.recordContact() })
	return a
}

// Start performs the ordered startup: connect + authenticate KLF200, load nodes,
// register the node updater, register covers (discovery + initial state), start
// the heartbeat, then launch the contact-tracking health-check and periodic
// reset loops. It returns an error if the initial KLF200 bring-up fails so main
// can exit non-zero. It does not block.
func (a *App) Start(ctx context.Context) error {
	a.stopCh = make(chan struct{})

	// Stamp initial contact so the health check does not trip before the first
	// heartbeat has had a chance to run.
	a.recordContact()

	if err := a.connectAndLoad(ctx); err != nil {
		return err
	}

	// Register covers: discovery + subscriptions + initial publish.
	if err := a.cm.Register(ctx); err != nil {
		return err
	}

	// Heartbeat: keepalive + liveness. Its confirmations stamp lastContact.
	a.startHeartbeatFn()

	// Startup reboot: clear any zombie session slots left by a prior unclean
	// shutdown. KLF200 has only 2 API slots and does not garbage-collect
	// abandoned ones, so a fresh startup can otherwise inherit a wedged state
	// that only a device reboot can recover from.
	logger.Info("Rebooting KLF200 on startup to clear any zombie session slots")
	if err := a.doReboot(ctx); err != nil {
		return fmt.Errorf("startup reboot: %w", err)
	}

	// Background loops.
	a.startHealthCheck()
	a.startRebootLoop()

	logger.Info("Application is now ready")
	return nil
}

// connectAndLoad runs the full connect + authenticate + load sequence and wires
// the node updater so live notifications flow into the node model. It is used by
// both Start and doReset. Ported from PyVLX.connect + load_nodes: connect the
// transport, GW_PASSWORD_ENTER, set the gateway clock, enable house-status
// monitoring, then load all nodes. The node updater is (re)registered on the
// client so inbound notifications mutate nodes; the per-node device-updated
// callbacks are wired by the bridge covers during Register.
func (a *App) connectAndLoad(ctx context.Context) error {
	logger.Info("Connecting to KLF200", "host", a.cfg.Velux.Host)
	if err := a.klf.Connect(ctx); err != nil {
		return err
	}
	if err := a.klf.PasswordEnter(ctx, a.cfg.Velux.Password); err != nil {
		return err
	}
	// Best-effort clock sync (matches pyvlx connect); non-fatal on failure.
	if err := a.klf.SetUTC(ctx, a.now()); err != nil {
		logger.Warn("KLF200 set UTC failed (continuing)", "error", err)
	}
	// Enable house-status monitoring so unsolicited position changes arrive.
	// Skipped when the seam is a test fake (no real client).
	if a.client != nil {
		if err := klf200.HouseStatusMonitorEnable(ctx, a.client); err != nil {
			logger.Warn("KLF200 house-status monitor enable failed (continuing)", "error", err)
		}
	}
	if err := a.klf.LoadNodes(ctx); err != nil {
		return err
	}
	// The node updater + contact stamper are registered once at construction and
	// persist across reconnects (the client re-installs its dispatch on Connect),
	// so nothing to re-register here.
	a.recordContact()
	logger.Info("KLF200 connected and nodes loaded")
	return nil
}

// startHeartbeat builds and starts the heartbeat, and launches a goroutine that
// stamps lastContact on each pulse and surfaces failures. The heartbeat's
// GetState confirmation is the primary liveness signal.
func (a *App) startHeartbeat() {
	a.heartbeat = klf200.NewHeartbeat(a.client)
	a.heartbeat.Start()

	a.loopsWG.Add(1)
	go func() {
		defer a.loopsWG.Done()
		for {
			select {
			case <-a.stopCh:
				return
			case err, ok := <-a.heartbeat.FailureCh:
				if !ok {
					return
				}
				logger.Warn("KLF200 heartbeat failed", "error", err)
			}
		}
	}()
}

// stopHeartbeat stops and clears the current heartbeat if one is running. It is
// the counterpart of startHeartbeat and the default stopHeartbeatFn seam.
func (a *App) stopHeartbeat() {
	if a.heartbeat != nil {
		a.heartbeat.Stop()
		a.heartbeat = nil
	}
}

// startHealthCheck launches the wedge-detection loop. Disabled when the interval
// is 0. On each tick it evaluates checkHealth against the current clock.
func (a *App) startHealthCheck() {
	interval := time.Duration(a.cfg.Restart.HealthCheckInterval) * time.Second
	if interval <= 0 {
		logger.Info("Health check disabled")
		return
	}
	logger.Info("Health check enabled", "interval", interval)

	a.loopsWG.Add(1)
	go func() {
		defer a.loopsWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				a.checkHealth(context.Background())
			}
		}
	}()
}

// startRebootLoop launches the periodic device-reboot loop. Disabled when the
// interval is 0. Each tick sends GW_REBOOT_REQ and reconnects with backoff,
// freeing both KLF200 session slots (ours and any zombie).
func (a *App) startRebootLoop() {
	interval := time.Duration(a.cfg.Restart.RestartInterval) * time.Hour
	if interval <= 0 {
		logger.Info("Periodic reboot disabled")
		return
	}
	logger.Info("Periodic reboot enabled", "interval", interval)

	a.loopsWG.Add(1)
	go func() {
		defer a.loopsWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-a.stopCh:
				return
			case <-ticker.C:
				logger.Info("Periodic reboot triggered")
				if err := a.doReboot(context.Background()); err != nil {
					logger.Error("Periodic reboot failed", "error", err)
				}
			}
		}
	}()
}

// recordContact stamps the current time as the last successful KLF200 contact.
// Ported from record_klf_contact; called on heartbeat success and node updates.
func (a *App) recordContact() {
	a.lastContact.Store(a.now().UnixNano())
}

// lastContactTime returns the time of the most recent successful KLF200 contact.
func (a *App) lastContactTime() time.Time {
	return time.Unix(0, a.lastContact.Load())
}

// checkHealth evaluates whether the KLF200 has gone silent longer than
// health-check-interval x threshold. On a trip it marks all covers unavailable
// and logs an actionable message, then attempts a clean reconnect so recovery is
// automatic once the operator power-cycles the gateway. On recovery it restores
// availability. Reactive reconnection is gated on Restart.RestartOnError,
// matching the Python health_check_task. This method is the injectable loop body
// (uses a.now) so it can be unit-tested without real timers.
func (a *App) checkHealth(ctx context.Context) {
	interval := time.Duration(a.cfg.Restart.HealthCheckInterval) * time.Second
	if interval <= 0 {
		return
	}
	threshold := time.Duration(float64(interval) * healthCheckFailureThreshold)
	sinceContact := a.now().Sub(a.lastContactTime())

	if sinceContact <= threshold {
		// Healthy. If we were previously wedged, restore availability.
		if a.wedged.CompareAndSwap(true, false) {
			logger.Info("KLF200 contact restored", "since_contact", sinceContact)
			a.cm.MarkAllAvailable()
		}
		return
	}

	// Wedged: no contact within the threshold.
	if a.wedged.CompareAndSwap(false, true) {
		logger.Error("KLF200 unreachable - manual power-cycle required",
			"since_contact", sinceContact, "threshold", threshold)
		a.cm.MarkAllUnavailable()
	} else {
		logger.Warn("KLF200 still unreachable - waiting for manual power-cycle",
			"since_contact", sinceContact)
	}

	if !a.cfg.Restart.RestartOnError {
		return
	}

	// Keep retrying reconnect visibly. A successful reset restores availability
	// (and clears the wedged flag) via the healthy branch on the next tick.
	logger.Info("Attempting KLF200 reconnect after wedge")
	if err := a.doReconnect(ctx); err != nil {
		logger.Warn("KLF200 reconnect attempt failed", "error", err)
		return
	}

	// Reconnected — but the wedge may have been caused by a zombie session slot
	// the reconnect itself cannot free. Reboot the device to guarantee a clean
	// slate before declaring recovery.
	logger.Info("KLF200 reconnected, rebooting to clear zombie session slots")
	if err := a.doReboot(ctx); err != nil {
		logger.Warn("KLF200 post-recovery reboot failed", "error", err)
		return
	}

	if a.wedged.CompareAndSwap(true, false) {
		logger.Info("KLF200 recovered after wedge")
		a.cm.MarkAllAvailable()
	}
}

// doReconnect performs an in-process reconnect without rebooting the gateway:
// stop the heartbeat, clean-disconnect (freeing our slot), reconnect +
// re-authenticate + reload nodes + re-register callbacks + restart heartbeat.
// The MQTT connection stays up so HA entities do not flap. Used by the
// health-check wedge-recovery path where we first need a live session before
// we can even ask the gateway to reboot. Serialized via resetMu.
func (a *App) doReconnect(ctx context.Context) error {
	a.resetMu.Lock()
	defer a.resetMu.Unlock()

	// Pause the heartbeat so it does not race the disconnect/reconnect.
	a.stopHeartbeatFn()

	// Clean-disconnect any half-open state.
	logger.Info("KLF200 clean disconnect (releasing session slot)")
	if err := a.klf.Disconnect(); err != nil {
		logger.Warn("KLF200 disconnect returned error (continuing)", "error", err)
	}

	// Reconnect + re-authenticate + reload nodes + re-register node updater.
	if err := a.connectAndLoad(ctx); err != nil {
		return err
	}

	// Re-register covers: re-publish discovery + re-wire per-node callbacks +
	// re-publish initial state. MQTT stayed up so HA entities do not flap.
	if err := a.cm.Register(ctx); err != nil {
		return err
	}

	// Restart the heartbeat against the fresh connection.
	a.startHeartbeatFn()

	logger.Info("KLF200 reconnect complete")
	return nil
}

// doReboot performs a device-level KLF200 reboot via GW_REBOOT_REQ. The gateway
// drops the TCP session as part of the reboot; we then reconnect with an
// exponential-backoff schedule (rebootBackoffSchedule) until the gateway is
// reachable again, and re-run the full connect/load/register/heartbeat
// sequence. This is the ONLY path that frees a zombie session slot the KLF200
// forgot to expire — a plain reconnect cannot. Serialized via resetMu.
//
// Callers: startup (clear inherited zombies), periodic loop (prophylactic
// daily reboot), and the health-check wedge-recovery branch (belt-and-braces
// after a successful reconnect).
func (a *App) doReboot(ctx context.Context) error {
	a.resetMu.Lock()
	defer a.resetMu.Unlock()

	// Pause the heartbeat so it does not race the reboot/reconnect.
	a.stopHeartbeatFn()

	// Send GW_REBOOT_REQ. If the session is already gone (e.g. we were called
	// from a wedge-recovery path where Reconnect just ran but the connection
	// dropped again in between), the send may fail — proceed to the reconnect
	// loop anyway; the gateway may have rebooted or may just need a fresh
	// session.
	logger.Info("KLF200 reboot requested (GW_REBOOT_REQ)")
	if err := a.klf.Reboot(ctx); err != nil {
		logger.Warn("KLF200 reboot request returned error (continuing to reconnect)", "error", err)
	}

	// The gateway drops the TCP session as part of the reboot; clean up our
	// local Client state (unregister callbacks, close socket) so the following
	// Connect starts from a known-clean slate.
	if err := a.klf.Disconnect(); err != nil {
		logger.Debug("KLF200 disconnect after reboot returned error (expected)", "error", err)
	}

	// Reconnect with exponential backoff. The KLF200 typically takes 60–90s to
	// come back; the schedule covers that with headroom.
	var lastErr error
	for i, wait := range rebootBackoffSchedule {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return ctx.Err()
		}

		if err := a.connectAndLoad(ctx); err != nil {
			lastErr = err
			logger.Info("KLF200 post-reboot reconnect attempt failed",
				"attempt", i+1, "of", len(rebootBackoffSchedule), "error", err)
			// Best-effort local cleanup before the next attempt.
			_ = a.klf.Disconnect()
			continue
		}

		if err := a.cm.Register(ctx); err != nil {
			return fmt.Errorf("post-reboot register: %w", err)
		}
		a.startHeartbeatFn()
		logger.Info("KLF200 reboot complete, reconnected", "attempt", i+1)
		return nil
	}

	return fmt.Errorf("KLF200 post-reboot reconnect exhausted %d attempts: %w",
		len(rebootBackoffSchedule), lastErr)
}

// Stop tears down the background loops, clears HA discovery, performs a CLEAN
// KLF200 disconnect (releasing the session slot), and returns. It never calls
// os.Exit before disconnecting cleanly — an unclean exit would create a zombie
// slot. MQTT is disconnected by main after Stop returns. Safe to call once.
func (a *App) Stop() {
	a.stopOnce.Do(func() {
		a.mu.Lock()
		if a.stopCh != nil {
			close(a.stopCh)
		}
		a.mu.Unlock()

		// Stop the heartbeat before waiting on the loops.
		a.resetMu.Lock()
		a.stopHeartbeatFn()
		a.resetMu.Unlock()

		// Wait for the health-check, reset, and heartbeat-failure goroutines.
		a.loopsWG.Wait()

		// Clear HA discovery + mark covers offline, then clean-disconnect KLF200.
		a.cm.CloseAll()

		logger.Info("KLF200 clean disconnect on shutdown")
		if err := a.klf.Disconnect(); err != nil {
			logger.Warn("KLF200 disconnect on shutdown returned error", "error", err)
		}
	})
}
