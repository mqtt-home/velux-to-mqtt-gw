## ADDED Requirements

### Requirement: Freeze diagnostic probe on wedge detection

The system SHALL, on the exact tick that the existing freeze detection
first declares the KLF200 wedged (and NOT on subsequent ticks that observe
the same wedge still in progress), run a diagnostic probe that opens a
fresh, plain (non-TLS) TCP connection to the KLF200 API port (`51200`)
with a bounded timeout of at most five seconds. The probe SHALL classify
the outcome as exactly one of `accepted`, `refused`, `reset`, `timeout`,
or `error`, and SHALL NOT influence the subsequent recovery flow — its
purpose is diagnostic only. The `error` outcome covers cases that are not
TCP-layer signals about the KLF200 (context cancellation during shutdown,
routing failures such as `EHOSTUNREACH` / `ENETUNREACH`, DNS failures,
permission errors, etc.); when `error` is emitted the raw dial-error text
SHALL be included on the log line so classification does not silently
discard diagnostic detail.

#### Scenario: Wedge detected, W5500 has free sockets

- **WHEN** the health check declares the gateway wedged
- **AND** a fresh plain-TCP connect to `<gateway>:51200` returns immediately
  with `accept`
- **THEN** the probe outcome is `accepted` and the recovery path proceeds
  unchanged

#### Scenario: Wedge detected, W5500 socket pool exhausted

- **WHEN** the health check declares the gateway wedged
- **AND** a fresh plain-TCP connect to `<gateway>:51200` neither succeeds
  nor is refused within the probe timeout
- **THEN** the probe outcome is `timeout` and the recovery path proceeds
  unchanged

#### Scenario: Wedge detected, gateway actively refuses

- **WHEN** the health check declares the gateway wedged
- **AND** a fresh plain-TCP connect returns `ECONNREFUSED` or `ECONNRESET`
- **THEN** the probe outcome is `refused` or `reset` respectively and the
  recovery path proceeds unchanged

#### Scenario: Wedge detected, dial fails for a non-TCP-layer reason

- **WHEN** the health check declares the gateway wedged
- **AND** the fresh plain-TCP connect fails with a non-TCP-layer reason
  (context canceled, routing failure, DNS failure, permission denied)
- **THEN** the probe outcome is `error`, the log line carries the raw
  dial-error text in a `probe_err` field, and the recovery path proceeds
  unchanged

### Requirement: Follow-up probes to test slot self-recycling

When the initial probe classifies as `timeout`, the system SHALL schedule
a follow-up probe at approximately two minutes after the initial probe and
a second follow-up at approximately five minutes after the initial probe,
each using the same fresh-TCP-connect classifier. The purpose is to test
whether the W5500 recycles its slots on its own within the `TIME_WAIT`
window (~1–4 min); given the observed recovery pattern (only mains
power-cycle helps), the expected outcome is that both follow-ups also
`timeout`. Any `accepted` follow-up is diagnostically important because it
contradicts the current mental model.

#### Scenario: Slots do not recycle on their own (expected)

- **WHEN** the initial probe classified as `timeout`
- **AND** both the two-minute and five-minute follow-ups also classify as
  `timeout`
- **THEN** the freeze event is logged with evidence consistent with a
  persistent socket-pool blockade (`CLOSE_WAIT`/`ESTABLISHED` stuck
  sockets, W5500 chip hang, or EFM32 task deadlock) — not `TIME_WAIT`

#### Scenario: Slots recycle unexpectedly

- **WHEN** the initial probe classified as `timeout`
- **AND** the two-minute or five-minute follow-up classifies as `accepted`
- **THEN** the freeze event is logged with evidence that contradicts the
  observed "only mains reboot helps" pattern and merits investigation

### Requirement: Post-soft-reboot probe to isolate deadlock from chip hang

The system SHALL run the same fresh-TCP-connect probe once more
immediately after the existing reactive `GW_REBOOT_REQ` recovery
completes and the client has reconnected. This attributes the freeze to
either an EFM32-task deadlock (soft reboot clears it → probe returns
`accepted`) or a W5500/EFM32 state that only a full power-cycle can clear
(probe still `timeout`). The result SHALL be logged as `post_soft_reboot`
on the same freeze event.

#### Scenario: Soft reboot clears the freeze

- **WHEN** the reactive `GW_REBOOT_REQ` recovery has completed and the
  client has reconnected
- **AND** the post-reboot probe returns `accepted`
- **THEN** the freeze event is logged with `post_soft_reboot=accepted`,
  evidence consistent with an EFM32 task deadlock

#### Scenario: Soft reboot does not clear the freeze

- **WHEN** the reactive `GW_REBOOT_REQ` recovery has completed and the
  client has reconnected
- **AND** the post-reboot probe returns `timeout`
- **THEN** the freeze event is logged with `post_soft_reboot=timeout`,
  evidence that only a mains power-cycle can resolve this freeze subclass

### Requirement: Structured freeze-event log line with stable event id

The system SHALL emit a structured log line for each probe result of a
freeze event, carrying at minimum a stable `event_id` for the freeze,
the probe stage (`initial` / `follow_up_120s` / `follow_up_300s` /
`post_soft_reboot`), the probe outcome, and the elapsed time to that
outcome in milliseconds. Using a common `event_id` per freeze lets a
single `grep event_id=<id>` retrieve every data point for that freeze
without a metrics pipeline.

#### Scenario: Log lines share a stable event_id per freeze

- **WHEN** a freeze event triggers the initial probe and one or more
  follow-ups or a post-soft-reboot probe
- **THEN** every emitted log line for that freeze carries the same
  `event_id`, together with `stage`, `freeze_probe`, and `t_ms` keys

#### Scenario: Log line keys are stable

- **WHEN** the initial probe completes
- **THEN** the log line contains at minimum `event_id`, `stage=initial`,
  `freeze_probe=<accepted|refused|reset|timeout|error>`, and `t_ms=<int>`

#### Scenario: Error outcome carries diagnostic detail

- **WHEN** a probe classifies as `error`
- **THEN** the log line additionally contains `probe_err=<raw dial-error text>`

### Requirement: Probe never blocks recovery beyond its timeout budget

The initial probe SHALL complete within its timeout budget (≤ 5 s) even
when the follow-up probes are pending, and the recovery path SHALL proceed
after the initial probe finishes. Follow-up probes SHALL run out of band
(on a separate goroutine or timer) and SHALL log their result without
delaying or interfering with reconnect/reboot.

#### Scenario: Recovery not delayed by probes

- **WHEN** the initial probe finishes (in any outcome)
- **THEN** the existing recovery path is released and proceeds concurrently
  with any pending follow-up probes
