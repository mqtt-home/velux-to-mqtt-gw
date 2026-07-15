## MODIFIED Requirements

### Requirement: Application bootstrap and shutdown

The system SHALL, on startup, in order: load and validate config, connect to MQTT, connect and
authenticate to the KLF200, load and register nodes (publishing HA discovery and initial state),
**perform a device reboot to clear any zombie session slots left by a prior unclean shutdown**,
re-run connect + authenticate + node-reload + registration after the reboot, and start the
background loops (heartbeat, health check, periodic reboot). On shutdown it SHALL stop the loops,
clear discovery, cleanly disconnect from the KLF200, and disconnect from MQTT.

#### Scenario: Clean startup

- **WHEN** config is valid and both MQTT and the KLF200 are reachable
- **THEN** the application connects, reboots the gateway, reconnects after the gateway comes back,
  and reaches a ready state; the startup log includes both "Rebooting KLF200 on startup" and
  "Application is now ready"

#### Scenario: Startup failure exits non-zero

- **WHEN** config is invalid, MQTT connect fails, KLF200 authentication fails, or the post-reboot
  reconnect exhausts its retry budget
- **THEN** the system logs the failure and exits with a non-zero status

#### Scenario: Graceful shutdown on SIGINT/SIGTERM

- **WHEN** the process receives SIGINT or SIGTERM
- **THEN** the system stops background loops, clears HA discovery, performs a clean KLF200
  disconnect (releasing the session slot), disconnects MQTT, and exits

## REMOVED Requirements

### Requirement: Periodic clean session reset

**Reason**: A TCP-only session reset only frees our own slot; it cannot recover from a zombie
slot held by the KLF200 after an unclean prior disconnect. Field evidence: after a scheduled
clean reset the reconnect wedged for ~12h until manual power-cycle. Replaced by
"Periodic device reboot".

**Migration**: The existing `restart-interval` config field is reused unchanged; its semantics
change from "clean TCP session reset" to "device reboot". Users who relied on sub-hour intervals
should note the increased downtime per cycle (~60–90s vs ~5s).

## ADDED Requirements

### Requirement: Periodic device reboot

The system SHALL, when `restart-interval` is configured (default 24h), periodically send
`GW_REBOOT_REQ` to the KLF200 to reboot the device, then reconnect with exponential backoff,
re-authenticate, reload nodes, re-register callbacks, and restart the heartbeat. The MQTT
connection stays up across the reboot window; HA covers become unavailable during the
~60–90s reboot and are restored automatically when the reconnect succeeds.

#### Scenario: Periodic reboot cycle

- **WHEN** the configured `restart-interval` elapses
- **THEN** the system pauses the heartbeat, sends `GW_REBOOT_REQ`, waits for the gateway to
  come back with exponential-backoff reconnect attempts, re-authenticates, reloads nodes,
  re-registers per-node callbacks, re-publishes discovery/state, and restarts the heartbeat

#### Scenario: MQTT continuity across reboot

- **WHEN** a periodic reboot occurs
- **THEN** the MQTT connection stays up; HA discovery entries are not withdrawn (covers only
  transition to `offline`/`online` availability)

#### Scenario: Reboot reconnect timeout

- **WHEN** the gateway does not respond within the total reconnect budget (~5 minutes) after a
  reboot request
- **THEN** the reboot attempt returns an error and the periodic loop logs it; the next tick
  retries; covers stay marked unavailable until reconnect succeeds

### Requirement: Reactive reboot after wedge recovery

The system SHALL, after a health-check-driven reconnect succeeds following a wedge, immediately
send a device reboot to ensure both session slots are cleared, since the wedge may have been
caused by a zombie slot the reconnect itself could not free.

#### Scenario: Reactive reboot after recovery

- **WHEN** the health check detected the gateway as wedged and a subsequent reconnect succeeds
- **THEN** the system sends `GW_REBOOT_REQ`, waits for the gateway to come back, reconnects
  again, and restores cover availability

#### Scenario: Reactive reboot serialized with periodic reboot

- **WHEN** a periodic reboot and a reactive reboot could run concurrently
- **THEN** the two are serialized (only one reboot at a time)
