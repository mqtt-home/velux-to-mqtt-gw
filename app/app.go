package main

import (
	"context"
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

// resetReconnectDelay is how long we wait between the clean disconnect and the
// reconnect during a periodic reset, giving the KLF200 time to release the slot.
// A var (not const) so tests can zero it to avoid a real sleep.
var resetReconnectDelay = 2 * time.Second

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
// loops (heartbeat, health check, periodic reset). It is the Go counterpart of
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

	// resetMu serializes a periodic reset against a health-check-driven reconnect
	// so the two never disconnect/reconnect concurrently.
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

	// Background loops.
	a.startHealthCheck()
	a.startResetLoop()

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

// startResetLoop launches the periodic clean-reset loop. Disabled when the
// interval is 0. Each tick performs an in-process clean disconnect/reconnect.
func (a *App) startResetLoop() {
	interval := time.Duration(a.cfg.Restart.RestartInterval) * time.Hour
	if interval <= 0 {
		logger.Info("Periodic reset disabled")
		return
	}
	logger.Info("Periodic reset enabled", "interval", interval)

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
				logger.Info("Periodic reset triggered")
				if err := a.doReset(context.Background()); err != nil {
					logger.Error("Periodic reset failed", "error", err)
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
	if err := a.doReset(ctx); err != nil {
		logger.Warn("KLF200 reconnect attempt failed", "error", err)
		return
	}
	if a.wedged.CompareAndSwap(true, false) {
		logger.Info("KLF200 reconnected after wedge")
		a.cm.MarkAllAvailable()
	}
}

// doReset performs an in-process CLEAN session reset: stop the heartbeat, cleanly
// disconnect from the KLF200 (releasing the API session slot), wait briefly,
// reconnect + re-authenticate + reload nodes + re-register callbacks, re-publish
// discovery/state, and restart the heartbeat. The MQTT connection stays up the
// whole time so HA entities do not flap. This replaces the Python bridge's
// process-exit restart. It is the injectable body used by both the periodic
// reset loop and the health-check reconnect. Serialized via resetMu.
func (a *App) doReset(ctx context.Context) error {
	a.resetMu.Lock()
	defer a.resetMu.Unlock()

	// Pause the heartbeat so it does not race the disconnect/reconnect.
	a.stopHeartbeatFn()

	// CLEAN disconnect: this is the active ingredient that frees the slot.
	logger.Info("KLF200 clean disconnect (releasing session slot)")
	if err := a.klf.Disconnect(); err != nil {
		logger.Warn("KLF200 disconnect returned error (continuing)", "error", err)
	}

	// Give the gateway a moment to release the slot before reacquiring.
	select {
	case <-time.After(resetReconnectDelay):
	case <-ctx.Done():
		return ctx.Err()
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

	logger.Info("KLF200 session reset complete")
	return nil
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
