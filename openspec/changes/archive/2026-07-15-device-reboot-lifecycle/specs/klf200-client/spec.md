## ADDED Requirements

### Requirement: Gateway reboot command

The system SHALL expose a `Reboot(ctx)` operation that sends `GW_REBOOT_REQ` (0x0001) and waits
for `GW_REBOOT_CFM` (0x0002). Callers MUST treat the connection as lost after a successful
confirmation, since the gateway drops the TCP session as part of the reboot.

#### Scenario: Reboot confirmation received

- **WHEN** the caller invokes `Reboot(ctx)` on an authenticated connection
- **THEN** the client sends `GW_REBOOT_REQ`, receives `GW_REBOOT_CFM`, and returns nil to the
  caller

#### Scenario: Connection lost after reboot

- **WHEN** the gateway has confirmed the reboot request
- **THEN** the caller SHALL close its connection state and reconnect from scratch (re-authenticate,
  re-load nodes) once the gateway is reachable again
