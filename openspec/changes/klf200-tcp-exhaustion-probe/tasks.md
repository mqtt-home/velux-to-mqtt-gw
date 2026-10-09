## 1. TCP hygiene on the client connection

- [x] 1.1 In the KLF200 TCP dial path, enable `SO_KEEPALIVE` and set the keepalive period to ~30 s via `net.TCPConn.SetKeepAlive(true)` + `SetKeepAlivePeriod(30 * time.Second)`; apply the same settings on every reconnect
- [x] 1.2 Add a `gracefulClose(conn *net.TCPConn) error` helper: `CloseWrite()` → `Read` into a discard buffer with a 2 s deadline until EOF or timeout → `Close()`; document that it MUST NOT be called on the emergency teardown path
- [x] 1.3 Route every disconnect — normal shutdown teardown, planned reconnect, and the wedge-recovery reconnect — through `gracefulClose`; its drain is bounded (2 s) and dead-peer-safe, so no separate fast-`Close()` path is needed for the wedge/watchdog case
- [x] 1.4 Add a targeted unit test that verifies keepalive is set on a real `*net.TCPConn` returned by the dial helper (use `net.Listen` + `net.Dial` locally), and a test that verifies `gracefulClose` returns within its bounded time even when the peer never sends `FIN`

## 2. Freeze diagnostic probe

- [x] 2.1 Add a `freezeProbe(ctx, addr) (outcome string, elapsed time.Duration, dialErr error)` function that performs a plain context-bounded TCP connect (5 s) and classifies the result into exactly one of `accepted`, `refused`, `reset`, `timeout`, `error` (map `syscall.ECONNREFUSED` → `refused`, `syscall.ECONNRESET` → `reset`, `os.IsTimeout(err)`/`context.DeadlineExceeded` → `timeout`, `err == nil` → `accepted` and immediately `Close()` the probe connection; any other error → `error`, returning the raw `dialErr` for the caller to log)
- [x] 2.2 Wire the probe into the freeze-detection path: at the point where the health check declares the gateway wedged (before the reconnect goroutine is released), synchronously run the initial probe with a 5 s cap
- [x] 2.3 When the initial probe returns `timeout`, schedule two follow-up probes on a separate goroutine at t+120 s and t+300 s from the initial probe start; each follow-up uses the same `freezeProbe` classifier
- [x] 2.4 Assign a stable `event_id` per freeze event and emit one structured log line per probe result with keys `event_id`, `stage` (`initial` / `follow_up_120s` / `follow_up_300s` / `post_soft_reboot`), `freeze_probe`, and `t_ms` (plus `probe_err` with the raw dial-error text when the outcome is `error`); a single `grep event_id=<id>` should return every data point for that freeze
- [x] 2.5 After the existing reactive `GW_REBOOT_REQ` recovery completes and the client has reconnected, run one more `freezeProbe` and log it with `stage=post_soft_reboot` under the same `event_id`; do not gate the recovery on the result (log-only)
- [x] 2.6 Unit-test the classifier with a fake listener that (a) accepts immediately, (b) is not listening at all (→ `refused`), (c) blackholes SYN (→ `timeout` via a firewall-drop stub or a context with tiny timeout), and (d) accepts then closes with RST

## 3. Verification

- [x] 3.1 Confirm the existing test suite still passes and that no existing recovery test observes changed behavior (the probe is log-only)
- [ ] 3.2 Run the gateway against a real KLF200 for at least 24 h; inspect logs for `freeze_probe=` entries; confirm the log shape is grep-friendly and the follow-up correlation works
- [x] 3.3 Confirm no change to the periodic-reboot cadence, reactive reboot, or power-cycle recovery in behavior or timing
