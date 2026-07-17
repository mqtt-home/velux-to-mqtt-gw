## MODIFIED Requirements

### Requirement: Wedge detection and visibility

The system SHALL, when a health-check interval is configured, detect a lost KLF200 (no successful
contact within the interval times the failure threshold) and make the condition visible rather
than silently retrying: publish `offline` availability for all covers and log an actionable
message, while continuing to attempt reconnection. When no external power-cycle plug is
configured, the actionable message SHALL indicate that a manual power-cycle is required. When an
external power-cycle plug is configured, the system SHALL instead escalate automatically per the
"Automated hardware power-cycle recovery" requirement.

#### Scenario: Health check trips without power-cycle plug configured

- **WHEN** no successful KLF200 contact has occurred within `health-check-interval` × threshold and
  no `power-cycle-topic` is configured
- **THEN** the system marks all covers unavailable and logs that a manual gateway power-cycle is
  required

#### Scenario: Recovery after power-cycle

- **WHEN** the gateway becomes reachable again after being wedged
- **THEN** the system reconnects, reloads nodes, and restores availability and state

## ADDED Requirements

### Requirement: Automated hardware power-cycle recovery

The system SHALL, when a `power-cycle-topic` is configured and a wedge persists after the existing
soft-recovery attempts (reconnect and `GW_REBOOT_REQ`) have failed to restore contact, escalate by
physically power-cycling the KLF200: publish the configured off-payload to the power-cycle topic,
wait the configured dwell, publish the on-payload, then wait for the gateway to boot and re-run the
full connect → authenticate → load-nodes → register → heartbeat bring-up. This escalation SHALL be
serialized with the reconnect and reboot paths (only one recovery action at a time) and SHALL be
debounced so the device is not power-cycled on every health-check tick.

#### Scenario: Escalate to power cycle when soft recovery fails

- **WHEN** the gateway is wedged, `restart-on-error` is enabled, a `power-cycle-topic` is configured,
  and a reconnect and/or `GW_REBOOT_REQ` have failed to restore contact
- **THEN** the system publishes the off-payload to the power-cycle topic, waits `power-cycle-off-seconds`,
  publishes the on-payload, waits for the gateway with exponential-backoff reconnect, reloads nodes,
  re-registers covers, restarts the heartbeat, and restores cover availability once contact resumes

#### Scenario: Power-cycle publishes are non-retained

- **WHEN** the system publishes the off-payload and on-payload to the power-cycle topic
- **THEN** the messages are published without the retain flag, so a plug does not remain forced off
  after the sequence completes

#### Scenario: Power cycle is debounced

- **WHEN** a wedge persists across multiple health-check ticks after a power cycle was already
  performed within the debounce window
- **THEN** the system does not power-cycle again during the window; it logs that it is still
  unreachable and keeps attempting reconnection

#### Scenario: Power-cycle escalation serialized with reboot

- **WHEN** a periodic reboot, a reactive reboot, and a power-cycle escalation could run concurrently
- **THEN** the actions are serialized (only one runs at a time)

#### Scenario: Feature disabled when unconfigured

- **WHEN** no `power-cycle-topic` is configured
- **THEN** no power-cycle publish is ever issued and the wedge path behaves exactly as before
  (logging that a manual power-cycle is required)
