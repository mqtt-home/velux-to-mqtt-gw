## ADDED Requirements

### Requirement: TCP keepalive on the KLF200 connection

The KLF200 TCP connection SHALL have `SO_KEEPALIVE` enabled with a keepalive
period short enough that the peer (a WIZnet W5500 with a fixed pool of eight
TCP sockets) can detect and reclaim a dead client within one keepalive round
— on the order of 30 seconds. This reduces the risk that a client failure
leaves a socket parked in a half-open state on the device.

#### Scenario: Keepalive enabled at connect time

- **WHEN** the KLF200 client establishes its TCP connection
- **THEN** the connection has TCP keepalive enabled with a keepalive period
  of approximately 30 seconds

#### Scenario: Keepalive survives across reconnects

- **WHEN** the client reconnects after any teardown (graceful, error, or
  post-reboot)
- **THEN** the new connection is configured with the same keepalive settings

### Requirement: Graceful close sends FIN and drains the peer's FIN

The client SHALL, on any graceful close of the KLF200 TCP connection
(shutdown, planned reconnect, graceful teardown after a reboot request),
initiate a half-close by sending `FIN` (via `CloseWrite`), read-drain the
peer for a bounded interval to consume the peer's `FIN`, and only then
fully close the socket. The client SHALL NOT force `RST`-style close (e.g.
via `SO_LINGER=0`) on the graceful path.

#### Scenario: Planned shutdown closes gracefully

- **WHEN** the application shuts down cleanly
- **THEN** the client sends `FIN`, drains until the peer's `FIN` or the
  bounded read-drain expires (≤ 2 s), and then fully closes the socket

#### Scenario: Emergency teardown may skip the drain

- **WHEN** the connection is being torn down after the health check
  declared the gateway wedged, or after a watchdog fires, or on hard error
- **THEN** the client MAY close without waiting for the peer's `FIN`,
  since the peer is presumed unresponsive
