## 1. Config

- [ ] 1.1 Add `PowerCycleTopic`, `PowerCycleOffPayload`, `PowerCycleOnPayload`, `PowerCycleOffSeconds` fields to the `restart` struct in `app/config/config.go` with JSON tags `power-cycle-topic`, `power-cycle-off-payload`, `power-cycle-on-payload`, `power-cycle-off-seconds`
- [ ] 1.2 Apply defaults on load: off-payload="OFF", on-payload="ON", off-seconds=10; leave topic empty (feature disabled) by default
- [ ] 1.3 Add a helper (e.g. `Restart.PowerCycleEnabled()`) returning true only when the topic is non-empty
- [ ] 1.4 Extend `app/config/config_test.go` for the new defaults and the enabled/disabled predicate
- [ ] 1.5 Update `app/config-example.json` and the README `restart` field reference with the new fields and per-plug examples (Tasmota, Shelly)

## 2. MQTT publish seam

- [ ] 2.1 Expose a minimal non-retained publish path the App can call for the power-cycle topic (via the existing `bridge`/`mqtt` wiring), with QoS 1 and retain=false
- [ ] 2.2 Add a `publishPowerCycle` seam on `App` (interface method) so the power-cycle logic is unit-testable without a live MQTT broker

## 3. Power-cycle recovery step

- [ ] 3.1 Add `doPowerCycle(ctx)` to `app/app.go`: acquire `resetMu`, stop heartbeat, publish off-payload, sleep `power-cycle-off-seconds`, publish on-payload, then reconnect using `rebootBackoffSchedule` and re-run `connectAndLoad` → `cm.Register` → `startHeartbeatFn`
- [ ] 3.2 Add debounce state (last-power-cycle timestamp via the `now` seam) and a minimum-gap floor so it cannot fire on every health-check tick
- [ ] 3.3 Wire `doPowerCycle` into `checkHealth` as the escalation after `doReconnect`/`doReboot` fail to restore contact, gated on `PowerCycleEnabled()` and `RestartOnError`
- [ ] 3.4 Adjust the wedge log message: "manual power-cycle required" only when the feature is disabled; otherwise log the automated escalation (including the exact topic + payload published)
- [ ] 3.5 Ensure `doPowerCycle` is serialized with `doReconnect`/`doReboot` via `resetMu` and that a persisting wedge falls back to the "still unreachable — waiting" path when debounced

## 4. Tests

- [ ] 4.1 Unit-test the escalation path in `app/app_test.go`: wedged + soft recovery fails + plug configured → power-cycle publishes off/on and re-runs bring-up (using fakes for `klfClient`, `coverManager`, and the publish seam)
- [ ] 4.2 Test the disabled path: no topic configured → no publish, logs "manual power-cycle required", behavior unchanged
- [ ] 4.3 Test debounce: repeated wedge ticks within the window do not re-trigger a power cycle
- [ ] 4.4 Test serialization: concurrent periodic reboot + power-cycle escalation run one-at-a-time
- [ ] 4.5 Test non-retained publish flag on the power-cycle messages

## 5. Verification & docs

- [ ] 5.1 Run `go build ./...`, `go test ./...`, and the linter clean
- [ ] 5.2 Manually verify against a real MQTT-controlled plug (or a mosquitto subscriber standing in for the plug): confirm off→dwell→on publishes and post-boot reconnect
- [ ] 5.3 Confirm an existing config without the new fields runs unchanged (no publishes, identical logs)
