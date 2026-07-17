## Context

The bridge already has a layered recovery model in `app/app.go`:

1. **Heartbeat** (`GW_GET_STATE_REQ` every 60 s) as keepalive + liveness; each confirmation/node update stamps `lastContact`.
2. **Health check** trips when no contact within `health-check-interval × 2`; it marks covers offline, logs "manual power-cycle required", and — when `restart-on-error` is set — attempts `doReconnect` then a soft `doReboot` (`GW_REBOOT_REQ`).
3. **Periodic reboot** sends `GW_REBOOT_REQ` on `restart-interval` (default 24 h).

Field evidence and hardware research (FCC BOM XSG832160) show the ~24 h freeze sits in the KLF200 network subsystem (SiLabs EFM32GG990 + WIZnet W5500) and is **not** cleared by `GW_REBOOT_REQ`: in the user's logs the device re-accepted a connection ~2 s after the reboot request (attempt=1 on the first backoff slot), which is far too fast for a genuine device boot. Multiple upstream projects confirm only a physical power cycle recovers this state. The current code acknowledges this by logging "manual power-cycle required" — this change automates that manual step for operators who wire the KLF200 through a controllable smart plug.

All reboot/reconnect paths are already serialized through `resetMu`; the new step must join that serialization.

## Goals / Non-Goals

**Goals:**
- Automatically recover from a wedge that survives the soft reboot, when (and only when) an MQTT-controlled power plug is configured.
- Reuse the existing MQTT connection and the existing connect/load/register/heartbeat bring-up.
- Zero behavior change when unconfigured; fully opt-in.

**Non-Goals:**
- No new device-discovery, plug auto-detection, or vendor-specific plug integrations — the operator supplies raw topic + payloads (works with Tasmota, Shelly, Zigbee2MQTT, ZHA-via-MQTT, etc.).
- Not replacing the periodic soft reboot; the power cycle is escalation-only, not a scheduled action (see Open Questions).
- No direct mains/relay/GPIO control from the process.

## Decisions

### D1: Trigger via MQTT publish, not a shell hook or HTTP call
The bridge is already an MQTT client with a live connection; publishing an off/on payload to an operator-named topic is the lowest-friction, most broadly compatible mechanism and adds no dependencies. Alternatives considered: (a) exec a user command — rejected, expands attack surface and complicates the container image; (b) built-in HTTP client for specific plug brands — rejected, brittle and vendor-specific.

### D2: Escalation-only, inside the health-check wedge path
The power cycle runs as the last resort in `checkHealth` after `doReconnect` and/or the soft `doReboot` fail to restore contact — not on every wedge tick and not on a timer. This bounds how often mains power to the gateway is toggled and preserves the cheaper soft paths as the first responders. A new `doPowerCycle(ctx)` method, serialized via `resetMu`, performs: publish off → sleep `power-cycle-off-seconds` → publish on → wait for boot (reuse `rebootBackoffSchedule`) → `connectAndLoad` → `cm.Register` → restart heartbeat.

### D3: Config lives under `restart`, feature-gated on `power-cycle-topic`
Keeps the recovery knobs together. Empty/unset `power-cycle-topic` = disabled. New fields:
- `power-cycle-topic` (string, default ""): MQTT topic to publish plug commands to.
- `power-cycle-off-payload` (string, default `"OFF"`), `power-cycle-on-payload` (string, default `"ON"`): payloads matching common plugs (Tasmota `POWER` = ON/OFF).
- `power-cycle-off-seconds` (int, default 10): dwell with power removed, long enough for the KLF200's caps to drain.
Published with QoS 1 and retain=false (a retained OFF could strand the plug off across restarts).

### D4: Debounce repeated power cycles
Track the last power-cycle time; enforce a minimum gap (reuse/derive from `restart-interval`, or a fixed floor such as 15 min) so a gateway that never recovers is not power-cycled every health-check tick. On repeated failure, fall back to the existing "still unreachable — waiting" logging.

### D5: Reuse existing bring-up and availability transitions
`doPowerCycle` ends by calling the same `connectAndLoad` + `cm.Register` + `startHeartbeatFn` used by `doReboot`, and the `wedged` CAS + `MarkAllAvailable` recovery is driven by the next healthy `checkHealth` tick, so HA entities do not flap beyond the existing offline→online transition.

## Risks / Trade-offs

- **Wrong topic/payload does nothing** → Mitigation: log the exact publish (topic + payload) at info; document per-plug examples (Tasmota/Shelly) in the README; feature is opt-in so misconfig cannot affect default users.
- **Power-cutting mid-movement could leave a cover partway** → Mitigation: only triggered after the gateway is already wedged (covers already unresponsive/offline), so no in-flight commands are lost that were not already lost.
- **Retained OFF payload strands the plug powered off** → Mitigation: publish non-retained (D3).
- **Boot time varies by plug + KLF200** → Mitigation: reuse the proven exponential backoff (~5 min budget) already used for soft reboots.
- **Flapping / repeated toggling wears the plug relay and gateway** → Mitigation: debounce (D4).
- **MQTT down at the moment of escalation** → the publish fails; log and fall back to "waiting for manual power-cycle"; next tick retries.

## Open Questions

- Should the power cycle also be offered as an explicit operator-triggered MQTT command (manual "recover now"), in addition to automatic escalation? (Deferred; out of scope for this change.)
- Should a configurable option allow the periodic loop to use a power cycle instead of a soft reboot on some cadence (e.g. weekly)? (Deferred; escalation-only for now per Non-Goals.)
- Exact debounce floor value — tie to `restart-interval` or a fixed 15 min? (Resolve in tasks; default to a fixed floor.)

## References

Root-cause evidence that the freeze is not cleared by a soft reboot and needs a physical power cycle:

- pyvlx #30 — "Keeping connection open freezes KLF" (SSL handshake stalls after ~24 h; only unplugging helps): https://github.com/Julius2342/pyvlx/issues/30
- Home Assistant core #23748 — KLF200 connection issues, gateway won't initiate SSL handshake until physically restarted: https://github.com/home-assistant/core/issues/23748
- Home Assistant core #154294 — KLF200 does not reboot at restart: https://github.com/home-assistant/core/issues/154294
- Home Assistant core #48395 — velux not responding after 1–2 days: https://github.com/home-assistant/core/issues/48395
- openHAB-addons #8462 — reboot feature request; device "needs power recycling with regular intervals": https://github.com/openhab/openhab-addons/issues/8462
- PLCHome/velux-klf200-api — daily GW_REBOOT_REQ still requires physical restart: https://github.com/PLCHome/velux-klf200-api
- Hardware (network subsystem = EFM32GG990 + WIZnet W5500) — FCC filing XSG832160 BOM/internal photos: https://fcc.report/FCC-ID/XSG832160
- Velux KLF200 API technical specification (2 sockets on port 51200, 15-min idle close): https://velcdn.azureedge.net/~/media/com/api/klf200/technical%20specification%20for%20klf%20200%20api-ver3-16.pdf
