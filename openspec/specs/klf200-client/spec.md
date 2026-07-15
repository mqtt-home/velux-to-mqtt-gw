# klf200-client Specification

## Purpose
TBD - created by archiving change rewrite-in-go. Update Purpose after archive.
## Requirements
### Requirement: Password authentication

The system SHALL authenticate to the KLF200 with the configured API password before issuing any
other command, and SHALL fail startup if authentication is rejected.

#### Scenario: Successful authentication

- **WHEN** the client connects and sends the configured password
- **THEN** the gateway confirms authentication and the client proceeds to load nodes

#### Scenario: Rejected password

- **WHEN** the gateway rejects the password
- **THEN** the client reports an authentication error to the caller

### Requirement: Node discovery and model

The system SHALL load all nodes known to the gateway and represent each with its concrete node
type (Window, Blind, Awning, RollerShutter, GarageDoor, Gate, Blade, and the remaining pyvlx node
types), exposing at least name, node id, current position, and target position.

#### Scenario: Load nodes on connect

- **WHEN** the client has authenticated
- **THEN** it requests all node information and populates the node list with typed nodes

#### Scenario: Typed opening devices

- **WHEN** a loaded node is an opening device
- **THEN** its concrete type is available for device-class mapping and its position/target
  position are readable

### Requirement: Command send

The system SHALL send open, close, stop, and set-position commands to an opening device, with an
option to return immediately or wait for completion.

#### Scenario: Set position

- **WHEN** the caller requests a position of N percent for a node
- **THEN** the client sends the corresponding command send frame with the encoded position

#### Scenario: Stop command

- **WHEN** the caller issues stop for a node
- **THEN** the client sends the stop command for that node

### Requirement: Position limitations (keep-open)

The system SHALL set and clear position limitations for an opening device, enabling the keep-open
feature, and SHALL read the current limitation for a node.

#### Scenario: Set limitation

- **WHEN** the caller sets a position limitation (min/max) for a node
- **THEN** the client sends `GW_SET_LIMITATION_REQ` with the encoded bounds and receives
  confirmation

#### Scenario: Clear limitation

- **WHEN** the caller clears the limitation for a node
- **THEN** the client removes the limitation so the node may reach its full range

#### Scenario: Read limitation

- **WHEN** the caller requests the limitation for a node
- **THEN** the client returns the node's current maximum limitation position

### Requirement: Live node-state subscription

The system SHALL deliver live node position and target-position updates to registered per-node
callbacks, driven by node-state-position-changed notifications and house-status monitoring.

#### Scenario: Position change notification

- **WHEN** the gateway reports a node's position changed
- **THEN** the node's registered update callback is invoked with the new position and target
  position

#### Scenario: Subscription re-established after reset

- **WHEN** the connection is cleanly reset and nodes are reloaded
- **THEN** per-node callbacks are re-registered so updates continue to be delivered

### Requirement: Gateway reboot command

The system SHALL expose a `Reboot(ctx)` operation that sends `GW_REBOOT_REQ` (0x0001) and waits
for `GW_REBOOT_CFM` (0x0002). Callers MUST treat the connection as lost after a successful
confirmation, since the gateway drops the TCP session as part of the reboot.

#### Scenario: Reboot confirmation received

- **WHEN** the caller invokes `Reboot(ctx)` on an authenticated connection
- **THEN** the client sends `GW_REBOOT_REQ`, receives `GW_REBOOT_CFM`, and returns nil to the
  caller

#### Scenario: Connection lost after reboot

- **WHEN** the gateway has confirmed the reboot request
- **THEN** the caller SHALL close its connection state and reconnect from scratch (re-authenticate,
  re-load nodes) once the gateway is reachable again

