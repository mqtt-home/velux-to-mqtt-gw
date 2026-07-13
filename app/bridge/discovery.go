package bridge

import (
	"encoding/json"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200"
)

// discoveryAvailability is one entry in the HA `availability` array.
type discoveryAvailability struct {
	Topic               string `json:"topic"`
	PayloadAvailable    string `json:"payload_available"`
	PayloadNotAvailable string `json:"payload_not_available"`
}

// discoveryDevice is the HA device-registry block shared by the cover entity
// and its keep-open switch, so HA groups them under a single device tile.
type discoveryDevice struct {
	Identifiers []string `json:"identifiers"`
	Name        string   `json:"name"`
}

// coverDiscoveryPayload is the JSON shape HA expects on
// homeassistant/cover/{prefix}{id}/config.
type coverDiscoveryPayload struct {
	Name                string                  `json:"name"`
	UniqueID            string                  `json:"unique_id"`
	ObjectID            string                  `json:"object_id"`
	StateTopic          string                  `json:"state_topic"`
	PositionTopic       string                  `json:"position_topic"`
	CommandTopic        string                  `json:"command_topic"`
	SetPositionTopic    string                  `json:"set_position_topic"`
	Availability        []discoveryAvailability `json:"availability"`
	PayloadAvailable    string                  `json:"payload_available"`
	PayloadNotAvailable string                  `json:"payload_not_available"`
	PositionOpen        int                     `json:"position_open"`
	PositionClosed      int                     `json:"position_closed"`
	DeviceClass         string                  `json:"device_class,omitempty"`
	Device              discoveryDevice         `json:"device"`
}

// switchDiscoveryPayload is the JSON shape HA expects on
// homeassistant/switch/{prefix}{id}-keepopen/config.
type switchDiscoveryPayload struct {
	Name         string                  `json:"name"`
	UniqueID     string                  `json:"unique_id"`
	ObjectID     string                  `json:"object_id"`
	StateTopic   string                  `json:"state_topic"`
	CommandTopic string                  `json:"command_topic"`
	Icon         string                  `json:"icon"`
	Availability []discoveryAvailability `json:"availability"`
	Device       discoveryDevice         `json:"device"`
}

// NOTE: discovery publishing + cleanup for production lives on the cover/manager
// path — Cover.PublishDiscovery (cover.go) records topics via recordDiscoveryTopic
// and Manager.CloseAll clears them via the package-level CleanupDiscovery
// (manager.go). The builders below assemble the payloads.

// buildCoverDiscovery assembles the homeassistant/cover/.../config topic and
// JSON payload for the given cover.
func buildCoverDiscovery(c *Cover) (topic string, payload []byte, err error) {
	t := c.topics

	// position_open/position_closed: normal mapping is open=0, closed=100.
	// Inverted (awnings) swaps them: open=100, closed=0.
	posOpen := 0
	posClosed := 100
	if c.inverted {
		posOpen = 100
		posClosed = 0
	}

	avail := []discoveryAvailability{
		{
			Topic:               t.Available,
			PayloadAvailable:    "online",
			PayloadNotAvailable: "offline",
		},
	}

	dev := sharedDevice(c)

	p := coverDiscoveryPayload{
		Name:                c.node.Name(),
		UniqueID:            c.id,
		ObjectID:            c.id,
		StateTopic:          t.State,
		PositionTopic:       t.Position,
		CommandTopic:        t.Set,
		SetPositionTopic:    t.Set,
		Availability:        avail,
		PayloadAvailable:    "online",
		PayloadNotAvailable: "offline",
		PositionOpen:        posOpen,
		PositionClosed:      posClosed,
		DeviceClass:         c.deviceClass,
		Device:              dev,
	}

	// HA discovery config topic: homeassistant/cover/{prefix}{id}/config
	// prefix is already encoded in c.topics.Base ({prefix}{id}), so we use the
	// raw id for the topic path and Base for the entity topics (which already carry prefix).
	// The Python ha_mqtt library constructs the discovery topic as:
	//   "homeassistant/{type}/{unique_id}/config"
	// where unique_id == HA_PREFIX + mqttid.  We replicate that:
	//   "homeassistant/cover/" + t.Base + "/config"
	discTopic := "homeassistant/cover/" + t.Base + "/config"

	b, err := json.Marshal(p)
	if err != nil {
		return "", nil, err
	}
	return discTopic, b, nil
}

// buildSwitchDiscovery assembles the homeassistant/switch/.../config topic and
// JSON payload for the keep-open switch of the given cover.
func buildSwitchDiscovery(c *Cover) (topic string, payload []byte, err error) {
	t := c.topics

	avail := []discoveryAvailability{
		{
			Topic:               t.KeepOpenAvailable,
			PayloadAvailable:    "online",
			PayloadNotAvailable: "offline",
		},
	}

	dev := sharedDevice(c)

	keepOpenID := c.id + "-keepopen"
	p := switchDiscoveryPayload{
		Name:         c.node.Name() + " Keep open",
		UniqueID:     keepOpenID,
		ObjectID:     keepOpenID,
		StateTopic:   t.KeepOpenState,
		CommandTopic: t.KeepOpenSet,
		Icon:         "mdi:lock-outline",
		Availability: avail,
		Device:       dev,
	}

	discTopic := "homeassistant/switch/" + t.KeepOpenBase + "/config"

	b, err := json.Marshal(p)
	if err != nil {
		return "", nil, err
	}
	return discTopic, b, nil
}

// sharedDevice builds the HA device block that ties the cover entity and the
// keep-open switch together under one device in the HA UI.
func sharedDevice(c *Cover) discoveryDevice {
	return discoveryDevice{
		// Python: HaDevice(HA_PREFIX + vlxnode.name, HA_PREFIX + mqttid)
		// The identifier is HA_PREFIX + mqttid == c.topics.Base.
		Identifiers: []string{c.topics.Base},
		Name:        c.node.Name(),
	}
}

// DeviceClassFor maps a concrete klf200 opening-device to its HA cover
// device_class, porting getHaDeviceClassFromVlxNode. Unknown types default to
// DeviceClassNone.
//
// This fills the foundation stub in cover.go.
func DeviceClassFor(node Opener) string {
	switch node.(type) {
	case *klf200.Window:
		return DeviceClassWindow
	case *klf200.Blind:
		return DeviceClassBlind
	case *klf200.Awning:
		return DeviceClassAwning
	case *klf200.RollerShutter:
		return DeviceClassShutter
	case *klf200.GarageDoor:
		return DeviceClassGarage
	case *klf200.Gate:
		return DeviceClassGate
	case *klf200.Blade:
		return DeviceClassShade
	default:
		return DeviceClassNone
	}
}
