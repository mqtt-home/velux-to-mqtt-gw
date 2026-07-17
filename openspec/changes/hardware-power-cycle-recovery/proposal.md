## Why

The KLF200's most common failure mode is a network-subsystem freeze after roughly a day of a long-lived open connection: the TLS handshake stops completing and the gateway accepts no new sessions. This freeze lives in the device's EFM32 Giant Gecko + WIZnet W5500 network subsystem and is **not** cleared by the soft `GW_REBOOT_REQ` the app already sends — in the field the device re-accepts a TCP connection ~2 s after the reboot request, far too fast to be a real device restart, and the covers stay unresponsive. The only reliable recovery is a physical power cycle. Today the bridge detects the wedge but can only log "manual power-cycle required" and wait for a human, leaving covers offline until someone is home.

## What Changes

- Add an **optional external power-cycle mechanism**: the gateway publishes to a configurable smart-plug MQTT topic to physically power the KLF200 off, wait a configurable dwell, then back on.
- Wire it as the **final escalation step in the health-check wedge-recovery path**: when a wedge is detected and the existing soft-reboot + reconnect fails to restore contact, the app power-cycles the device, waits for it to boot, and re-runs the full connect → authenticate → load → register → heartbeat sequence.
- Add config under the existing `restart` section: `power-cycle-topic`, `power-cycle-off-payload`, `power-cycle-on-payload`, and `power-cycle-off-seconds` (dwell). The feature is **disabled unless `power-cycle-topic` is set**.
- When disabled, behavior is unchanged: the wedge path still logs "manual power-cycle required" and keeps retrying reconnect. **Not breaking.**

## Capabilities

### New Capabilities
<!-- none: this extends existing runtime/config behavior rather than introducing a new capability -->

### Modified Capabilities
- `app-runtime`: the wedge-recovery flow gains an external power-cycle escalation step after soft-reboot+reconnect fails; recovery no longer necessarily requires a human when a power-cycle plug is configured.
- `app-config`: new optional `restart.power-cycle-*` settings and their validation/defaults.

## Impact

- **Code**: `app/app.go` (health-check/wedge-recovery path, new `doPowerCycle` step and its serialization via `resetMu`); `app/config/config.go` (new `restart.power-cycle-*` fields, defaults, validation); a small MQTT publish helper reachable from the App (via the existing `bridge`/`mqtt` wiring).
- **Config/docs**: `app/config-example.json` and README `restart` field reference.
- **External dependency**: relies on an operator-provided, MQTT-controllable smart plug wired to the KLF200's power; no new Go dependencies.
- **Backward compatibility**: fully opt-in; existing configs run unchanged.
