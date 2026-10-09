## Context

The KLF200's ~24 h TLS-handshake freeze remains the largest reliability risk
for this bridge. So far, the runtime treats every freeze the same: unanswered
SLIP pings → declare wedged → reconnect → reactive `GW_REBOOT_REQ` → mains
power-cycle if nothing else clears it. That works, but it is expensive
(physical relay actuation, brief network drop) and it explains nothing.

The static-analysis path to a root cause is closed: the EFM32 network image is
AES-encrypted (see `research/klf200-firmware/FINDINGS.md`). The next-most-
promising hypothesis, drawn from the Home Assistant community, is that the
WIZnet W5500 — a hardwired-TCP/IP controller with a fixed pool of **eight
concurrent TCP sockets** — is running out of slots. Note that the observed
recovery behavior (only a mains power-cycle reliably clears the freeze;
neither the passage of hours nor most soft reboots do) is **inconsistent
with sockets sitting in `TIME_WAIT`**, which would auto-recycle in about
one to four minutes. It IS consistent with sockets stuck in `CLOSE_WAIT` or
`ESTABLISHED` (never freed because the EFM32 application never called
`close()` or because no keepalive is running), or with the W5500 chip
itself in a bad state, or with an EFM32 task deadlock. From the client's
point of view all of these look identical to a TLS-state-machine stall.

Distinguishing these matters: client-side TCP hygiene (keepalive,
graceful `FIN`) is not a cure for the acute freeze — under `CLOSE_WAIT`
or hang, only a chip-level reset frees the slots. But it is preventive:
if we never leave the peer in a half-closed state, the EFM32 accumulates
stuck sockets more slowly and the freeze becomes rarer. The probe's job
is to attribute each freeze to a concrete cause so we can eventually
reason about the cadence and about whether a soft `GW_REBOOT_REQ`
suffices.

## Goals / Non-Goals

**Goals:**
- Make the KLF200 TCP client a well-behaved peer: never leave sockets in an
  unclean state on the W5500.
- Attribute future freezes to a concrete cause (socket-exhaustion vs. other)
  via a targeted probe run at the moment of freeze detection.
- Produce a reviewable evidence trail (structured logs) across many freeze
  events, so a follow-up change can be justified from data, not from theory.

**Non-Goals:**
- No change to the periodic-reboot cadence, the reactive-reboot behavior, or
  the mains power-cycle recovery. All three stay in place.
- No new metrics endpoint or dashboards. Logs are sufficient at this scale.
- No user-visible config surface for the probe (leave configurability for
  when there is a reason to disable it).
- No firmware modification or further static analysis of the KLF200 image.

## Decisions

### D1: Add TCP keepalive with a ~30 s idle + ~15 s interval

Enabling `SO_KEEPALIVE` and setting a short idle+interval on the KLF200 TCP
connection makes the peer (the W5500) observe our liveness and (per RFC 1122)
gives it grounds to reclaim a slot if we ever silently disappear. 30 s idle +
15 s interval + 3 probes gives worst-case ~75 s to notice a dead peer, well
inside the ~4 min `TIME_WAIT` window. Alternative considered: rely on
application-layer SLIP pings alone. Rejected: those only tell *us* the peer is
gone; they don't help the W5500 free its slot when *we* are the one that died.

### D2: Graceful close: `FIN` + bounded read-drain, never bare `Close` on error

Go's `net.TCPConn.Close()` sends `FIN` if the socket is in `ESTABLISHED`, but
if there's an error path where we call `Close` on a connection that already
had `SetLinger(0)` or where the OS returns `RST` because of buffered unread
data, the W5500 sees `RST` and may keep the slot in a worse state than a
clean half-close. The fix is a small helper: on graceful close, call
`CloseWrite()` (sends `FIN`), read-drain the peer for up to 2 s to consume
its `FIN`, then `Close()`. Alternative considered: `SetLinger(0)` for
"fast" close. Explicitly rejected: `linger=0` sends `RST`, which is exactly
the pattern we suspect leaves the W5500 in a bad state.

### D3: Freeze diagnostic probe = fresh plain-TCP connect, no TLS

When the health-check declares the gateway wedged, the client already tears
down its (dead) session and starts reconnecting. Before that reconnect
begins, run a **plain TCP** `Dial` with a short timeout (~5 s) to
`<gateway>:51200`. Four outcomes matter, and the follow-up probes at t+120 s
and t+300 s (plus a probe after the reactive soft reboot, see D6) let us
attribute the freeze:

- **`accepted`**: the W5500 has free slots. The freeze is not TCP-layer
  socket starvation; something else (TLS state, EFM32 higher layer) is at
  fault.
- **`refused` / `reset`**: the W5500 actively rejected. Not a full pool (a
  full pool stalls, it does not `RST`); indicates a different device-side
  fault mode.
- **`timeout` at initial AND at t+120 s AND at t+300 s**: full pool, and
  slots are not recycling on their own — matches `CLOSE_WAIT`/stuck-
  `ESTABLISHED` accumulation, a W5500 chip-level hang, or an EFM32 task
  deadlock. **This is the outcome consistent with the observed "only mains
  power-cycle helps" behavior.** The post-soft-reboot probe (D6) then
  distinguishes an EFM32-task deadlock (soft reboot clears it) from a
  W5500 chip hang (soft reboot does not).
- **`timeout` at initial but `accepted` at t+120 s or t+300 s**: would
  contradict the observed behavior. If we ever see this, it means slots DO
  recycle sometimes — reopens the door to `TIME_WAIT`-like transient
  exhaustion, which our current mental model does not expect.

We do plain TCP, not TLS, because a TLS handshake against a wedged gateway
is exactly what times out today; the point of the probe is to isolate the
**TCP layer** result.

### D4: Log-only outcomes; do not change recovery flow based on the probe

The probe classifies but does not act. The existing recovery path
(reconnect → reactive reboot → power-cycle) runs unchanged. This keeps the
change small and safe: the worst thing the probe can do is add ~5 s to a
freeze event, and even if the probe's classification is wrong we lose
nothing operationally.

### D6: Re-probe after soft reboot to measure `GW_REBOOT_REQ` efficacy

The existing recovery path already sends a reactive `GW_REBOOT_REQ` after a
health-check-driven reconnect (see `app-runtime` spec). We piggyback on that:
once the soft reboot has completed and the client has reconnected, run the
probe again from a fresh source. This attributes the freeze to one of:

- Soft reboot fixed it (probe now `accepted`) → EFM32-task deadlock or an
  EFM32 socket-cleanup bug that the firmware reset clears. **A soft-reboot
  policy could — in principle — replace mains power-cycling for this class
  of freeze.**
- Soft reboot did NOT fix it (probe still `timeout`) → W5500 chip-level
  hang or wider EFM32 corruption that only a chip power-off resolves.
  The mains power-cycle remains necessary.

Alternative considered: measure only whether the reconnect succeeded. Rejected:
a successful reconnect after a soft reboot only tells us at least one slot is
free; the probe run against the same address from a fresh dial gives us the
TCP-layer signal we want in a form that matches the initial probe (same
classifier, so results are directly comparable).

### D5: Structured log key `freeze_probe` for evidence collection

Emit exactly one line per freeze event at the moment the probe finishes,
with a stable key set: `freeze_probe=<accepted|refused|reset|timeout>`,
`t_ms=<int>`, `follow_up_120s=<same set>` (nullable), `follow_up_300s=<same
set>` (nullable), `post_soft_reboot=<same set>` (nullable — populated when
the reactive reboot completes and its post-probe returns). Emit an update
line with a stable `event_id` when each follow-up completes, so the four
data points per freeze can be correlated with a single `grep`. A follow-up
change (or a manual review after N events) can grep for `freeze_probe=` in
logs and decide whether the evidence justifies loosening the reboot cadence
or replacing the mains power-cycle with a soft reboot for the deadlock
subclass.

## Risks / Trade-offs

- **Keepalive interval too aggressive** → wastes device CPU cycles on the
  EFM32. Mitigation: 30 s idle is inline with common defaults for
  IoT-embedded TCP stacks; not tight enough to matter.
- **Graceful-close read-drain adds latency on error paths** → up to 2 s
  extra per teardown. Mitigation: this only fires on graceful close;
  emergency teardown paths (context canceled, watchdog fired) still use
  fast close.
- **Probe classification is imperfect** → a stalled probe could also mean
  general network unreachability, not W5500 exhaustion. Mitigation: the
  two follow-up probes at t+120s and t+300s discriminate — general
  unreachability stays stalled; socket exhaustion transitions to
  `accepted` around the ~4 min mark.
- **Log spam if freezes ever become frequent** → the probe emits one line
  per freeze; freezes today are ~daily; if they became per-minute we would
  have bigger problems than log volume.
- **Probe interacts badly with an in-flight reconnect** → serialize: the
  probe runs synchronously in the health-check goroutine before releasing
  the reconnect trigger. Under 5 s worst case; well within the reconnect
  budget.

## Migration Plan

Pure additive change. Steps:

1. Land the TCP-keepalive + graceful-close changes to `klf200-client`.
2. Land the probe helper and its invocation from the freeze-detection path.
3. Deploy; do nothing else. Let logs accumulate.
4. After ≥ 5 observed freeze events on my device (about 1 week at current
   rate), review logs. Expected distribution given the observed "only
   mains power-cycle helps" behavior:
   - `freeze_probe=timeout` at initial, `follow_up_120s=timeout`,
     `follow_up_300s=timeout`: persistent socket-pool blockade
     (`CLOSE_WAIT`, W5500 hang, or EFM32 task deadlock).
   - If `post_soft_reboot=accepted` in that cluster: **EFM32 task
     deadlock confirmed** → file a follow-up change to promote soft
     reboot over mains power-cycle for this subclass.
   - If `post_soft_reboot=timeout` in that cluster: **W5500 chip hang or
     deeper EFM32 corruption** → keep the mains power-cycle as-is.
   - If `freeze_probe=accepted` dominates: the freeze is above the TCP
     layer (TLS state, application logic) — drop the socket-exhaustion
     hypothesis and consider archiving this probe.
   - If `follow_up_120s=accepted` or `follow_up_300s=accepted`
     dominates: contradicts the observed persistence, worth
     investigating why some devices/deployments recover on their own.

Rollback: revert the two commits. There is no state to migrate.

## Open Questions

- **Should the probe also try a second connect from a different source
  port immediately?** A same-source-port retry within `MSL` could hit
  `TIME_WAIT` on the local stack, not the peer. Deferred: Go picks
  ephemeral source ports, so this is likely already fine, but worth
  eyeballing during code review.
- **Do we need to correlate the probe outcome with the current session
  slot the client is on (0 vs 1)?** The `power-cycle-recovery` doc talks
  about "zombie session slots"; if only one of the two KLF200 API slots
  is exhausted, the probe would still succeed and we would misclassify.
  Deferred: revisit after first evidence review — if outcomes look
  inconsistent, add slot-awareness then.
