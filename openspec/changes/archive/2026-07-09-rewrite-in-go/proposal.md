## Why

The Velux KLF200→MQTT bridge (`vlxmqttha`) is a Python application that depends on an
asyncio event loop, a custom fork of the `pyvlx` library (as a git submodule), and several
threading workarounds (`call_async_blocking`, semaphores, `run_coroutine_threadsafe`) to
bridge synchronous MQTT callbacks with async KLF200 I/O. Reimplementing it in Go — mirroring
the sibling `miele-to-mqtt-gw` project — produces a single static distroless binary, removes
the Python runtime and submodule friction, and lets Go's native concurrency model replace the
threading hacks with goroutines and channels (a far better fit for a persistent
socket + callback bridge). It also aligns this bridge with the shared smart-home stack
(`go-logger`, `mqtt-gateway`, Makefile/goreleaser/distroless).

## What Changes

- **BREAKING**: Complete rewrite of the application from Python to Go. Python, the asyncio
  loop, and the `pyvlx` git submodule are no longer required at runtime.
- **BREAKING**: Configuration format changes from INI (`vlxmqttha.conf`) to **JSON with
  `${ENV}` substitution**, matching the `miele-to-mqtt-gw` template and reusing the shared
  `mqtt-gateway` config plumbing.
- **NEW — full Go port of `pyvlx`**: port the entire `tjaehnel/pyvlx@master_vlxmqttha` fork
  (KLF200 binary protocol over TLS `:51200`, SLIP framing) to a standalone, reusable Go
  library — not just the cover subset. This includes the `set_limitation`/`get_limitation`
  extension (keep-open feature) and live position-change notifications that are unique to that
  fork.
- Replace Python dependencies with Go equivalents:
  - `paho-mqtt` + `homeassistant-mqtt-binding` (ha_mqtt) → shared `mqtt-gateway`
    (`eclipse/paho.mqtt.golang`) + hand-built HA discovery payloads (as miele does).
  - `pyvlx` submodule → the new in-repo `klf200` Go package.
  - asyncio event loop + threading → goroutines, channels, `sync`/`context`.
  - `configparser` (INI) → JSON config loader with env-var substitution.
  - Python `logging` + `RotatingFileHandler` → shared `go-logger`.
- Preserve the **KLF200 session-reset resilience algorithm** (see `design.md`): the KLF200
  allows only two API sessions that are not freed on unclean disconnect, so the bridge performs
  a periodic **clean** disconnect/reconnect to release and reacquire the session slot, and
  makes a wedged gateway visible (HA availability offline + clear log) so the operator can
  power-cycle it manually.
- Maintain **100% MQTT topic and message compatibility** with the Python version: same
  `{prefix}{id}/state`, `{prefix}{id}/position`, `{prefix}{id}/set`,
  `{prefix}{id}-keepopen/state|set`, and `homeassistant/.../config` discovery topics with the
  same payload semantics (`open`/`closed`/`opening`/`closing`, position `0-100`, `ON`/`OFF`).
- Preserve behavioral features: per-cover HA **device** (not just entities), keep-open switch,
  live opening/closing states, awning inversion, umlaut-normalized MQTT ids.
- Add a Makefile, multi-stage distroless Dockerfile, and goreleaser config mirroring miele.

## Capabilities

### New Capabilities

- `klf200-protocol`: The low-level KLF200 protocol library (full pyvlx port) — TLS `:51200`
  connection, SLIP tokenizing/framing, GW_* frame (de)serialization, session-id generation,
  and heartbeat. Standalone and reusable.
- `klf200-client`: High-level device layer over the protocol — password authentication, node
  discovery/loading, node type model (all node classes), command send (open/close/stop/set
  position), position limitations (keep-open), and live node-state-change subscriptions.
- `velux-cover-state`: Translation of KLF200 node position/target-position into the MQTT cover
  state model (`open`/`closed`/`opening`/`closing`) and position, including awning inversion.
- `mqtt-bridge`: Bidirectional MQTT — publishing retained cover state/position/availability and
  subscribing to `/set` and `keepopen/set` command topics; bridge status reporting.
- `ha-discovery`: Home Assistant MQTT auto-discovery payloads for each cover (as a device) plus
  its keep-open switch entity, and their cleanup on shutdown.
- `app-config`: JSON configuration loading with `${ENV}` substitution, defaults, and validation
  of required fields (mqtt, velux).
- `app-runtime`: Application bootstrap/shutdown, the KLF200 session-reset resilience algorithm
  (heartbeat, contact tracking, periodic clean reset, wedge detection), structured logging, and
  graceful shutdown.
- `build-and-deploy`: Makefile-driven build/test/lint/run workflow, multi-stage distroless
  Docker image, and goreleaser configuration.

### Modified Capabilities

<!-- None — openspec/specs/ is empty; this is the foundational change. -->

## Impact

- **Affected code**: The entire Python tree (`vlxmqttha.py`, `mqtt_cover.py`,
  `mqtt_switch_with_icon.py`, `requirements.txt`, `mypy.ini`, the `mod/pyvlx` submodule) is
  replaced by a Go module. Target layout mirrors miele: `app/` (or repo root) with `main.go`,
  `app.go`, `config/`, `klf200/` (protocol + client), `bridge/` (publisher + discovery),
  `metrics/`, `version/`.
- **Config**: `vlxmqttha.conf` (INI) → `config.json` (JSON + `${ENV}`). Existing deployments
  must migrate their config; the MQTT contract itself is unchanged.
- **Dependencies**: All Python/pip deps and the pyvlx submodule removed. New Go deps:
  `paho.mqtt.golang` (via `mqtt-gateway`), `go-logger`, and (optionally) a SLIP helper —
  though a from-scratch SLIP implementation keeps the port self-contained.
- **Build/CI**: `Dockerfile`, `docker-compose.yml`, and GitHub Actions move from Python to Go
  (`go build`/`go test`, distroless image, goreleaser). PID-file single-instance handling is
  dropped in favor of the supervisor.
- **Runtime**: No Python at runtime; lower memory footprint; single binary.
- **Documentation**: `README.md` rewritten for Go build/run and JSON config.
