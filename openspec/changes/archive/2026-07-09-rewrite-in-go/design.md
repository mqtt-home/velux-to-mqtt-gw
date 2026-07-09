## Context

`vlxmqttha` bridges devices paired with a Velux KLF200 gateway (io-homecontrol covers: Velux
windows/blinds/awnings, Somfy shutters, etc.) to MQTT, using Home Assistant MQTT auto-discovery.
Unlike `miele-to-mqtt-gw` — which talks to a cloud REST/SSE API over plain `net/http` — the
KLF200 exposes a **proprietary binary protocol over a persistent TLS socket** (`:51200`) with
SLIP (RFC-1055) framing. The Python bridge relies on the `pyvlx` library (a custom
`tjaehnel/pyvlx@master_vlxmqttha` fork, vendored as a git submodule) for that protocol.

Two properties make this port materially different from the miele rewrite:

1. **It is a two-layer port.** miele had no protocol layer; here the protocol library (`pyvlx`)
   must itself be ported to Go before the bridge can be written on top.
2. **It is a bidirectional bridge.** miele mostly pulls cloud data and publishes it. Here MQTT
   `/set` commands flow *back* to physical actuators, and node-state changes flow *out* — the
   data path runs both ways and must stay consistent.

The MQTT contract (topics, payloads, discovery) is the public interface consumed by Home
Assistant and user automations; the rewrite MUST preserve it byte-for-behavior.

### The KLF200 session/reset constraint (central to this design)

The KLF200 permits only **two concurrent API sessions**, and it does **not** free a session
slot on an *unclean* disconnect (process crash, network drop, hard kill). Zombie slots
accumulate; once both slots are zombified, no client — not even a freshly restarted one — can
connect. Key consequences, established with the maintainer:

- A bridge/container restart does **not** recover a wedged gateway: the zombie slots live on the
  KLF200, not in the client. Only a **clean** disconnect (TLS close_notify / TCP FIN) makes the
  gateway release the slot.
- The Python bridge's `restart_interval` (periodic full process restart) reduces hangs precisely
  because its shutdown path disconnects cleanly, releasing + reacquiring the slot before cruft
  accumulates. The *clean disconnect* is the active ingredient — not the process restart itself.
- Once already wedged, the only recovery is a **physical power-cycle** of the gateway, which the
  operator performs **manually** (no automated smart-plug is in use or requested).

## Goals / Non-Goals

**Goals:**

- Port `tjaehnel/pyvlx@master_vlxmqttha` to Go in full as a standalone, reusable `klf200`
  package (all node types, frames, and the `set_limitation` extension), byte-exact on the wire.
- Reimplement the bridge mirroring the `miele-to-mqtt-gw` structure and shared libraries.
- Preserve 100% MQTT/HA contract compatibility with the Python version.
- Preserve the session-reset resilience *behavior* (periodic clean reset; visible wedge state),
  translated to idiomatic Go concurrency.
- Move config to JSON + `${ENV}` (miele-consistent).

**Non-Goals:**

- Automating physical power-cycle recovery (no smart-plug integration).
- Changing the MQTT topic layout or payload semantics.
- Supporting device categories the Python bridge did not (it handled covers/`OpeningDevice`;
  the *library* port is full, but the *bridge* still exposes covers + keep-open switch only).
- A backward-compatible INI config reader (JSON only; users migrate once).

## Decisions

### 1. Full pyvlx port as a dedicated `klf200` Go package

Port the entire fork, not just the cover subset, so the result is a reusable Go KLF200 library.
Map the Python design as follows:

- **Connection** (`connection.py`) → a `Conn` type owning the TLS socket. A single **read-loop
  goroutine** feeds a `SlipTokenizer`, decodes each frame via a `frame_from_raw` equivalent, and
  dispatches to registered handlers over channels. This replaces asyncio's protocol callbacks.
- **Frames** (`api/frames/*`) → one Go type per `GW_*` command implementing a
  `Frame` interface (`Command() Command`, `Marshal() []byte`, `Unmarshal([]byte) error`). A
  `frameCreation` registry maps command codes → constructors. Must be byte-exact against the
  official protocol spec.
- **API events** (`api/*` request/confirm pairs) → a request/response helper keyed by
  session-id, with a per-request channel awaiting the matching confirmation/notification.
- **SLIP** (`slip.py`) → a small self-contained Go implementation (RFC-1055).
- **Session ids** (`session_id.py`) → an atomic counter wrapping at 65535.
- **Heartbeat** (`heartbeat.py`) → a goroutine sending `GW_GET_STATE_REQ` every 60s (plus a
  status request per Blind — the FP3 workaround), used both as keepalive and liveness signal.
- **Node model** (`node.py`, `nodes.py`, `opening_device.py`, `node_helper.py`,
  `node_updater.py`) → node structs with `open/close/stop/set_position`, position limitations,
  and an update-callback registration mechanism (channel or callback func per node).

### 2. Idiomatic Go for the async→concurrency mapping

The Python bridge fights the async/sync boundary (`call_async_blocking`, `Semaphore(2)`,
`run_coroutine_threadsafe`). In Go this dissolves: MQTT command callbacks run on their own
goroutines and issue KLF200 commands directly through the client, whose request/response layer
serializes access to the socket. A bounded worker or mutex around the write path preserves the
original two-in-flight command semantics where needed.

### 3. Session-reset resilience — clean periodic reset, visible wedge

Preserve the algorithm's *effect* using goroutines:

```
heartbeat  (Ticker 60s)   → GW_GET_STATE_REQ; on success stamp lastContact (atomic)
health     (Ticker Nsec)  → if now-lastContact > interval×threshold → mark wedged
reset      (Ticker 24h)   → CLEAN Disconnect() (release slot) → wait → reconnect
                             → reload nodes → re-register callbacks → re-publish discovery
```

- The periodic reset is an **in-process clean disconnect/reconnect**, not a process exit: it
  guarantees a controlled close (releasing the slot), and keeps the MQTT connection and HA
  discovery up so entities do not flap.
- **Wedge handling**: when the health check trips and reconnect keeps failing, publish HA
  availability `offline` for all covers and log a clear, actionable message
  ("KLF200 unreachable — manual power-cycle required"), then keep retrying visibly. There is no
  automated physical reset.
- A hard `os.Exit` without a preceding clean disconnect is explicitly avoided — it would *create*
  the zombie slot the whole algorithm exists to prevent. The PID-file single-instance guard from
  the Python version is dropped (the supervisor owns lifecycle).
- `GW_REBOOT_REQ` (a proactive software gateway reboot while the session is alive) is available
  in the ported library and noted as a possible future stronger reset, but is out of scope here.

### 4. Config: JSON + `${ENV}`, mapped from the INI sections

Mirror miele's loader. Proposed shape (final field names settled during specs):

```json
{
  "mqtt": { "url": "tcp://host:1883", "username": "...", "password": "...", "retain": true },
  "velux": { "host": "192.168.x.x", "password": "<wifi pass>" },
  "homeassistant": { "prefix": "", "invert-awning": false },
  "restart": { "restart-interval": 24, "health-check-interval": 300, "restart-on-error": true },
  "loglevel": "info"
}
```

### 5. Preserve the MQTT/HA contract exactly

Topics and payloads match the Python version: `{prefix}{id}/state` (`open`/`closed`/`opening`/
`closing`), `{prefix}{id}/position` (`0-100`), `{prefix}{id}/set` (`OPEN`/`CLOSE`/`STOP`/`0-100`),
`{prefix}{id}-keepopen/state|set` (`ON`/`OFF`), `{prefix}{id}/available` (`online`/`offline`),
and the `homeassistant/cover|switch/.../config` discovery payloads (per-cover HA *device*,
device-class mapped from node type, position_open/closed swapped when inverted). MQTT ids are the
umlaut-normalized, lowercased, hyphenated node name prefixed with `vlx-`.

## Risks / Trade-offs

- **Byte-exact protocol port is the dominant risk.** Each GW_* frame must serialize identically
  to pyvlx/the spec, and parameter encoding (`parameter.py` — position ↔ percent, the `0xC800`
  special values) is fiddly. Mitigation: port `parameter.py` and frame (de)serialization with
  golden-byte unit tests derived from pyvlx, and test against a real KLF200 early.
- **Live-update semantics.** Correct `opening`/`closing` reporting depends on
  `GW_NODE_STATE_POSITION_CHANGED_NTF` + house-status-monitor + target-position tracking behaving
  like the fork. Mitigation: port `node_updater.py` faithfully and verify against hardware.
- **No golden reference for the full library.** Porting *all* of pyvlx (scenes, dimmers, etc.)
  expands surface and test burden beyond what the bridge exercises. Trade-off accepted for
  reusability; those paths get lighter coverage than the cover path.
- **Clean-disconnect assumption.** The resilience design assumes a TLS/TCP clean close reliably
  frees the KLF200 slot. If firmware behaves otherwise, the periodic reset may be less effective
  and manual power-cycles more frequent — a hardware limitation, not a code defect.
- **Config migration is breaking.** Existing `.conf` users must rewrite config as JSON; mitigated
  by documenting a mapping table in the README.
