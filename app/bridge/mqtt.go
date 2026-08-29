package bridge

import (
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/config"
	"github.com/philipparndt/go-logger"
	gwconfig "github.com/philipparndt/mqtt-gateway/config"
	"github.com/philipparndt/mqtt-gateway/mqtt"
)

// DefaultPublishTimeout bounds how long a single publish may block the caller.
//
// mqtt-gateway's PublishAbsolute ends in paho's token.Wait(), which has NO
// timeout: at QoS 1 the token only completes once the broker's PUBACK arrives,
// so while the connection is down the call blocks for as long as the outage
// lasts. Every publish here is fire-and-forget from the caller's point of view —
// cover state, availability, discovery, and the power-cycle plug command — so
// none of them is worth blocking a caller on.
//
// That distinction is not academic. On 2026-08-29 the KLF200 wedged, the health
// check tripped, and its first availability publish landed in that unbounded
// Wait() while MQTT was down. The health-check goroutine — the very goroutine
// that would have reconnected, rebooted, and finally power-cycled the gateway —
// stopped there and never ran again. The roof windows stayed dead until a human
// restarted the pod: the recovery ladder could not run because it was parked on
// a cosmetic publish. Five seconds is far longer than a healthy publish needs
// and far shorter than any outage worth stalling recovery for.
const DefaultPublishTimeout = 5 * time.Second

// MQTT is the thin application-side MQTT wrapper the bridge uses. It sits on
// top of github.com/philipparndt/mqtt-gateway (which internally owns the paho
// client, connection retry, and resubscription-on-reconnect) and exposes only
// the three operations the bridge needs: connect, publish retained, subscribe
// to a command topic.
//
// The wrapper indirects through function-valued fields so unit tests can swap
// in stubs without standing up a real broker, following the miele publisher's
// publishAbsolute seam.
type MQTT struct {
	retain bool

	// publishTimeout bounds a single publish (see DefaultPublishTimeout). Zero
	// disables the bound and publishes straight through, which is what the
	// tests that assert on publish ordering want.
	publishTimeout time.Duration

	// Seams (default to the mqtt-gateway implementations; overridable in tests).
	startFn     func(cfg gwconfig.MQTTConfig, clientIDPrefix string)
	publishFn   func(topic string, message any, retained bool)
	subscribeFn func(topic string, onMessage mqtt.OnMessageListener)
}

// NewMQTT builds an MQTT wrapper honoring the config's retain flag. It does not
// connect; call Connect.
func NewMQTT(cfg config.Config) *MQTT {
	return &MQTT{
		retain:         cfg.MQTT.Retain,
		publishTimeout: DefaultPublishTimeout,
		startFn:        mqtt.Start,
		publishFn:      mqtt.PublishAbsolute,
		subscribeFn:    mqtt.Subscribe,
	}
}

// publish is the single choke point every publish goes through. It runs the
// underlying publish on its own goroutine and gives up waiting after
// publishTimeout, so a broker outage delays a caller by seconds instead of
// parking it indefinitely.
//
// The orphaned goroutine is deliberate and bounded in practice: it finishes as
// soon as paho drains the message (immediately on reconnect), and the callers
// that can fire during an outage are the health check's four availability
// publishes every five minutes plus the power-cycle pair. It is the lesser evil
// against blocking the recovery path — a leaked goroutine costs a few KB, a
// blocked health check costs every cover in the house.
func (m *MQTT) publish(topic string, payload string, retained bool) {
	if m.publishTimeout <= 0 {
		m.publishFn(topic, payload, retained)
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		m.publishFn(topic, payload, retained)
	}()

	timer := time.NewTimer(m.publishTimeout)
	defer timer.Stop()

	select {
	case <-done:
	case <-timer.C:
		// Not an error for the caller: the publish may still land when the
		// broker returns. Worth an operator-visible line, because a burst of
		// these means MQTT is down and retained state is drifting stale.
		logger.Warn("MQTT publish did not complete in time - continuing without waiting",
			"topic", topic, "timeout", m.publishTimeout)
	}
}

// Connect starts the underlying mqtt-gateway client with the given config and
// client-id prefix. mqtt-gateway manages connection retry/backoff internally
// and blocks until the initial connection succeeds. The client-id prefix falls
// back to cfg.MQTT.ClientID when non-empty, else a caller-provided default.
func (m *MQTT) Connect(cfg config.Config, clientIDPrefix string) {
	if cfg.MQTT.ClientID != "" {
		clientIDPrefix = cfg.MQTT.ClientID
	}
	m.startFn(cfg.MQTT.ToGatewayConfig(), clientIDPrefix)
}

// PublishRetained publishes payload to an absolute topic with the configured
// retain flag. Used for state, position, availability, keep-open state, and HA
// discovery configs — all of which the Python bridge published retained.
func (m *MQTT) PublishRetained(topic, payload string) {
	m.publish(topic, payload, m.retain)
}

// Publish publishes with an explicit retain flag, for the rare cases (e.g.
// clearing a discovery topic with an empty retained payload) where the caller
// needs to override the default.
func (m *MQTT) Publish(topic, payload string, retained bool) {
	m.publish(topic, payload, retained)
}

// Subscribe registers a handler for an absolute command topic. The handler
// receives only the decoded string payload; the topic is fixed per
// subscription so callers rarely need it. Ported behavior mirrors the Python
// message_callback_add per command topic.
func (m *MQTT) Subscribe(topic string, handler func(payload string)) {
	m.subscribeFn(topic, func(_ string, payload []byte) {
		handler(string(payload))
	})
}
