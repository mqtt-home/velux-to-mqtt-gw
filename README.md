# velux-to-mqtt-gw — Velux KLF200 to MQTT Bridge (Go)

A Go rewrite of the Python [vlxmqttha](https://github.com/tjaehnel/vlxmqttha) bridge. Exposes
Velux KLF200 cover devices over MQTT with Home Assistant auto-discovery, live position/state
updates, the keep-open limitation feature, and a built-in watchdog for unattended operation.

> **Note:** Only cover devices (shutters, awnings, roller blinds, windows) are supported.

---

## Features

- One Home Assistant device per cover, with auto-discovery via MQTT
- Live `open`/`closed`/`opening`/`closing` state and `0–100` position reporting
- Keep-open switch per cover: limits how far the cover may close
- Automatic health monitoring: periodic heartbeat to the KLF200
- Device-level KLF200 reboot on startup, periodically, and after a wedge —
  freeing both API session slots (KLF200 has only 2 and never garbage-collects
  them), so the bridge self-heals from zombie-slot lockups
- Single static binary, no runtime dependencies; multi-arch Docker image (amd64, arm64, armv6, armv7)

---

## Configuration

The bridge is configured with a single JSON file passed as the first command-line argument.

### Full example

See [`app/config-example.json`](app/config-example.json) for a ready-to-copy template.

```json
{
  "mqtt": {
    "url": "tcp://192.168.1.10:1883",
    "username": "mqttuser",
    "password": "mqttpassword",
    "client-id": "velux-to-mqtt-gw",
    "retain": true,
    "qos": 1,
    "topic": "velux"
  },
  "velux": {
    "host": "192.168.1.50",
    "password": "velux123",
    "connect-timeout-seconds": 45
  },
  "homeassistant": {
    "prefix": "",
    "invert-awning": false
  },
  "restart": {
    "restart-interval": 24,
    "restart-at": "",
    "health-check-interval": 300,
    "restart-on-error": true,
    "power-cycle-topic": "",
    "power-cycle-off-payload": "OFF",
    "power-cycle-on-payload": "ON",
    "power-cycle-off-seconds": 10,
    "periodic-power-cycle": false
  },
  "loglevel": "info"
}
```

### Field reference

#### `mqtt` (required)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `url` | string | — | MQTT broker URL, e.g. `tcp://host:1883` or `ssl://host:8883` |
| `username` | string | `""` | Broker login name (optional) |
| `password` | string | `""` | Broker password (optional) |
| `client-id` | string | `""` | MQTT client identifier (optional; broker assigns one if empty) |
| `retain` | bool | `true` | Publish state messages with the MQTT retain flag |
| `qos` | integer | `1` | MQTT QoS level (0, 1, or 2) |
| `topic` | string | `""` | Base topic prefix for all device topics |

#### `velux` (required)

| Field | Type | Description |
|-------|------|-------------|
| `host` | string | IP address of the KLF200 gateway |
| `password` | string | KLF200 WiFi/API password |
| `connect-timeout-seconds` | integer | Deadline for one bring-up attempt (TLS dial + handshake + password + node load). Default `45`; `0` disables it |

> **Why the bring-up needs a deadline.** The gateway's freeze mode accepts the TCP
> connection and then never finishes the TLS handshake. With no deadline the dial
> blocks forever: the process neither fails nor exits, so nothing restarts it and
> no recovery path is reached — a bridge was observed sitting in
> `Connecting to KLF200` for minutes with zero restarts while the covers were
> dead. With the timeout the attempt fails, which is what lets the power-cycle
> escalation (or the supervisor) act.

#### `homeassistant` (optional)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `prefix` | string | `""` | String prepended to every device name in HA, e.g. `"DEV-"` |
| `invert-awning` | bool | `false` | Invert positions and open/closed semantics for awning devices |

#### `restart` (optional)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `restart-interval` | integer | `24` | Reboot the KLF200 device (via `GW_REBOOT_REQ`) every N hours; frees both API session slots. `0` disables the periodic reboot (the startup reboot still runs). Ignored when `restart-at` is set |
| `restart-at` | string | `""` | Run the periodic restart at a wall-clock time of day, `"HH:MM"` in 24h form and **local** time (set `TZ`), e.g. `"02:00"`. Replaces `restart-interval` as the trigger. A malformed value is rejected at startup |
| `health-check-interval` | integer | `300` | Send a KLF200 heartbeat every N seconds (0 = disabled) |
| `restart-on-error` | bool | `true` | On a wedge (no KLF200 contact within `health-check-interval × 2`), reconnect and then reboot the gateway to clear any zombie session slot |
| `power-cycle-topic` | string | `""` | MQTT topic of a smart plug powering the KLF200. **Empty disables the whole power-cycle feature.** |
| `power-cycle-off-payload` | string | `"OFF"` | Payload published to cut power |
| `power-cycle-on-payload` | string | `"ON"` | Payload published to restore power |
| `power-cycle-off-seconds` | integer | `10` | Dwell with power removed, long enough for the gateway's capacitors to drain |
| `periodic-power-cycle` | bool | `false` | Make the `restart-interval` loop **cut mains** instead of sending `GW_REBOOT_REQ`. Requires `power-cycle-topic`; without it the soft reboot is used and a warning is logged once |

> **Automatic power-cycle recovery (optional).** The KLF200's most common failure
> mode is a network-subsystem freeze after roughly a day of an open connection:
> TLS handshakes stop completing and the gateway accepts no new sessions. A soft
> `GW_REBOOT_REQ` does **not** clear it — the device re-accepts TCP within ~2 s,
> far too fast for a genuine boot, while the covers stay dead. Only cutting mains
> recovers it.
>
> If the gateway is wired through an MQTT-controllable plug, set
> `power-cycle-topic` and the bridge will do it by itself: as the **last** step of
> wedge recovery (after a failed reconnect or failed soft reboot) it publishes the
> off payload, waits `power-cycle-off-seconds`, publishes the on payload, then
> re-runs the full connect → authenticate → load → register → heartbeat sequence
> using the same backoff schedule as a soft reboot. Without the topic, behaviour
> is exactly as before: the wedge is logged as `manual power-cycle required`.
>
> Two safeguards worth knowing: the publishes are **never retained** (a retained
> OFF would strand the plug off across a broker restart, leaving the gateway dead
> with nobody to publish the ON), and repeat cycles are **debounced to 15 min**
> so a gateway that never returns does not get its mains toggled on every
> health-check tick. Cancelling the process mid-cycle restores power on the way
> out.
>
> Per-plug examples:
>
> | Plug | `power-cycle-topic` | off / on |
> |---|---|---|
> | [hs100-to-mqtt-gw](https://github.com/mqtt-home/hs100-to-mqtt-gw) | `hs100/<name>/set` | `OFF` / `ON` |
> | Tasmota | `cmnd/<device>/POWER` | `OFF` / `ON` |
> | Shelly Gen1 | `shellies/<id>/relay/0/command` | `off` / `on` |
> | Shelly Gen2 | `shellies/<id>/command/switch:0` | `off` / `on` |
> | zigbee2mqtt | `zigbee2mqtt/<name>/set` | `{"state":"OFF"}` / `{"state":"ON"}` |
>
> Do not point this at the plug's **state** topic — it must be the command topic,
> or the bridge will publish into a topic the plug only reads back.

> **Prophylactic hard restart (`periodic-power-cycle`).** On some gateways the
> soft reboot does not clear the freeze at all — the tell is that the device
> re-accepts TCP a second or two after `GW_REBOOT_REQ`, far too fast to have
> actually restarted, while the covers stay unresponsive. There, waiting for the
> wedge and then recovering means the covers are dead for up to
> `health-check-interval × 2` every time. Setting `periodic-power-cycle: true`
> turns the existing `restart-interval` maintenance into a mains cycle, so the
> gateway is restarted for real before it ever wedges.
>
> Behaviour details: a scheduled cycle that fails falls back to the soft reboot
> (better than skipping maintenance), and one that lands within 15 minutes of a
> wedge-triggered cycle is skipped rather than repeated. Both are logged.
>
> The escalation also runs **during startup**: if the very first bring-up fails
> and a plug is configured, the bridge power-cycles the gateway and retries once
> before giving up. Without that, a gateway that was already frozen when the
> bridge started could never be recovered by the bridge itself — the health-check
> and periodic loops only start after a successful bring-up.
>
> This is safe for covers in motion: io-homecontrol actuators execute a movement
> themselves once commanded, so de-powering the *gateway* mid-travel costs
> position feedback, not the movement.

> **Interval or time of day?** `restart-interval` counts from process start, so
> every redeploy moves the maintenance window — "every 24 h" quietly becomes
> "whenever the container last restarted", which on a busy day can mean the middle
> of the afternoon. `restart-at: "02:00"` pins it to a quiet hour instead. The next
> slot is recomputed after every run, so it survives DST: on the spring-forward
> night 02:00 does not exist and the restart lands at 03:00 once, on the
> fall-back night the earlier 02:00 is used — never skipped, never twice.
>
> **Startup and reboot timing.** The bridge always reboots the KLF200 once
> during startup to clear any zombie session slots left by a previous unclean
> shutdown. This adds ~60–90 s to the initial bring-up. Container
> orchestrators (Docker `healthcheck`, Kubernetes readiness probes) should
> use a `start_period` / `initialDelaySeconds` of at least **120 s**.
> Each periodic reboot causes the same ~60–90 s window during which HA covers
> are marked unavailable; the default 24 h interval keeps this to once per day.

#### `loglevel` (optional)

String log level: `"debug"`, `"info"` (default), `"warn"`, or `"error"`.

Environment variable placeholders in the form `${VARNAME}` are substituted before parsing,
so secrets can be injected without storing them in the file:

```json
{
  "mqtt": { "password": "${MQTT_PASSWORD}" },
  "velux": { "password": "${VELUX_PASSWORD}" }
}
```

---

## Building and running

### Prerequisites

- Go 1.22 or later
- `make` (optional but recommended)
- Docker (optional)

### Build from source

```bash
cd app
make build          # produces build/velux-to-mqtt-gw
make test           # run unit tests
make lint           # run golangci-lint
```

### Run directly

```bash
make run CONFIG=/path/to/config.json
# or
./build/velux-to-mqtt-gw /path/to/config.json
```

### Build and run with Docker

```bash
cd app
make image          # builds mqtt-home/velux-to-mqtt-gw:latest

docker run -d \
  --name velux-to-mqtt-gw \
  --restart unless-stopped \
  -v /path/to/config.json:/var/lib/velux-to-mqtt-gw/config.json:ro \
  mqtt-home/velux-to-mqtt-gw:latest
```

### Docker Compose

```yaml
services:
  velux-to-mqtt-gw:
    image: mqtt-home/velux-to-mqtt-gw:latest
    restart: unless-stopped
    volumes:
      - ./config.json:/var/lib/velux-to-mqtt-gw/config.json:ro
```

```bash
docker compose up -d
docker compose logs -f
docker compose down
```

Multi-arch images (amd64, arm64, armv6, armv7) are built with
[GoReleaser](https://goreleaser.com) and the `.goreleaser.yml` in the `app/` directory.

---

## MQTT topics and payloads

All topics below use `{device-id}` as the slug derived from the KLF200 node name.
The optional `mqtt.topic` prefix is prepended (e.g. `velux/{device-id}/state`).

### Home Assistant auto-discovery

| Topic | Description |
|-------|-------------|
| `homeassistant/cover/{prefix}{device-id}/config` | Cover entity configuration (published once at startup) |
| `homeassistant/switch/{prefix}{device-id}-keepopen/config` | Keep-open switch configuration |

### State publishing (retained)

| Topic | Payload | Description |
|-------|---------|-------------|
| `{device-id}/state` | `open` / `closed` / `opening` / `closing` | Current cover state |
| `{device-id}/position` | `0`–`100` | Current position in percent (0 = open, 100 = closed) |
| `{device-id}-keepopen/state` | `on` / `off` | Keep-open active (`on`) or inactive (`off`) |

### Commands (subscribe)

| Topic | Payload | Description |
|-------|---------|-------------|
| `{device-id}/set` | `OPEN` | Move to fully open position |
| `{device-id}/set` | `CLOSE` | Move to fully closed position |
| `{device-id}/set` | `STOP` | Stop current movement immediately |
| `{device-id}/set` | `0`–`100` | Move to the given position in percent |
| `{device-id}-keepopen/set` | `ON` | Activate keep-open limit (cover cannot close fully) |
| `{device-id}-keepopen/set` | `OFF` | Clear keep-open limit (full range restored) |

### Example commands

```bash
# Open a cover
mosquitto_pub -h 192.168.1.10 -t "velux/vlx-living-room-shutter/set" -m "OPEN"

# Move to 50 % closed
mosquitto_pub -h 192.168.1.10 -t "velux/vlx-living-room-shutter/set" -m "50"

# Activate keep-open
mosquitto_pub -h 192.168.1.10 -t "velux/vlx-living-room-shutter-keepopen/set" -m "ON"

# Monitor all state topics
mosquitto_sub -h 192.168.1.10 -t "velux/#"
```

---

## Migrating from the Python version

The Go rewrite is a drop-in functional replacement. The only change you need to make is
converting the `vlxmqttha.conf` INI file to a `config.json` JSON file.

### INI → JSON mapping

| INI section | INI key | JSON path | Notes |
|-------------|---------|-----------|-------|
| `[mqtt]` | `host` + `port` | `mqtt.url` | Combine: `"tcp://<host>:<port>"` |
| `[mqtt]` | `login` | `mqtt.username` | |
| `[mqtt]` | `password` | `mqtt.password` | |
| `[velux]` | `host` | `velux.host` | |
| `[velux]` | `password` | `velux.password` | |
| `[homeassistant]` | `prefix` | `homeassistant.prefix` | |
| `[homeassistant]` | `invert_awning` | `homeassistant.invert-awning` | Underscore becomes hyphen |
| `[restart]` | `restart_interval` | `restart.restart-interval` | Unit stays hours. Semantics change: now reboots the KLF200 device instead of just restarting the process |
| `[restart]` | `health_check_interval` | `restart.health-check-interval` | Unit stays seconds |
| `[restart]` | `restart_on_error` | `restart.restart-on-error` | Semantics change: now triggers an in-process reconnect + device reboot instead of a process exit |
| `[log]` | `verbose = true` | `loglevel` | Set to `"debug"` |
| `[log]` | `klf200 = true` | `loglevel` | Set to `"debug"` |
| `[log]` | `logfile` | — | Not supported; redirect stdout in your shell/Docker |

### Example conversion

**vlxmqttha.conf (Python)**

```ini
[mqtt]
host = 192.168.1.10
port = 1883
login = mqttuser
password = mqttpassword

[velux]
host = 192.168.1.50
password = velux123

[homeassistant]
prefix = DEV-
invert_awning = false

[restart]
restart_interval = 24
health_check_interval = 300
restart_on_error = true

[log]
verbose = false
```

**config.json (Go)**

```json
{
  "mqtt": {
    "url": "tcp://192.168.1.10:1883",
    "username": "mqttuser",
    "password": "mqttpassword"
  },
  "velux": {
    "host": "192.168.1.50",
    "password": "velux123",
    "connect-timeout-seconds": 45
  },
  "homeassistant": {
    "prefix": "DEV-",
    "invert-awning": false
  },
  "restart": {
    "restart-interval": 24,
    "restart-at": "",
    "health-check-interval": 300,
    "restart-on-error": true,
    "power-cycle-topic": "",
    "power-cycle-off-payload": "OFF",
    "power-cycle-on-payload": "ON",
    "power-cycle-off-seconds": 10,
    "periodic-power-cycle": false
  },
  "loglevel": "info"
}
```

### Runtime changes

- The binary takes the config file path as its only argument: `velux-to-mqtt-gw config.json`
- MQTT topic structure and payloads are identical to the Python version
- Home Assistant auto-discovery topics and device entities are identical
- The Docker image entry point is `/velux-to-mqtt-gw /var/lib/velux-to-mqtt-gw/config.json`
- Log output goes to stdout only; use Docker log drivers or shell redirection for file logging
