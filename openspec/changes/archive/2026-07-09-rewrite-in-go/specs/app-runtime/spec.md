## ADDED Requirements

### Requirement: Application bootstrap and shutdown

The system SHALL, on startup, in order: load and validate config, connect to MQTT, connect and
authenticate to the KLF200, load and register nodes (publishing HA discovery and initial state),
and start the background loops (heartbeat, health check, periodic reset). On shutdown it SHALL
stop the loops, clear discovery, cleanly disconnect from the KLF200, and disconnect from MQTT.

#### Scenario: Clean startup

- **WHEN** config is valid and both MQTT and the KLF200 are reachable
- **THEN** the application reaches a ready state and logs that it is ready

#### Scenario: Startup failure exits non-zero

- **WHEN** config is invalid, MQTT connect fails, or KLF200 authentication fails
- **THEN** the system logs the failure and exits with a non-zero status

#### Scenario: Graceful shutdown on SIGINT/SIGTERM

- **WHEN** the process receives SIGINT or SIGTERM
- **THEN** the system stops background loops, clears HA discovery, performs a clean KLF200
  disconnect (releasing the session slot), disconnects MQTT, and exits

### Requirement: KLF200 contact tracking and heartbeat

The system SHALL record the time of the most recent successful KLF200 contact (heartbeat
confirmation or node update) and SHALL use the heartbeat as both keepalive and liveness signal.

#### Scenario: Contact recorded

- **WHEN** a heartbeat confirmation or a node update is received
- **THEN** the last-contact timestamp is updated

### Requirement: Periodic clean session reset

The system SHALL, when a restart interval is configured, periodically perform an in-process
**clean** disconnect and reconnect — releasing the KLF200 session slot and reacquiring it,
reloading nodes and re-registering callbacks — without exiting the process or dropping the MQTT
connection.

#### Scenario: Periodic reset cycle

- **WHEN** the configured restart interval elapses
- **THEN** the system cleanly disconnects from the KLF200, waits briefly, reconnects,
  re-authenticates, reloads nodes, re-registers update callbacks, and re-publishes discovery/state

#### Scenario: MQTT and HA continuity across reset

- **WHEN** a periodic reset occurs
- **THEN** the MQTT connection stays up and HA discovery entities do not flap

### Requirement: Wedge detection and visibility

The system SHALL, when a health-check interval is configured, detect a lost KLF200 (no successful
contact within the interval times the failure threshold) and make the condition visible rather
than silently retrying: publish `offline` availability for all covers and log an actionable
message indicating a manual power-cycle is required, while continuing to attempt reconnection.

#### Scenario: Health check trips

- **WHEN** no successful KLF200 contact has occurred within `health-check-interval` × threshold
- **THEN** the system marks all covers unavailable and logs that a manual gateway power-cycle is
  required

#### Scenario: Recovery after power-cycle

- **WHEN** the gateway becomes reachable again after being wedged
- **THEN** the system reconnects, reloads nodes, and restores availability and state

### Requirement: Structured logging

The system SHALL emit structured logs at the configured level, with an option for more verbose
KLF200 protocol logging.

#### Scenario: Log level applied

- **WHEN** `loglevel` is set to `debug`
- **THEN** debug-level messages are emitted
