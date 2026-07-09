## ADDED Requirements

### Requirement: Cover discovery as a device

The system SHALL publish a retained Home Assistant MQTT discovery config for each cover under
`homeassistant/cover/{prefix}{id}/config`, grouping the cover and its keep-open switch under a
single HA device (one device per cover, not loose entities), with a cover device_class derived
from the KLF200 node type.

#### Scenario: Cover config published

- **WHEN** a cover is registered
- **THEN** a retained discovery payload is published naming the state, position, command,
  set-position, and availability topics
- **AND** the payload references a device block shared with the cover's keep-open switch

#### Scenario: Device class from node type

- **WHEN** the node is a Window, Blind, Awning, RollerShutter, GarageDoor, Gate, or Blade
- **THEN** the discovery payload sets the corresponding HA cover device_class
  (window/blind/awning/shutter/garage/gate/shade), defaulting to none for unknown types

#### Scenario: Inverted position mapping

- **WHEN** awning inversion applies to the node
- **THEN** the discovery payload sets `position_open`/`position_closed` swapped relative to the
  non-inverted mapping

### Requirement: Keep-open switch discovery

The system SHALL publish a retained discovery config for each cover's keep-open switch under
`homeassistant/switch/{prefix}{id}-keepopen/config`, with an icon and the same shared device
block.

#### Scenario: Switch config published

- **WHEN** a cover is registered
- **THEN** a retained switch discovery payload is published with its state and command topics and
  a lock icon

### Requirement: Discovery cleanup on shutdown

The system SHALL clear every discovery topic it published (by publishing an empty retained
payload) on graceful shutdown, so Home Assistant removes the devices.

#### Scenario: Cleanup on shutdown

- **WHEN** the application shuts down gracefully
- **THEN** each previously published discovery topic receives an empty retained payload
