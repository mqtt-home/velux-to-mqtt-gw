## Why

The KLF200 gateway has only 2 API session slots and never cleans up abandoned
ones. Our current "clean session reset" only tears down and re-establishes our
TCP session — it cannot free a zombie slot left by a previous unclean process
exit or by the KLF200 forgetting our earlier connect. Field evidence: after a
scheduled 24h clean reset the reconnect wedged for ~12h until a manual restart,
even though the disconnect itself was clean. The only reliable recovery is to
reboot the KLF200 device itself, which frees *all* slots. The API command for
that (`GW_REBOOT_REQ`) already exists in our client (`Client.Reboot`) but is
not wired anywhere.

## What Changes

- Replace the current in-process "clean session reset" with a real **device
  reboot** flow driven by `Client.Reboot` (`GW_REBOOT_REQ` → `GW_REBOOT_CFM`).
- Introduce a single `doReboot()` path used by three triggers:
  1. **Startup**: after the initial `connectAndLoad`, reboot once — clears
     zombie slots from prior unclean shutdowns.
  2. **Periodic**: the existing `restart-interval` loop (default 24h) sends a
     device reboot instead of a TCP-only reset.
  3. **Reactive**: after a health-check-driven reconnect succeeds following a
     wedge, reboot to force a clean slot state.
- After sending `GW_REBOOT_REQ`, wait for the gateway to come back with an
  exponential-backoff reconnect (~2s, 5s, 10s, 30s, cap 2min, total budget
  ~5min), then re-run `connectAndLoad` + `cm.Register` + heartbeat restart.
- Serialize all three paths via the existing `resetMu`.
- **BREAKING** (operational): the old `doReset` (TCP-only) is removed; the
  visible periodic action now costs ~60–90s of downtime per cycle instead of
  a ~5s TCP flap. HA covers go unavailable during the reboot window.
- Bump minor version.

## Capabilities

### New Capabilities
_None_ — this refines existing app-runtime and klf200-client behaviour.

### Modified Capabilities
- `app-runtime`: replace the "Periodic clean session reset" requirement with a
  device-reboot lifecycle covering startup, periodic, and reactive triggers.
- `klf200-client`: add a requirement documenting the reboot command and its
  post-condition (connection loss + gateway restart).

## Impact

- Code: `app/app.go` (new `doReboot`, startup wiring, reset-loop swap,
  health-check post-recovery hook), `app/app_test.go` (fake needs a `Reboot`
  method, new coverage for backoff and the three trigger paths).
- Client: `app/klf200/client.go` — `Client.Reboot` already implemented; the
  `klfClient` seam in `app/app.go` grows a `Reboot(ctx)` method.
- Config: no schema change. `restart-interval` semantics change (reboot instead
  of TCP reset) — documented in README.
- Docs: `README.md` restart section — describe the reboot-based lifecycle and
  the ~60–90s downtime window per cycle.
- Ops: users on <24h restart intervals see more frequent availability blips;
  default (24h) is unchanged. First startup after upgrade includes an extra
  ~60–90s bring-up delay from the initial reboot.
