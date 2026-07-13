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
- Configurable auto-restart on connection loss or on a fixed interval
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
    "password": "velux123"
  },
  "homeassistant": {
    "prefix": "",
    "invert-awning": false
  },
  "restart": {
    "restart-interval": 24,
    "health-check-interval": 300,
    "restart-on-error": true
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

#### `homeassistant` (optional)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `prefix` | string | `""` | String prepended to every device name in HA, e.g. `"DEV-"` |
| `invert-awning` | bool | `false` | Invert positions and open/closed semantics for awning devices |

#### `restart` (optional)

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `restart-interval` | integer | `24` | Exit and let the process supervisor restart after N hours (0 = disabled) |
| `health-check-interval` | integer | `300` | Send a KLF200 heartbeat every N seconds (0 = disabled) |
| `restart-on-error` | bool | `true` | Exit when a connection error is detected so the supervisor can restart |

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
| `[restart]` | `restart_interval` | `restart.restart-interval` | Unit stays hours |
| `[restart]` | `health_check_interval` | `restart.health-check-interval` | Unit stays seconds |
| `[restart]` | `restart_on_error` | `restart.restart-on-error` | |
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
    "password": "velux123"
  },
  "homeassistant": {
    "prefix": "DEV-",
    "invert-awning": false
  },
  "restart": {
    "restart-interval": 24,
    "health-check-interval": 300,
    "restart-on-error": true
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
