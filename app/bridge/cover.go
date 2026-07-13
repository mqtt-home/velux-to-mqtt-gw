package bridge

import (
	"context"
	"strconv"
	"strings"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200"
	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
	"github.com/philipparndt/go-logger"
)

// Opener is the subset of a klf200 opening-device the bridge drives. Every
// concrete opening type (Window, Blind, Awning, RollerShutter,
// DualRollerShutter, GarageDoor, Gate, Blade) satisfies it — the klf200 package
// exposes no shared interface, so the bridge declares its own narrow one. This
// is the seam the fan-out cover logic and tests depend on.
type Opener interface {
	klf200.Node

	// Live state accessors.
	Position() protocol.Position
	TargetPosition() protocol.Position
	LimitationMax() protocol.Position

	// Commands (waitForCompletion is always false from MQTT callbacks, matching
	// the Python bridge which used wait_for_completion=False throughout).
	Open(ctx context.Context, waitForCompletion bool) error
	Close(ctx context.Context, waitForCompletion bool) error
	Stop(ctx context.Context, waitForCompletion bool) error
	SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error

	// Keep-open limitation control.
	SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error
	ClearPositionLimitations(ctx context.Context) error
	GetLimitation(ctx context.Context) (*klf200.LimitationResult, error)
}

// HA cover device_class strings, mirroring the values HaCoverDeviceClass
// produced in the Python bridge.
const (
	DeviceClassWindow  = "window"
	DeviceClassBlind   = "blind"
	DeviceClassAwning  = "awning"
	DeviceClassShutter = "shutter"
	DeviceClassGarage  = "garage"
	DeviceClassGate    = "gate"
	DeviceClassShade   = "shade"
	DeviceClassNone    = ""
)

// keepOpen switch state payloads, matching the README contract.
const (
	keepOpenOn  = "on"
	keepOpenOff = "off"
)

// availability payloads, matching the Python cover's payload_available /
// payload_not_available.
const (
	availableOnline  = "online"
	availableOffline = "offline"
)

// DeviceClassFor is implemented in discovery.go (maps a concrete klf200
// opening-device to its HA cover device_class).

// Cover bridges a single klf200 opening-device to MQTT: it publishes state /
// position / availability / keep-open state and subscribes to the cover and
// keep-open command topics, forwarding commands to the node.
type Cover struct {
	// node is the underlying klf200 opening-device.
	node Opener

	// mqtt is the shared MQTT wrapper used to publish and subscribe.
	mqtt *MQTT

	// topics holds every topic for this cover and its keep-open switch.
	topics Topics

	// id is the generated MQTT id ("vlx-..."), without the HA prefix.
	id string

	// inverted is true for awnings when HomeAssistant.InvertAwning is enabled;
	// it flips both command mapping and state derivation.
	inverted bool

	// deviceClass is the HA cover device_class for discovery.
	deviceClass string

	// prefix is the HA prefix (config.HomeAssistant.Prefix) prepended to the id
	// verbatim, mirroring the Python HA_PREFIX+mqttid used for the device name.
	prefix string

	// lastState is the last MQTT state string published, used to log/detect
	// changes (mirrors Python's last_state).
	lastState string
}

// NewCover constructs a Cover for a node. It computes the id, topics, inversion
// flag, and device class up front so the pure derivation seams are wired.
func NewCover(node Opener, m *MQTT, prefix string, inverted bool) *Cover {
	id := GenerateID(node.Name())
	return &Cover{
		node:        node,
		mqtt:        m,
		topics:      NewTopics(prefix, id),
		id:          id,
		inverted:    inverted,
		deviceClass: DeviceClassFor(node),
		prefix:      prefix,
	}
}

// Start registers the KLF200 device-updated callback and subscribes to the
// cover and keep-open command topics, then publishes discovery, initial
// availability, and the current state. Ported from registerMqttCallbacks +
// register_devices' initial update.
func (c *Cover) Start(ctx context.Context) error {
	// Publish HA discovery configs before anything else so HA has topics wired.
	if err := c.PublishDiscovery(); err != nil {
		return err
	}

	// Forward live KLF200 state changes to MQTT.
	c.node.RegisterDeviceUpdatedCB(func(_ klf200.Node) {
		c.UpdateNode()
	})

	// Subscribe to the command topics. Handlers use a background context so a
	// command outlives the (short-lived) message dispatch.
	c.mqtt.Subscribe(c.topics.Set, func(payload string) {
		c.handleCoverCommand(context.Background(), payload)
	})
	c.mqtt.Subscribe(c.topics.KeepOpenSet, func(payload string) {
		c.handleKeepOpenCommand(context.Background(), payload)
	})

	// Publish initial availability and state (mirrors the Python bridge which
	// publishes availability(True) then updates the node on registration).
	c.PublishAvailability(true)
	c.UpdateNode()
	return nil
}

// PublishDiscovery publishes the retained HA discovery configs for the cover
// (homeassistant/cover/...) and keep-open switch (homeassistant/switch/...),
// using the builders in discovery.go, and records the topics so the Manager can
// clear them on shutdown.
func (c *Cover) PublishDiscovery() error {
	coverTopic, coverJSON, err := buildCoverDiscovery(c)
	if err != nil {
		return err
	}
	switchTopic, switchJSON, err := buildSwitchDiscovery(c)
	if err != nil {
		return err
	}
	c.mqtt.Publish(coverTopic, string(coverJSON), true)
	c.mqtt.Publish(switchTopic, string(switchJSON), true)
	recordDiscoveryTopic(coverTopic)
	recordDiscoveryTopic(switchTopic)
	logger.Debug("[bridge] published discovery", "node", c.node.Name(),
		"cover", coverTopic, "switch", switchTopic)
	return nil
}

// UpdateNode is the KLF200 device-updated callback body: recompute and publish
// cover state/position and keep-open state, plus availability. Ported from
// updateNode/updateCover/updateLimitSwitch.
func (c *Cover) UpdateNode() {
	c.updateCover()
	c.updateLimitSwitch()
	// Publish online status when node is updated (Python publishes availability
	// on every updateNode).
	c.PublishAvailability(true)
}

// updateCover derives and publishes state + position, porting updateCover /
// VeluxMqttCoverInverted.updateCover.
func (c *Cover) updateCover() {
	rawPosition := c.node.Position().PositionPercent()
	rawTarget := c.node.TargetPosition().PositionPercent()

	position, state := CoverState(rawPosition, rawTarget, c.inverted)

	c.mqtt.PublishRetained(c.topics.Position, strconv.Itoa(position))
	c.mqtt.PublishRetained(c.topics.State, state)

	if state != c.lastState {
		c.lastState = state
		logger.Debug("[bridge] cover state changed", "node", c.node.Name(),
			"position", position, "target", rawTarget, "state", state, "inverted", c.inverted)
	}
}

// updateLimitSwitch derives and publishes the keep-open switch state, porting
// updateLimitSwitch: 'on' when the max limitation is below fully-open (< 100%),
// else 'off'. An unknown/invalid limitation falls back to 'off'.
func (c *Cover) updateLimitSwitch() {
	maxPos := c.node.LimitationMax()
	state := keepOpenOff
	if maxPos.Known() && maxPos.PositionPercent() < 100 {
		state = keepOpenOn
	}
	c.mqtt.PublishRetained(c.topics.KeepOpenState, state)
}

// PublishAvailability publishes online/offline to the cover and keep-open
// availability topics.
func (c *Cover) PublishAvailability(online bool) {
	payload := availableOffline
	if online {
		payload = availableOnline
	}
	c.mqtt.PublishRetained(c.topics.Available, payload)
	c.mqtt.PublishRetained(c.topics.KeepOpenAvailable, payload)
}

// handleCoverCommand processes an OPEN/CLOSE/STOP/0-100 payload on the cover
// /set topic. Honors inversion (OPEN<->close, CLOSE<->open) as the Python
// VeluxMqttCoverInverted did. The numeric position is NOT inverted here: the
// Python bridge leaves the numeric position untouched and lets HA's
// position_open/closed discovery mapping handle inversion. Invalid payloads are
// logged and ignored.
func (c *Cover) handleCoverCommand(ctx context.Context, payload string) {
	cmd := strings.TrimSpace(payload)
	switch cmd {
	case "OPEN":
		logger.Debug("[bridge] cover command", "node", c.node.Name(), "cmd", "OPEN", "inverted", c.inverted)
		c.runCommand("OPEN", c.openFunc()(ctx))
		return
	case "CLOSE":
		logger.Debug("[bridge] cover command", "node", c.node.Name(), "cmd", "CLOSE", "inverted", c.inverted)
		c.runCommand("CLOSE", c.closeFunc()(ctx))
		return
	case "STOP":
		logger.Debug("[bridge] cover command", "node", c.node.Name(), "cmd", "STOP")
		c.runCommand("STOP", c.node.Stop(ctx, false))
		return
	}

	pos, err := strconv.Atoi(cmd)
	if err != nil {
		logger.Error("[bridge] unknown cover command", "node", c.node.Name(), "payload", payload)
		return
	}
	if pos < 0 || pos > 100 {
		logger.Error("[bridge] invalid cover position (must be 0-100)", "node", c.node.Name(), "position", pos)
		return
	}
	logger.Debug("[bridge] cover position command", "node", c.node.Name(), "position", pos)
	position, perr := protocol.NewPosition(nil, nil, &pos)
	if perr != nil {
		logger.Error("[bridge] build position failed", "node", c.node.Name(), "position", pos, "error", perr)
		return
	}
	c.runCommand("SET_POSITION", c.node.SetPosition(ctx, position, false))
}

// openFunc returns the node method that represents an HA "open" command,
// swapping to close when inverted (VeluxMqttCoverInverted.mqtt_callback_open).
func (c *Cover) openFunc() func(context.Context) error {
	if c.inverted {
		return func(ctx context.Context) error { return c.node.Close(ctx, false) }
	}
	return func(ctx context.Context) error { return c.node.Open(ctx, false) }
}

// closeFunc returns the node method that represents an HA "close" command,
// swapping to open when inverted (VeluxMqttCoverInverted.mqtt_callback_close).
func (c *Cover) closeFunc() func(context.Context) error {
	if c.inverted {
		return func(ctx context.Context) error { return c.node.Open(ctx, false) }
	}
	return func(ctx context.Context) error { return c.node.Close(ctx, false) }
}

// handleKeepOpenCommand processes an ON/OFF payload on the keep-open /set
// topic. ON sets a (0,0) position limitation (keep fully open); OFF clears the
// limitation. Ported from mqtt_callback_keepopen_on/off. Invalid payloads are
// logged and ignored.
func (c *Cover) handleKeepOpenCommand(ctx context.Context, payload string) {
	switch strings.TrimSpace(payload) {
	case "ON":
		logger.Debug("[bridge] enable keep-open limitation", "node", c.node.Name())
		zero, err := protocol.NewPosition(nil, nil, intPtr(0))
		if err != nil {
			logger.Error("[bridge] build limitation position failed", "node", c.node.Name(), "error", err)
			return
		}
		c.runCommand("KEEPOPEN_ON", c.node.SetPositionLimitations(ctx, zero, zero))
	case "OFF":
		logger.Debug("[bridge] disable keep-open limitation", "node", c.node.Name())
		c.runCommand("KEEPOPEN_OFF", c.node.ClearPositionLimitations(ctx))
	default:
		logger.Error("[bridge] unknown keep-open command", "node", c.node.Name(), "payload", payload)
	}
}

// runCommand logs a KLF200 command error, mirroring the Python bridge which
// caught and logged command failures rather than propagating them.
func (c *Cover) runCommand(cmd string, err error) {
	if err != nil {
		logger.Error("[bridge] KLF200 command failed", "node", c.node.Name(), "cmd", cmd, "error", err)
	}
}

// intPtr is a tiny helper to take the address of an int literal for
// protocol.NewPosition's positionPercent option.
func intPtr(v int) *int { return &v }

// ID returns the cover's generated MQTT id.
func (c *Cover) ID() string { return c.id }

// Topics returns the cover's topic set.
func (c *Cover) Topics() Topics { return c.topics }
