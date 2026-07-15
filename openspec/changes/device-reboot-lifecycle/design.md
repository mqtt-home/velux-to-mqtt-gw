## Context

The KLF200 gateway advertises 2 API session slots and does not garbage-collect
abandoned ones. Our current `doReset` in `app/app.go` pauses the heartbeat,
tears down the TCP session, waits 2s, and re-establishes it — but that only
releases *our* slot. A zombie slot held by the gateway (e.g. from a prior
process killed uncleanly, or an earlier connection the KLF200 forgot to expire)
survives untouched. Field evidence: after a scheduled 24h clean reset the
reconnect wedged for ~12h until manual power-cycle.

The KLF200 SLIP API includes `GW_REBOOT_REQ` (0x0001) / `GW_REBOOT_CFM`
(0x0002), and `Client.Reboot(ctx)` is already implemented in
`app/klf200/auth_system.go:261`. It is not wired to the app-level lifecycle.

Related seams already in place:
- `resetMu` in `App` serializes reset paths.
- `klfClient` interface abstracts the client for tests (`app/app_test.go`).
- `coverManager` interface abstracts the bridge manager the same way.

## Goals / Non-Goals

**Goals:**
- Actual KLF200 device reboots on startup, periodically, and reactively after
  wedge recovery — through a single funnel (`doReboot`) that is exercised in
  tests.
- Robust post-reboot reconnect that tolerates variable boot times without a
  fixed sleep (exponential backoff with an overall time budget).
- MQTT connection stays up across the reboot; HA covers flap only via
  availability (offline → online), not via discovery withdraw/republish.
- Backwards-compatible config (no new required fields).

**Non-Goals:**
- Rebooting the KLF200 via HTTP / smart-plug / other side-channels — API-only.
- Detecting *which* session slot is a zombie; we treat "any zombie" as
  possibility and always start clean via reboot.
- Changing the health-check wedge-detection logic itself (still keyed off
  `lastContact` and the failure-threshold multiplier).

## Decisions

### Decision: One `doReboot` funnel, three call sites

All three triggers (startup, periodic, reactive) use the same function.
Rationale: single code path → single set of tests; the operational blip is
identical, so the observability (log lines, availability transitions) is
identical too.

Alternative considered: separate `doStartupReboot` / `doPeriodicReboot` /
`doReactiveReboot`. Rejected — would duplicate the delicate stop-heartbeat →
reboot → backoff → reconnect sequence three times.

### Decision: Exponential backoff, not a fixed sleep

Post-reboot the KLF200 typically takes 60–90s to come back, but this varies.
A fixed sleep is either too short (fails on slow boots) or too long (delays
recovery). Chosen schedule: attempt reconnect after 2s, then 5s, 10s, 30s, 60s,
120s, 120s, capped at 120s per attempt. Total budget: 5 minutes. On budget
exhaustion `doReboot` returns an error; the caller decides what to do (periodic
loop logs and retries next tick; startup exits non-zero; reactive path leaves
covers unavailable for the next health-check tick to notice).

Alternative considered: TCP-connect probe every 1s until success. Rejected —
tight polling generates noisy logs and doesn't buy real speed since the gateway
isn't listening at all during the first ~30–60s of boot.

### Decision: Startup reboot is mandatory, not opt-in

The startup reboot is the single most valuable trigger — it clears the exact
class of zombie slot that has bitten us in production. Making it optional would
just mean users forget to enable it. Cost: one extra ~90s at container start;
acceptable for a service that runs indefinitely.

Alternative considered: `startup-reboot: bool` config flag. Rejected —
unnecessary knob; there is no scenario in which skipping the startup reboot is
better than doing it.

### Decision: Reactive reboot triggered from `checkHealth` post-recovery branch

The existing `checkHealth` in `app/app.go:299` already runs a `doReset` after a
wedge and clears the `wedged` flag on success. That success branch is the exact
place to call `doReboot` — the session slot situation may be unhealthy even
though our reconnect worked (that is precisely how we get zombies).

Alternative considered: reboot inside `doReset` unconditionally. Rejected —
would couple periodic and reactive semantics; keeps `doReset` (renamed
`doReconnect`) as a smaller building block for future non-reboot use.

### Decision: Rename `doReset` → `doReconnect` and keep it as internal helper

`doReboot` decomposes into: stop heartbeat → `Client.Reboot(ctx)` → wait for
gateway → `doReconnect(ctx)` (existing sequence: connectAndLoad + Register +
startHeartbeat). Reusing the connect path in one function keeps drift low.

### Decision: `Reboot` errors are non-fatal in the periodic path

If `GW_REBOOT_REQ` fails (e.g. session already gone), we still proceed to the
backoff-reconnect phase — the gateway may have rebooted already, or may just
need a fresh session. The periodic loop should be self-healing.

The startup path treats `Reboot`-send failure as non-fatal but treats
reconnect-budget-exhausted as fatal (exits non-zero).

## Risks / Trade-offs

- **Longer per-cycle downtime.** ~60–90s of "cover unavailable" per periodic
  cycle instead of ~5s. → Default interval is 24h; the daily blip is
  acceptable. README updated so ops teams know.
- **Startup delay of ~90s.** Container health checks that assume sub-30s
  ready time will trip. → Docker healthcheck in the compose stack is
  documented; recommended `start_period` in README.
- **KLF200 wear.** Frequent reboots may accelerate flash wear on the gateway.
  → Default 24h keeps reboots to ~365/year, well within reasonable device
  lifecycle; users are free to lower the interval at their own risk.
- **Both slots zombie at startup.** Initial connect fails → app exits non-zero
  → operator must power-cycle. → Same failure mode we already have today; no
  regression. Log message is actionable.
- **Reactive reboot loop.** If the gateway is truly broken, `checkHealth`
  could trigger reboot after reboot. → `resetMu` prevents concurrent reboots;
  the health-check interval (default 5min) throttles the cadence naturally;
  wedged-log-once guard limits log noise.

## Migration Plan

- Config: no changes required. `restart-interval` default (24h) is reused.
- Rollout: single binary release. On first run the app performs its startup
  reboot; users will see one ~90s longer bring-up in the container logs and
  one HA availability blip.
- Rollback: previous binary continues to work; the config field is unchanged,
  and there is no on-disk state that would be incompatible.

## Open Questions

_None._ Backoff schedule, trigger points, and error handling are decided above;
config surface is unchanged.
