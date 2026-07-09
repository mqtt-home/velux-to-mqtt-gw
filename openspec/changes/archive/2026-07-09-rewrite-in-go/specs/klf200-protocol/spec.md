## ADDED Requirements

### Requirement: TLS connection to the KLF200 gateway

The system SHALL connect to the KLF200 gateway over TLS on TCP port `51200`, accepting the
gateway's self-signed certificate (no hostname or CA verification), and SHALL expose an explicit
clean disconnect that performs an orderly TLS/TCP close.

#### Scenario: Successful connect

- **WHEN** a connection is opened with a reachable host and valid TLS handshake
- **THEN** the socket is established and the connection is marked connected
- **AND** a read loop begins consuming inbound bytes

#### Scenario: Clean disconnect releases the session

- **WHEN** the caller invokes disconnect
- **THEN** the system performs an orderly close (TLS close_notify / TCP FIN) so the gateway can
  release the API session slot
- **AND** the connection is marked disconnected

#### Scenario: Connection lost is observable

- **WHEN** the underlying socket is closed by the peer or the network drops
- **THEN** the system marks the connection disconnected and notifies registered observers

### Requirement: SLIP framing

The system SHALL encode outbound frames and decode inbound bytes using SLIP (RFC-1055),
tokenizing a byte stream into discrete SLIP packets even when packet boundaries do not align
with socket reads.

#### Scenario: Round-trip framing

- **WHEN** a frame's bytes are SLIP-packed and then fed back through the tokenizer
- **THEN** exactly one packet is produced whose payload equals the original frame bytes

#### Scenario: Fragmented reads reassemble

- **WHEN** a SLIP packet arrives split across multiple socket reads
- **THEN** the tokenizer yields the packet only once the full frame has been received

### Requirement: GW_* frame serialization

The system SHALL provide a frame type per KLF200 `GW_*` command that serializes to and
deserializes from the exact byte layout defined by the KLF200 API specification, and SHALL map
inbound command codes to the correct frame type.

#### Scenario: Byte-exact serialization

- **WHEN** a known frame is serialized
- **THEN** the produced bytes match the specification's golden byte sequence for that command

#### Scenario: Unknown command code

- **WHEN** an inbound packet carries a command code with no registered frame type
- **THEN** the system ignores the packet without terminating the read loop

### Requirement: Session id generation

The system SHALL generate monotonically increasing session ids for request/response correlation,
wrapping from 65535 back to 1.

#### Scenario: Wraparound

- **WHEN** the session id reaches 65535 and a new id is requested
- **THEN** the next id is 1

### Requirement: Heartbeat keepalive

The system SHALL periodically (default every 60 seconds) send a state request to keep the
single API session alive and to provide a liveness signal, and SHALL treat a failed heartbeat as
a loss of contact.

#### Scenario: Heartbeat pulse

- **WHEN** the heartbeat interval elapses and the connection is healthy
- **THEN** a `GW_GET_STATE_REQ` is sent and a confirmation is received

#### Scenario: Heartbeat failure signals loss

- **WHEN** a heartbeat pulse does not receive its confirmation
- **THEN** the system surfaces the failure to the resilience layer
