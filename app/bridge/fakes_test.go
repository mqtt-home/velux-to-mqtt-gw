package bridge

import (
	"context"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200"
	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
	gwconfig "github.com/philipparndt/mqtt-gateway/config"
	"github.com/philipparndt/mqtt-gateway/mqtt"
)

// publishedMessage records one publish that passed through the fake MQTT seam.
type publishedMessage struct {
	topic    string
	payload  string
	retained bool
}

// subscription records one subscribe that passed through the fake MQTT seam,
// keeping the handler so tests can deliver synthetic command payloads.
type subscription struct {
	topic   string
	handler func(topic string, payload []byte)
}

// newFakeMQTT builds an *MQTT whose seams capture publishes and subscriptions
// into the returned recorder, without touching a real broker. retain is the
// configured retain flag (as NewMQTT would set from cfg.MQTT.Retain).
func newFakeMQTT(retain bool) (*MQTT, *mqttRecorder) {
	rec := &mqttRecorder{}
	m := &MQTT{
		retain:    retain,
		startFn:   func(_ gwconfig.MQTTConfig, _ string) {},
		publishFn: rec.publish,
		subscribeFn: func(topic string, onMessage mqtt.OnMessageListener) {
			rec.subs = append(rec.subs, subscription{topic: topic, handler: onMessage})
		},
	}
	return m, rec
}

// mqttRecorder captures the effects of the bridge on MQTT.
type mqttRecorder struct {
	published []publishedMessage
	subs      []subscription
}

func (r *mqttRecorder) publish(topic string, message any, retained bool) {
	payload, _ := message.(string)
	r.published = append(r.published, publishedMessage{topic: topic, payload: payload, retained: retained})
}

// last returns the most recent payload published to topic, or ("", false).
func (r *mqttRecorder) last(topic string) (string, bool) {
	for i := len(r.published) - 1; i >= 0; i-- {
		if r.published[i].topic == topic {
			return r.published[i].payload, true
		}
	}
	return "", false
}

// deliver finds the subscription for topic and invokes its handler with payload.
// Returns false if no handler is registered for topic.
func (r *mqttRecorder) deliver(topic, payload string) bool {
	for _, s := range r.subs {
		if s.topic == topic {
			s.handler(topic, []byte(payload))
			return true
		}
	}
	return false
}

// subscribed reports whether a subscription exists for topic.
func (r *mqttRecorder) subscribed(topic string) bool {
	for _, s := range r.subs {
		if s.topic == topic {
			return true
		}
	}
	return false
}

// mustPercentPosition builds a protocol.Position from a 0..100 percentage,
// panicking on error (only used with in-range test inputs).
func mustPercentPosition(pct int) protocol.Position {
	p, err := protocol.NewPosition(nil, nil, &pct)
	if err != nil {
		panic(err)
	}
	return p
}

// fakeOpener is a test double satisfying the bridge Opener interface. It records
// which command methods were called (and with what) and returns configurable
// positions / limitation, so the pure command-mapping and state-publishing logic
// can be exercised without a KLF200 gateway.
type fakeOpener struct {
	name string

	position protocol.Position
	target   protocol.Position
	limitMax protocol.Position

	// Recorded calls.
	openCalls        int
	closeCalls       int
	stopCalls        int
	setPositions     []int // percentages passed to SetPosition
	setLimits        [][2]int
	clearLimitCalls  int
	getLimitCalls    int
	updatedCallbacks []klf200.DeviceUpdatedCallback
}

// newFakeOpener builds a fakeOpener with the given name and an UNKNOWN
// limitation max (so the keep-open switch defaults to off unless overridden).
func newFakeOpener(name string) *fakeOpener {
	return &fakeOpener{
		name:     name,
		position: protocol.NewUnknownPosition(),
		target:   protocol.NewUnknownPosition(),
		limitMax: protocol.NewUnknownPosition(),
	}
}

// --- klf200.Node ---

func (f *fakeOpener) NodeID() uint16        { return 0 }
func (f *fakeOpener) Name() string          { return f.name }
func (f *fakeOpener) SerialNumber() [8]byte { return [8]byte{} }
func (f *fakeOpener) AfterUpdate()          {}
func (f *fakeOpener) String() string        { return "fakeOpener(" + f.name + ")" }
func (f *fakeOpener) RegisterDeviceUpdatedCB(cb klf200.DeviceUpdatedCallback) {
	f.updatedCallbacks = append(f.updatedCallbacks, cb)
}

// --- accessors ---

func (f *fakeOpener) Position() protocol.Position       { return f.position }
func (f *fakeOpener) TargetPosition() protocol.Position { return f.target }
func (f *fakeOpener) LimitationMax() protocol.Position  { return f.limitMax }

// --- commands ---

func (f *fakeOpener) Open(_ context.Context, _ bool) error  { f.openCalls++; return nil }
func (f *fakeOpener) Close(_ context.Context, _ bool) error { f.closeCalls++; return nil }
func (f *fakeOpener) Stop(_ context.Context, _ bool) error  { f.stopCalls++; return nil }

func (f *fakeOpener) SetPosition(_ context.Context, position protocol.Position, _ bool) error {
	f.setPositions = append(f.setPositions, position.PositionPercent())
	return nil
}

func (f *fakeOpener) SetPositionLimitations(_ context.Context, min, max protocol.Position) error {
	f.setLimits = append(f.setLimits, [2]int{min.PositionPercent(), max.PositionPercent()})
	return nil
}

func (f *fakeOpener) ClearPositionLimitations(_ context.Context) error {
	f.clearLimitCalls++
	return nil
}

func (f *fakeOpener) GetLimitation(_ context.Context) (*klf200.LimitationResult, error) {
	f.getLimitCalls++
	return &klf200.LimitationResult{}, nil
}

var _ Opener = (*fakeOpener)(nil)
