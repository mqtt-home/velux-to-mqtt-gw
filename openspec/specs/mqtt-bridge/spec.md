# mqtt-bridge Specification

## Purpose
TBD - created by archiving change rewrite-in-go. Update Purpose after archive.
## Requirements
### Requirement: MQTT connection lifecycle

The system SHALL connect to the configured MQTT broker on startup with the optional
username/password, retry on failure, and disconnect cleanly on shutdown.

#### Scenario: Successful connect

- **WHEN** the application starts with a valid broker URL and credentials
- **THEN** the MQTT client connects and is ready to publish and subscribe

#### Scenario: Retry on failure

- **WHEN** the initial MQTT connection attempt fails
- **THEN** the system retries with backoff before giving up

### Requirement: Cover state publishing

The system SHALL publish, with the retain flag, each cover's state to `{prefix}{id}/state`, its
position to `{prefix}{id}/position`, its availability to `{prefix}{id}/available`, and the
keep-open switch state to `{prefix}{id}-keepopen/state`, matching the Python topic layout and
payloads.

#### Scenario: State published on change

- **WHEN** a node's derived cover state or position changes
- **THEN** the new value is published retained to the corresponding topic

#### Scenario: Availability reflects contact

- **WHEN** a node is updated from the gateway
- **THEN** `{prefix}{id}/available` is published as `online`
- **AND** when the gateway is unreachable, availability is published as `offline`

### Requirement: Command subscription

The system SHALL subscribe to `{prefix}{id}/set` and `{prefix}{id}-keepopen/set`, interpreting
`OPEN`, `CLOSE`, `STOP`, and an integer `0-100` on the cover command topic, and `ON`/`OFF` on the
keep-open command topic, forwarding each to the KLF200 client.

#### Scenario: Position command

- **WHEN** an integer `0-100` is received on a cover's `/set` topic
- **THEN** the cover is commanded to that position

#### Scenario: Open/close/stop command

- **WHEN** `OPEN`, `CLOSE`, or `STOP` is received on a cover's `/set` topic
- **THEN** the corresponding KLF200 action is invoked

#### Scenario: Keep-open command

- **WHEN** `ON` is received on a `-keepopen/set` topic
- **THEN** the keep-open position limitation is enabled; **WHEN** `OFF` is received the limitation
  is cleared

#### Scenario: Invalid command ignored

- **WHEN** an unrecognized payload is received on a command topic
- **THEN** the system logs an error and takes no device action

### Requirement: MQTT id generation

The system SHALL derive a cover's MQTT id from its node name as `vlx-` followed by the name
lowercased, spaces replaced with hyphens, and German umlauts transliterated (ä→ae, ö→oe, ü→ue,
ß→ss), preserving compatibility with existing topics.

#### Scenario: Umlaut normalization

- **WHEN** a node is named "Dachfenster Büro"
- **THEN** its MQTT id is `vlx-dachfenster-buero`

