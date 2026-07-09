# velux-cover-state Specification

## Purpose
TBD - created by archiving change rewrite-in-go. Update Purpose after archive.
## Requirements
### Requirement: Cover state derivation

The system SHALL derive an MQTT cover state from a node's current position and target position:
`closed` at the fully-closed position, `open` at any other resting position, `opening` while
moving toward a more-open position, and `closing` while moving toward a more-closed position.

#### Scenario: Resting fully closed

- **WHEN** a non-inverted node's position equals its target position and equals the
  fully-closed position (100%)
- **THEN** the derived state is `closed`

#### Scenario: Resting open

- **WHEN** a node's position equals its target position but is not the fully-closed position
- **THEN** the derived state is `open`

#### Scenario: Moving

- **WHEN** the target position differs from the current position
- **THEN** the derived state is `opening` when moving toward more-open, or `closing` when moving
  toward more-closed

### Requirement: Position reporting and validation

The system SHALL report node position as an integer percentage `0-100`, and SHALL clamp or fall
back sanely when the gateway reports an out-of-range position or target position.

#### Scenario: Valid position published

- **WHEN** a node reports a position within 0-100
- **THEN** that percentage is published as the cover position

#### Scenario: Invalid target treated as stopped

- **WHEN** a node's target position is outside 0-100
- **THEN** the system treats the node as stopped at its current position rather than reporting a
  spurious moving state

### Requirement: Awning inversion

The system SHALL, when awning inversion is enabled and a node is an awning, invert both the
open/close command mapping and the open/closed state derivation so that the awning presents
consistently in Home Assistant.

#### Scenario: Inverted closed position

- **WHEN** inversion is enabled and an awning rests at position 0%
- **THEN** the derived state is `closed`

#### Scenario: Inverted command mapping

- **WHEN** inversion is enabled and an OPEN command is received for an awning
- **THEN** the underlying KLF200 close action is invoked (and CLOSE maps to open)

