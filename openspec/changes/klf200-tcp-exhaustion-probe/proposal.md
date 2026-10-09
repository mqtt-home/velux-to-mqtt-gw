## Why

Static reverse-engineering of the KLF200 v2.0.0.71 firmware turned out to be a
dead-end (the EFM32 image is AES-encrypted; see `research/klf200-firmware/FINDINGS.md`).
Meanwhile, the Home Assistant community has independently converged on a
concrete alternative hypothesis for the ~24 h TLS-handshake freeze: **TCP
socket exhaustion on the WIZnet W5500**. The W5500 has a hardware limit of
eight concurrent TCP sockets. But the observed recovery pattern (only a
mains power-cycle reliably clears the freeze — not the passage of hours,
often not even a soft `GW_REBOOT_REQ`) rules out `TIME_WAIT`-driven
exhaustion, which would auto-recycle within about one to four minutes. What
fits the observation is either sockets stuck in `CLOSE_WAIT`/`ESTABLISHED`
because the EFM32 application never closes them, a W5500 chip-level hang,
or an EFM32 task deadlock. All three look identical from the client. This
hypothesis is testable from the client side alone — no firmware decryption
required — and, if we can attribute freezes to one of the three subclasses,
we can reason about whether a soft reboot could ever replace the mains
power-cycle for at least some of them.

## What Changes

- Make the KLF200 TCP client **actively socket-hygienic**: on graceful
  disconnect, send a proper `FIN` and drain the peer's `FIN` before returning
  (never `RST`); enable TCP keepalive with a short interval (~30 s) so the
  W5500 discovers and reclaims half-dead peers on its own.
- Add a **freeze-diagnostic probe** that runs when the existing freeze
  detection (unanswered SLIP pings) fires. The probe opens a **fresh, plain
  TCP connect to `:51200`** — no TLS, no auth — and classifies the outcome
  as `accepted`, `refused`, `reset`, or `timeout`. Two follow-up probes at
  t+120 s and t+300 s check whether the W5500 recycles slots on its own.
- Add a **post-soft-reboot probe** that re-classifies right after the
  existing reactive `GW_REBOOT_REQ` recovery finishes, so we can tell an
  EFM32-task deadlock (soft reboot fixes it) from a W5500 chip hang
  (only mains helps).
- Log all probe outcomes per freeze event with a stable `event_id`, so an
  evidence trail can be built across many freezes before any policy
  change is made.
- **Not in this change**: touching the periodic-reboot cadence, removing the
  power-cycle recovery, or changing the reactive-reboot path. Those stay as
  they are. This change is instrumentation + client-side hygiene only.

## Capabilities

### New Capabilities
- `klf200-freeze-diagnostics`: a client-side probe that classifies freeze
  events by their TCP-layer signature (accepted / refused / stalled), so we
  can distinguish W5500 socket exhaustion from a genuine TLS-state stall.

### Modified Capabilities
- `klf200-client`: connection close SHALL send `FIN` and wait for the peer's
  `FIN` (bounded), never a bare `RST`; the TCP connection SHALL have keepalive
  enabled with an idle/interval short enough (~30 s) to let the W5500 recycle
  a half-dead peer within one keepalive round.

## Impact

- **Code**: KLF200 client connection setup (add `SetKeepAlive` / `SetKeepAlivePeriod`
  and a bounded-wait graceful close) and freeze-detection path (add a probe
  helper invoked when the health-check declares the gateway wedged, before the
  existing recovery kicks in).
- **Logs / observability**: one structured log line per freeze event with the
  probe classification and timing; no new metrics endpoint.
- **Runtime behavior**: no functional change on the happy path. On freeze, the
  probe adds a short (~5 s) diagnostic step before recovery; the existing
  wedge-recovery flow is unchanged.
- **Docker/build/deploy**: none. Pure application-code change.
- **Config**: no new required config. The probe is on by default; a hidden
  toggle can disable it if it ever causes false negatives (config addition
  optional; leave for a follow-up if actually needed).
