package bridge

import (
	"context"
	"sync"

	"github.com/mqtt-home/velux-to-mqtt-gw/config"
	"github.com/mqtt-home/velux-to-mqtt-gw/klf200"
	"github.com/philipparndt/go-logger"
)

// discoveredTopics tracks every HA discovery config topic published this run so
// CleanupDiscovery can clear them (empty retained payload) on graceful
// shutdown. Guarded by discoveredMu.
var (
	discoveredMu     sync.Mutex
	discoveredTopics = map[string]struct{}{}
)

// recordDiscoveryTopic remembers a published discovery topic for later cleanup.
func recordDiscoveryTopic(topic string) {
	discoveredMu.Lock()
	discoveredTopics[topic] = struct{}{}
	discoveredMu.Unlock()
}

// CleanupDiscovery clears every discovery topic recorded this run by publishing
// an empty retained payload, so Home Assistant removes the devices on graceful
// shutdown. Safe to call when nothing was published (the set is empty).
func CleanupDiscovery(m *MQTT) {
	discoveredMu.Lock()
	topics := make([]string, 0, len(discoveredTopics))
	for t := range discoveredTopics {
		topics = append(topics, t)
	}
	discoveredTopics = map[string]struct{}{}
	discoveredMu.Unlock()

	for _, t := range topics {
		m.Publish(t, "", true)
	}
	if len(topics) > 0 {
		logger.Info("[bridge] cleared discovery topics", "count", len(topics))
	}
}

// Manager is the top-level bridge coordinator, the Go counterpart of the Python
// VeluxMqttHomeassistant. It iterates the KLF200 client's nodes, wraps every
// opening device in a Cover, wires MQTT command/state plumbing, and owns the
// discovery-cleanup lifecycle.
type Manager struct {
	cfg    config.Config
	client *klf200.Client
	mqtt   *MQTT

	// covers is the registered set, keyed by generated id (mirrors Python's
	// mqttDevices dict).
	covers map[string]*Cover
}

// NewManager builds a Manager for the given config, KLF200 client, and MQTT
// wrapper. It does not register anything yet; call Register.
func NewManager(cfg config.Config, client *klf200.Client, m *MQTT) *Manager {
	return &Manager{
		cfg:    cfg,
		client: client,
		mqtt:   m,
		covers: make(map[string]*Cover),
	}
}

// Register iterates every KLF200 node, wraps each opening device in a Cover,
// and starts it (discovery + subscriptions + initial publish). Ported from
// VeluxMqttHomeassistant.register_devices: awnings are inverted only when
// HomeAssistant.InvertAwning is set. Non-opening nodes are skipped.
func (mgr *Manager) Register(ctx context.Context) error {
	for _, node := range mgr.client.Nodes().All() {
		opener, ok := node.(Opener)
		if !ok {
			// Not an opening device (e.g. a switch or light) — skip, matching
			// the Python isinstance(vlxnode, OpeningDevice) guard.
			continue
		}

		inverted := false
		if _, isAwning := node.(*klf200.Awning); isAwning && mgr.cfg.HomeAssistant.InvertAwning {
			inverted = true
		}

		cover := NewCover(opener, mgr.mqtt, mgr.cfg.HomeAssistant.Prefix, inverted)
		if err := cover.Start(ctx); err != nil {
			logger.Error("[bridge] failed to start cover", "node", node.Name(), "error", err)
			return err
		}
		mgr.covers[cover.ID()] = cover
		logger.Info("[bridge] registered cover", "node", node.Name(), "id", cover.ID(), "inverted", inverted)
	}
	logger.Info("[bridge] registration complete", "covers", len(mgr.covers))
	return nil
}

// CloseAll marks every cover offline and clears the HA discovery topics, so a
// graceful shutdown removes the devices from Home Assistant. Ported from the
// Python close() path (publishing availability + tearing down discovery).
func (mgr *Manager) CloseAll() {
	for _, cover := range mgr.covers {
		cover.PublishAvailability(false)
	}
	CleanupDiscovery(mgr.mqtt)
}

// Covers returns the registered covers keyed by generated id, for callers that
// need to drive periodic state refreshes (the Python state_update_task loop).
func (mgr *Manager) Covers() map[string]*Cover {
	return mgr.covers
}

// MarkAllUnavailable publishes offline availability for every registered cover.
// Used by the app-runtime wedge handler to make a lost KLF200 visible in Home
// Assistant without tearing down discovery (unlike CloseAll, this is reversible
// via MarkAllAvailable once the gateway recovers).
func (mgr *Manager) MarkAllUnavailable() {
	for _, cover := range mgr.covers {
		cover.PublishAvailability(false)
	}
	logger.Info("[bridge] marked all covers unavailable", "covers", len(mgr.covers))
}

// MarkAllAvailable publishes online availability for every registered cover.
// Used by the app-runtime wedge handler to restore covers after the KLF200
// becomes reachable again.
func (mgr *Manager) MarkAllAvailable() {
	for _, cover := range mgr.covers {
		cover.PublishAvailability(true)
	}
	logger.Info("[bridge] marked all covers available", "covers", len(mgr.covers))
}
