## 1. Client seam

- [ ] 1.1 Add `Reboot(ctx context.Context) error` to the `klfClient` interface in `app/app.go`
- [ ] 1.2 Update the test fake in `app/app_test.go` to implement `Reboot`, with knobs to
      simulate success, immediate error, and delayed connection loss

## 2. Reboot lifecycle in App

- [ ] 2.1 Rename `doReset` → `doReconnect`; keep it as the reusable
      "stop-heartbeat + connectAndLoad + Register + startHeartbeat" primitive
- [ ] 2.2 Add `doReboot(ctx)` that: acquires `resetMu`, pauses heartbeat, calls `klf.Reboot(ctx)`
      (log-and-continue on error), then runs the exponential-backoff reconnect loop
      (2s, 5s, 10s, 30s, 60s, 120s, 120s; cap 120s; total budget 5min) and finally re-runs
      `connectAndLoad` + `cm.Register` + `startHeartbeatFn`
- [ ] 2.3 Extract the backoff schedule to package-level `var rebootBackoffSchedule` so tests can
      override it to `[]time.Duration{0}` and avoid real sleeps
- [ ] 2.4 Add a package-level `var rebootBudget = 5 * time.Minute` for the same reason

## 3. Wire triggers

- [ ] 3.1 In `App.Start`, after the initial `connectAndLoad` + `cm.Register` + `startHeartbeatFn`,
      call `doReboot` and log "Rebooting KLF200 on startup". Return the error from `doReboot` so
      startup fails non-zero if the post-reboot reconnect budget is exhausted
- [ ] 3.2 In `startResetLoop`, rename to `startRebootLoop`; on each tick call `doReboot` instead
      of `doReset`; keep the log line but change to "Periodic reboot triggered"
- [ ] 3.3 In `checkHealth`, after the successful reconnect branch that clears `wedged`, call
      `doReboot` (log-and-warn on error; keep availability restored)

## 4. Tests (`app/app_test.go`)

- [ ] 4.1 Update all existing tests that reference `doReset` to `doReconnect`
- [ ] 4.2 Add test: `doReboot` happy path — fake `Reboot` returns nil, first backoff attempt
      succeeds → heartbeat restarted, covers still available
- [ ] 4.3 Add test: `doReboot` with slow gateway — first N reconnect attempts return err, the
      Nth+1 succeeds → returns nil, heartbeat restarted
- [ ] 4.4 Add test: `doReboot` budget exhausted — all reconnect attempts fail →
      returns error, heartbeat NOT restarted
- [ ] 4.5 Add test: `doReboot` when `Reboot` itself errors — still proceeds to reconnect loop
- [ ] 4.6 Add test: startup path calls `doReboot` after initial connect (assert order via a
      recorded call log on the fake)
- [ ] 4.7 Add test: periodic loop calls `doReboot` per tick
- [ ] 4.8 Add test: `checkHealth` post-recovery calls `doReboot`; second call while first is
      running is serialized via `resetMu` (not concurrent)

## 5. Docs

- [ ] 5.1 Update `README.md` "Restart / health" section: describe device-reboot lifecycle,
      startup reboot, ~60–90s downtime per cycle, docker `start_period` recommendation
- [ ] 5.2 Update the top-level project description if it still mentions "clean session reset"

## 6. Verify

- [ ] 6.1 `go build ./...` succeeds
- [ ] 6.2 `go test ./...` passes
- [ ] 6.3 `golangci-lint run` (or equivalent from `.golangci.yml`) passes
- [ ] 6.4 Manual smoke: no `KLF200_LIVE` change required — reboot is exercised by unit tests

## 7. Release

- [ ] 7.1 Conventional commit(s): `feat(app): reboot KLF200 on startup and periodically to clear zombie session slots`, plus a `BREAKING CHANGE:` footer noting the semantic change of `restart-interval` (minor bump due to config-semantics change; not major since the field name is unchanged and behaviour is strictly more reliable)
- [ ] 7.2 Push to `main`
- [ ] 7.3 Confirm release workflow tags the new minor version (per `chore(build): add release targets driven by conventional commits` in recent history)

## 8. Archive

- [ ] 8.1 After deploy / verification, archive this OpenSpec change via `/opsx:archive`
