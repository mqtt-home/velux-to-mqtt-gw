package bridge

import (
	"github.com/mqtt-home/velux-to-mqtt-gw/config"
	gwconfig "github.com/philipparndt/mqtt-gateway/config"
	"github.com/philipparndt/mqtt-gateway/mqtt"
)

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

	// Seams (default to the mqtt-gateway implementations; overridable in tests).
	startFn     func(cfg gwconfig.MQTTConfig, clientIDPrefix string)
	publishFn   func(topic string, message any, retained bool)
	subscribeFn func(topic string, onMessage mqtt.OnMessageListener)
}

// NewMQTT builds an MQTT wrapper honoring the config's retain flag. It does not
// connect; call Connect.
func NewMQTT(cfg config.Config) *MQTT {
	return &MQTT{
		retain:      cfg.MQTT.Retain,
		startFn:     mqtt.Start,
		publishFn:   mqtt.PublishAbsolute,
		subscribeFn: mqtt.Subscribe,
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
	m.publishFn(topic, payload, m.retain)
}

// Publish publishes with an explicit retain flag, for the rare cases (e.g.
// clearing a discovery topic with an empty retained payload) where the caller
// needs to override the default.
func (m *MQTT) Publish(topic, payload string, retained bool) {
	m.publishFn(topic, payload, retained)
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
