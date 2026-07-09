## 1. Project scaffolding

- [x] 1.1 Create `go.mod` (module path mirroring the GitHub repo, current stable Go), mirroring the miele layout *(module `github.com/mqtt-home/velux-mqtt-gw`, Go 1.26, under `app/`)*
- [x] 1.2 Add `main.go`: single config-file arg, logger init, config load, bootstrap, SIGINT/SIGTERM handling *(`app/main.go` + `app/app.go`)*
- [x] 1.3 Set up directory layout: `config/`, `klf200/` (protocol + client), `bridge/` (publisher + discovery), `metrics/`, `version/` *(config/, klf200/, klf200/protocol/, version/ created; bridge/ + metrics/ added by their workflows)*
- [x] 1.4 Add `Makefile` (`build`, `test`, `lint`, `run CONFIG=...`, `image`, `clean`); build uses `-ldflags="-s -w" -trimpath`
- [x] 1.5 Add `.golangci.yml` matching the sibling project

## 2. klf200-protocol (pyvlx port — low level)

- [x] 2.1 Port `slip.py` → SLIP pack/tokenize with fragmented-read reassembly; golden round-trip tests
- [x] 2.2 Port `const.py` → command codes, node types, status enums
- [x] 2.3 Port `parameter.py` → position ↔ percent encoding incl. special values (0xC800 etc.); golden tests
- [x] 2.4 Port frame base + `frame_helper` + `frame_creation` → `Frame` interface + command→constructor registry *(decentralized `RegisterFrame`/`FrameFromRaw`, byte-exact framing + XOR CRC)*
- [x] 2.5 Port all `api/frames/*` GW_* frames with byte-exact Marshal/Unmarshal; golden-byte tests per frame
- [x] 2.6 Port `connection.py` → TLS `:51200` conn, read-loop goroutine, frame dispatch, clean disconnect *(`klf200/connection.go`: `Conn` w/ `Disconnect()`, `Lost()`)*
- [x] 2.7 Port `session_id.py` → atomic wrapping counter (tests incl. wraparound)
- [x] 2.8 Port `api/*` request/confirm event layer → session-id-keyed request/response helper *(`klf200/event.go`: `ApiEvent`/`DoAPICall`)*
- [x] 2.9 Port `heartbeat.py` → 60s GetState pulse (+ per-Blind status request), liveness signal *(`klf200/heartbeat.go`, tested)*

## 3. klf200-client (pyvlx port — high level)

- [x] 3.1 Port password authentication (`password_enter`) *(`klf200/auth_system.go`)*
- [x] 3.2 Port node model: `node.py`, `nodes.py`, `node_helper.py`, all node classes; `opening_device.py` open/close/stop/set_position *(registry-based; `opening_device.go` all types, tested)*
- [x] 3.3 Port node loading (`get_all_nodes_information`, `get_node_information`) and gateway facade (`klf200gateway.py`, `pyvlx.py`) *(`node_loading.go`, `client.go`)*
- [x] 3.4 Port command send + status request *(`command.go`, tested)*
- [x] 3.5 Port `set_limitation`/`get_limitation` (keep-open); tests for set/clear/read *(`limitation.go`, tested)*
- [x] 3.6 Port live updates: `node_state_position_changed_notification`, house-status-monitor enable/disable, `node_updater.py`, per-node callback registration *(`node_updater.go`, `house_monitor.go`; fixed a real type-assertion bug, tested)*
- [x] 3.7 Port remaining library surface for reusability (scenes, dimmable/on-off, wink, discovery, network setup, reboot, local time) — lighter test coverage than the cover path *(`other_devices.go`, `scenes.go`, `auth_system.go`)*
- [x] 3.8 Integration smoke test against a real KLF200 (auth → load nodes → command → observe live update) *(`klf200/smoke_integration_test.go`, tag `integration`; PASSED against real gateway: connect→auth→load 4 nodes→clean disconnect. Surfaced+fixed 2 hardware-only bugs: TLS 1.3 hang → cap TLS 1.2; alias-array Go reslice off-by-one → `[103:124]`)*

## 4. Config loader (app-config)

- [x] 4.1 Define `Config` structs per the app-config spec (mqtt, velux, homeassistant, restart, loglevel)
- [x] 4.2 Implement `${ENV}` substitution before unmarshal
- [x] 4.3 Implement defaults + required-field validation (mqtt.url, velux.host, velux.password)
- [x] 4.4 Tests: substitution (set/unset/multiple), defaults, overrides, missing-required error, sample round-trip *(17 tests)*

## 5. velux-cover-state

- [x] 5.1 Implement state derivation (open/closed/opening/closing) from position + target *(`bridge/cover_state.go`)*
- [x] 5.2 Implement position validation/clamping and stopped-when-invalid-target fallback
- [x] 5.3 Implement awning inversion (command mapping + state derivation)
- [x] 5.4 Tests: each state case, invalid ranges, inverted vs non-inverted

## 6. mqtt-bridge (bidirectional)

- [x] 6.1 Wire the shared `mqtt-gateway` connection (retry, retain, bridge status) *(`bridge/mqtt.go`)*
- [x] 6.2 Implement state/position/availability/keepopen publishing to the Python-compatible topics *(`bridge/cover.go`)*
- [x] 6.3 Implement command subscription: `/set` (OPEN/CLOSE/STOP/0-100) and `-keepopen/set` (ON/OFF) → client
- [x] 6.4 Implement MQTT id generation with umlaut transliteration; tests *(`bridge/ids.go`)*
- [x] 6.5 Tests: command parsing incl. invalid payloads, publish paths, id normalization

## 7. ha-discovery

- [x] 7.1 Build per-cover discovery payload (device block, device_class from node type, inverted position mapping) *(`bridge/discovery.go`)*
- [x] 7.2 Build keep-open switch discovery payload (shared device block, icon)
- [x] 7.3 Track published discovery topics and clear them on shutdown *(`CleanupDiscovery`)*
- [x] 7.4 Tests: payload shape per node type, inversion, cleanup

## 8. app-runtime & resilience

- [x] 8.1 Implement bootstrap/shutdown ordering per spec (clean KLF200 disconnect on shutdown) *(`app.go` Stop: CloseAll→clean Disconnect, idempotent)*
- [x] 8.2 Implement contact tracking (atomic) + heartbeat wiring *(`recordContact` stamped on every inbound frame incl. heartbeat confirmations)*
- [x] 8.3 Implement periodic **clean** in-process reset (disconnect → wait → reconnect → reload → re-register → re-publish) keeping MQTT/HA up *(`doReset`, verified no os.Exit, MQTT stays up)*
- [x] 8.4 Implement wedge detection: offline availability + actionable log + continued reconnect attempts *(`checkHealth`, threshold 2.0)*
- [x] 8.5 Implement structured logging + optional verbose KLF200 protocol logging
- [x] 8.6 Tests: reset cycle, wedge detection/recovery, shutdown clean-disconnect ordering *(`app_test.go`, 4 tests)*

## 9. Build & deploy

- [x] 9.1 Multi-stage distroless Dockerfile (binary entrypoint, config path arg)
- [x] 9.2 goreleaser config (multi-arch binaries + images)
- [x] 9.3 Update `docker-compose.yml` for the Go binary
- [x] 9.4 Update GitHub Actions/CI from Python to Go (build/test) *(`.github/workflows/ci.yml`)*

## 10. Documentation & migration

- [x] 10.1 Rewrite `README.md` for Go build/run and JSON config
- [x] 10.2 Add an INI→JSON config mapping table for migrating existing deployments
- [x] 10.3 Provide a `config-example.json`
