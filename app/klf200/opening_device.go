package klf200

import (
	"context"
	"fmt"

	"github.com/mqtt-home/velux-mqtt-gw/klf200/protocol"
)

// This file ports pyvlx's opening_device.py in full: the OpeningDevice base and
// the concrete types Window, Blind, Awning, RollerShutter, GarageDoor, Gate and
// Blade. Each concrete type self-registers its protocol node-type code(s) via
// RegisterNodeType so node_helper builds the right device from a node
// information notification.
//
// The command/limitation exchanges delegate to the Client helpers in command.go
// and limitation.go (Client.CommandSend / Client.SetLimitation /
// Client.GetLimitation), which are the Go equivalents of pyvlx's CommandSend,
// SetLimitation and GetLimitation ApiEvents.

// OpeningDevice is the meta class for an opening device with one main parameter
// for position (pyvlx opening_device.OpeningDevice). Concrete devices embed it
// by value and add their own AfterUpdate shim.
type OpeningDevice struct {
	BaseNode

	position       protocol.Position
	targetPosition protocol.Position

	limitationMin        protocol.Position
	limitationMax        protocol.Position
	limitationTime       uint8
	limitationOriginator protocol.Originator
}

// newOpeningDevice initialises the shared OpeningDevice state, porting
// OpeningDevice.__init__. position/target come from the node information; the
// limitation fields start at their pyvlx defaults (Ignore / 255 / USER).
func newOpeningDevice(base BaseNode, positionParameter, targetParameter protocol.Parameter) OpeningDevice {
	posFrom := positionParameter
	targetFrom := targetParameter
	position, _ := protocol.NewPosition(&posFrom, nil, nil)
	target, _ := protocol.NewPosition(&targetFrom, nil, nil)
	return OpeningDevice{
		BaseNode:             base,
		position:             position,
		targetPosition:       target,
		limitationMin:        protocol.NewIgnorePosition(),
		limitationMax:        protocol.NewIgnorePosition(),
		limitationTime:       255,
		limitationOriginator: protocol.OriginatorUser,
	}
}

// Position returns the device's current position. Ported from the
// OpeningDevice.position attribute.
func (d *OpeningDevice) Position() protocol.Position { return d.position }

// TargetPosition returns the device's target position. Ported from the
// OpeningDevice.target_position attribute.
func (d *OpeningDevice) TargetPosition() protocol.Position { return d.targetPosition }

// SetPositionState stores a new current position without issuing a command. It
// is called by the node updater when a house-monitor / status notification
// reports a new position, mirroring pyvlx's direct assignment
// node.position = position. It does not fire AfterUpdate; the updater does that
// after all fields are set.
func (d *OpeningDevice) SetPositionState(pos protocol.Position) { d.position = pos }

// SetTargetState stores a new target position without issuing a command. Ported
// from the node updater's node.target_position = target assignment.
func (d *OpeningDevice) SetTargetState(target protocol.Position) { d.targetPosition = target }

// SetLimitationState stores new limitation fields without issuing a command.
// Ported from NodeUpdater.process_frame_limitation_status_notification.
func (d *OpeningDevice) SetLimitationState(min, max protocol.Position, originator protocol.Originator, limitTime uint8) {
	d.limitationMin = min
	d.limitationMax = max
	d.limitationOriginator = originator
	d.limitationTime = limitTime
}

// LimitationMin returns the current minimum position limitation. Ported from the
// OpeningDevice.limitation_min attribute.
func (d *OpeningDevice) LimitationMin() protocol.Position { return d.limitationMin }

// LimitationMax returns the current maximum position limitation. Ported from the
// OpeningDevice.limitation_max attribute.
func (d *OpeningDevice) LimitationMax() protocol.Position { return d.limitationMax }

// SetPosition sets the device to the desired position, optionally waiting for
// the movement to complete. Ported from OpeningDevice.set_position.
//
// self is the concrete node (Window, Awning, ...) so observers receive the real
// typed node from AfterUpdate.
func (d *OpeningDevice) setPosition(ctx context.Context, self Node, position protocol.Position, waitForCompletion bool) error {
	if err := d.Client().CommandSend(ctx, uint8(d.NodeID()), position.Parameter, nil, waitForCompletion); err != nil {
		return err
	}
	d.AfterUpdate(self)
	return nil
}

// open opens the device (position 0%). Ported from OpeningDevice.open.
func (d *OpeningDevice) open(ctx context.Context, self Node, waitForCompletion bool) error {
	pos, err := positionPercent(0)
	if err != nil {
		return err
	}
	return d.setPosition(ctx, self, pos, waitForCompletion)
}

// close closes the device (position 100%). Ported from OpeningDevice.close.
func (d *OpeningDevice) close(ctx context.Context, self Node, waitForCompletion bool) error {
	pos, err := positionPercent(100)
	if err != nil {
		return err
	}
	return d.setPosition(ctx, self, pos, waitForCompletion)
}

// stop stops the device (CURRENT position sentinel). Ported from
// OpeningDevice.stop.
func (d *OpeningDevice) stop(ctx context.Context, self Node, waitForCompletion bool) error {
	return d.setPosition(ctx, self, protocol.NewCurrentPosition(), waitForCompletion)
}

// setPositionLimitations sets a minimum and maximum position limit. Ported from
// OpeningDevice.set_position_limitations. The default limitation time is
// Unlimited (matching SetLimitation's default).
func (d *OpeningDevice) setPositionLimitations(ctx context.Context, self Node, positionMin, positionMax protocol.Position) error {
	if err := d.Client().SetLimitation(ctx, uint8(d.NodeID()), positionMin, positionMax, protocol.NewLimitationTimeUnlimited()); err != nil {
		return err
	}
	d.limitationMin = positionMin
	d.limitationMax = positionMax
	d.AfterUpdate(self)
	return nil
}

// clearPositionLimitations clears any position limits. Ported from
// OpeningDevice.clear_position_limitations: SetLimitation with the ClearAll
// limitation time and Ignore min/max, then reset the cached limits to Ignore.
func (d *OpeningDevice) clearPositionLimitations(ctx context.Context, self Node) error {
	if err := d.Client().SetLimitation(ctx, uint8(d.NodeID()), protocol.NewIgnorePosition(), protocol.NewIgnorePosition(), protocol.NewLimitationTimeClearAll()); err != nil {
		return err
	}
	d.limitationMin = protocol.NewIgnorePosition()
	d.limitationMax = protocol.NewIgnorePosition()
	d.AfterUpdate(self)
	return nil
}

// getLimitation returns the current (minimum) limitation. Ported from
// OpeningDevice.get_limitation.
func (d *OpeningDevice) getLimitation(ctx context.Context) (*LimitationResult, error) {
	return d.Client().GetLimitation(ctx, uint8(d.NodeID()), protocol.LimitationTypeMinLimitation)
}

// getLimitationMax returns the maximum limitation. Ported from
// OpeningDevice.get_limitation_max.
func (d *OpeningDevice) getLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return d.Client().GetLimitation(ctx, uint8(d.NodeID()), protocol.LimitationTypeMaxLimitation)
}

// positionPercent builds a Position from a percentage. Helper for the 0%/100%
// open/close calls (mirrors Position(position_percent=...)).
func positionPercent(percent int) (protocol.Position, error) {
	return protocol.NewPosition(nil, nil, &percent)
}

// ============================================================
// Window
// ============================================================

// Window is a window opener (pyvlx opening_device.Window). It adds a rain-sensor
// flag on top of OpeningDevice.
type Window struct {
	OpeningDevice
	rainSensor bool
}

// AfterUpdate delivers the concrete Window to observers. See BaseNode.AfterUpdate.
func (n *Window) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// RainSensor reports whether the window is equipped with a rain sensor.
func (n *Window) RainSensor() bool { return n.rainSensor }

// SetPosition ports OpeningDevice.set_position for a Window.
func (n *Window) SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error {
	return n.setPosition(ctx, n, position, waitForCompletion)
}

// Open ports OpeningDevice.open for a Window.
func (n *Window) Open(ctx context.Context, waitForCompletion bool) error {
	return n.open(ctx, n, waitForCompletion)
}

// Close ports OpeningDevice.close for a Window.
func (n *Window) Close(ctx context.Context, waitForCompletion bool) error {
	return n.close(ctx, n, waitForCompletion)
}

// Stop ports OpeningDevice.stop for a Window.
func (n *Window) Stop(ctx context.Context, waitForCompletion bool) error {
	return n.stop(ctx, n, waitForCompletion)
}

// SetPositionLimitations ports OpeningDevice.set_position_limitations.
func (n *Window) SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error {
	return n.setPositionLimitations(ctx, n, positionMin, positionMax)
}

// ClearPositionLimitations ports OpeningDevice.clear_position_limitations.
func (n *Window) ClearPositionLimitations(ctx context.Context) error {
	return n.clearPositionLimitations(ctx, n)
}

// GetLimitation ports OpeningDevice.get_limitation.
func (n *Window) GetLimitation(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitation(ctx)
}

// GetLimitationMax ports OpeningDevice.get_limitation_max.
func (n *Window) GetLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitationMax(ctx)
}

// String ports Window.__str__.
func (n *Window) String() string {
	return fmt.Sprintf("<Window name=%q node_id=%d rain_sensor=%t serial_number=%v position=%q/>",
		n.Name(), n.NodeID(), n.rainSensor, n.SerialNumber(), n.position.String())
}

var _ Node = (*Window)(nil)

func init() {
	RegisterNodeType(protocol.NodeTypeWithSubtypeWindowOpener, func(base BaseNode, info NodeInfo) Node {
		return &Window{
			OpeningDevice: newOpeningDevice(base, info.CurrentPosition, info.Target),
			rainSensor:    false,
		}
	})
	RegisterNodeType(protocol.NodeTypeWithSubtypeWindowOpenerWithRainSensor, func(base BaseNode, info NodeInfo) Node {
		return &Window{
			OpeningDevice: newOpeningDevice(base, info.CurrentPosition, info.Target),
			rainSensor:    true,
		}
	})
}

// ============================================================
// RollerShutter
// ============================================================

// RollerShutter is a roller shutter (pyvlx opening_device.RollerShutter). It has
// no extra behaviour over OpeningDevice.
type RollerShutter struct {
	OpeningDevice
}

// AfterUpdate delivers the concrete RollerShutter to observers.
func (n *RollerShutter) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// SetPosition ports OpeningDevice.set_position.
func (n *RollerShutter) SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error {
	return n.setPosition(ctx, n, position, waitForCompletion)
}

// Open ports OpeningDevice.open.
func (n *RollerShutter) Open(ctx context.Context, waitForCompletion bool) error {
	return n.open(ctx, n, waitForCompletion)
}

// Close ports OpeningDevice.close.
func (n *RollerShutter) Close(ctx context.Context, waitForCompletion bool) error {
	return n.close(ctx, n, waitForCompletion)
}

// Stop ports OpeningDevice.stop.
func (n *RollerShutter) Stop(ctx context.Context, waitForCompletion bool) error {
	return n.stop(ctx, n, waitForCompletion)
}

// SetPositionLimitations ports OpeningDevice.set_position_limitations.
func (n *RollerShutter) SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error {
	return n.setPositionLimitations(ctx, n, positionMin, positionMax)
}

// ClearPositionLimitations ports OpeningDevice.clear_position_limitations.
func (n *RollerShutter) ClearPositionLimitations(ctx context.Context) error {
	return n.clearPositionLimitations(ctx, n)
}

// GetLimitation ports OpeningDevice.get_limitation.
func (n *RollerShutter) GetLimitation(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitation(ctx)
}

// GetLimitationMax ports OpeningDevice.get_limitation_max.
func (n *RollerShutter) GetLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitationMax(ctx)
}

// String ports OpeningDevice.__str__.
func (n *RollerShutter) String() string {
	return openingDeviceString("RollerShutter", &n.OpeningDevice)
}

var _ Node = (*RollerShutter)(nil)

func init() {
	// ROLLER_SHUTTER, DUAL_ROLLER_SHUTTER, SWINGING_SHUTTERS all map to
	// RollerShutter with position+target from the node information.
	rollerCtor := func(base BaseNode, info NodeInfo) Node {
		return &RollerShutter{OpeningDevice: newOpeningDevice(base, info.CurrentPosition, info.Target)}
	}
	RegisterNodeType(protocol.NodeTypeWithSubtypeRollerShutter, rollerCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeDualRollerShutter, rollerCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeSwingingShutters, rollerCtor)

	// INTERIOR_VENETIAN_BLIND and VERTICAL_INTERIOR_BLINDS also map to
	// RollerShutter in pyvlx (without position parameters).
	interiorCtor := func(base BaseNode, info NodeInfo) Node {
		return &RollerShutter{OpeningDevice: newOpeningDevice(base, protocol.Parameter{}, protocol.Parameter{})}
	}
	RegisterNodeType(protocol.NodeTypeWithSubtypeInteriorVenetianBlind, interiorCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeVerticalInteriorBlinds, interiorCtor)
}

// ============================================================
// Awning
// ============================================================

// Awning is an awning (pyvlx opening_device.Awning). No extra behaviour.
type Awning struct {
	OpeningDevice
}

// AfterUpdate delivers the concrete Awning to observers.
func (n *Awning) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// SetPosition ports OpeningDevice.set_position.
func (n *Awning) SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error {
	return n.setPosition(ctx, n, position, waitForCompletion)
}

// Open ports OpeningDevice.open.
func (n *Awning) Open(ctx context.Context, waitForCompletion bool) error {
	return n.open(ctx, n, waitForCompletion)
}

// Close ports OpeningDevice.close.
func (n *Awning) Close(ctx context.Context, waitForCompletion bool) error {
	return n.close(ctx, n, waitForCompletion)
}

// Stop ports OpeningDevice.stop.
func (n *Awning) Stop(ctx context.Context, waitForCompletion bool) error {
	return n.stop(ctx, n, waitForCompletion)
}

// SetPositionLimitations ports OpeningDevice.set_position_limitations.
func (n *Awning) SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error {
	return n.setPositionLimitations(ctx, n, positionMin, positionMax)
}

// ClearPositionLimitations ports OpeningDevice.clear_position_limitations.
func (n *Awning) ClearPositionLimitations(ctx context.Context) error {
	return n.clearPositionLimitations(ctx, n)
}

// GetLimitation ports OpeningDevice.get_limitation.
func (n *Awning) GetLimitation(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitation(ctx)
}

// GetLimitationMax ports OpeningDevice.get_limitation_max.
func (n *Awning) GetLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitationMax(ctx)
}

// String ports OpeningDevice.__str__.
func (n *Awning) String() string { return openingDeviceString("Awning", &n.OpeningDevice) }

var _ Node = (*Awning)(nil)

func init() {
	awningCtor := func(base BaseNode, info NodeInfo) Node {
		return &Awning{OpeningDevice: newOpeningDevice(base, info.CurrentPosition, info.Target)}
	}
	RegisterNodeType(protocol.NodeTypeWithSubtypeVerticalExteriorAwning, awningCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeHorizontalAwning, awningCtor)
}

// ============================================================
// GarageDoor
// ============================================================

// GarageDoor is a garage door opener (pyvlx opening_device.GarageDoor).
type GarageDoor struct {
	OpeningDevice
}

// AfterUpdate delivers the concrete GarageDoor to observers.
func (n *GarageDoor) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// SetPosition ports OpeningDevice.set_position.
func (n *GarageDoor) SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error {
	return n.setPosition(ctx, n, position, waitForCompletion)
}

// Open ports OpeningDevice.open.
func (n *GarageDoor) Open(ctx context.Context, waitForCompletion bool) error {
	return n.open(ctx, n, waitForCompletion)
}

// Close ports OpeningDevice.close.
func (n *GarageDoor) Close(ctx context.Context, waitForCompletion bool) error {
	return n.close(ctx, n, waitForCompletion)
}

// Stop ports OpeningDevice.stop.
func (n *GarageDoor) Stop(ctx context.Context, waitForCompletion bool) error {
	return n.stop(ctx, n, waitForCompletion)
}

// SetPositionLimitations ports OpeningDevice.set_position_limitations.
func (n *GarageDoor) SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error {
	return n.setPositionLimitations(ctx, n, positionMin, positionMax)
}

// ClearPositionLimitations ports OpeningDevice.clear_position_limitations.
func (n *GarageDoor) ClearPositionLimitations(ctx context.Context) error {
	return n.clearPositionLimitations(ctx, n)
}

// GetLimitation ports OpeningDevice.get_limitation.
func (n *GarageDoor) GetLimitation(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitation(ctx)
}

// GetLimitationMax ports OpeningDevice.get_limitation_max.
func (n *GarageDoor) GetLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitationMax(ctx)
}

// String ports OpeningDevice.__str__.
func (n *GarageDoor) String() string { return openingDeviceString("GarageDoor", &n.OpeningDevice) }

var _ Node = (*GarageDoor)(nil)

func init() {
	garageCtor := func(base BaseNode, info NodeInfo) Node {
		return &GarageDoor{OpeningDevice: newOpeningDevice(base, info.CurrentPosition, info.Target)}
	}
	RegisterNodeType(protocol.NodeTypeWithSubtypeGarageDoorOpener, garageCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeLinarAngularPositionOfGarageDoor, garageCtor)
}

// ============================================================
// Gate
// ============================================================

// Gate is a gate opener (pyvlx opening_device.Gate).
type Gate struct {
	OpeningDevice
}

// AfterUpdate delivers the concrete Gate to observers.
func (n *Gate) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// SetPosition ports OpeningDevice.set_position.
func (n *Gate) SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error {
	return n.setPosition(ctx, n, position, waitForCompletion)
}

// Open ports OpeningDevice.open.
func (n *Gate) Open(ctx context.Context, waitForCompletion bool) error {
	return n.open(ctx, n, waitForCompletion)
}

// Close ports OpeningDevice.close.
func (n *Gate) Close(ctx context.Context, waitForCompletion bool) error {
	return n.close(ctx, n, waitForCompletion)
}

// Stop ports OpeningDevice.stop.
func (n *Gate) Stop(ctx context.Context, waitForCompletion bool) error {
	return n.stop(ctx, n, waitForCompletion)
}

// SetPositionLimitations ports OpeningDevice.set_position_limitations.
func (n *Gate) SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error {
	return n.setPositionLimitations(ctx, n, positionMin, positionMax)
}

// ClearPositionLimitations ports OpeningDevice.clear_position_limitations.
func (n *Gate) ClearPositionLimitations(ctx context.Context) error {
	return n.clearPositionLimitations(ctx, n)
}

// GetLimitation ports OpeningDevice.get_limitation.
func (n *Gate) GetLimitation(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitation(ctx)
}

// GetLimitationMax ports OpeningDevice.get_limitation_max.
func (n *Gate) GetLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitationMax(ctx)
}

// String ports OpeningDevice.__str__.
func (n *Gate) String() string { return openingDeviceString("Gate", &n.OpeningDevice) }

var _ Node = (*Gate)(nil)

func init() {
	gateCtor := func(base BaseNode, info NodeInfo) Node {
		return &Gate{OpeningDevice: newOpeningDevice(base, info.CurrentPosition, info.Target)}
	}
	RegisterNodeType(protocol.NodeTypeWithSubtypeGateOpener, gateCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeGateOpenerAngularPosition, gateCtor)
}

// ============================================================
// Blade
// ============================================================

// Blade is a blade opener (pyvlx opening_device.Blade).
type Blade struct {
	OpeningDevice
}

// AfterUpdate delivers the concrete Blade to observers.
func (n *Blade) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// SetPosition ports OpeningDevice.set_position.
func (n *Blade) SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error {
	return n.setPosition(ctx, n, position, waitForCompletion)
}

// Open ports OpeningDevice.open.
func (n *Blade) Open(ctx context.Context, waitForCompletion bool) error {
	return n.open(ctx, n, waitForCompletion)
}

// Close ports OpeningDevice.close.
func (n *Blade) Close(ctx context.Context, waitForCompletion bool) error {
	return n.close(ctx, n, waitForCompletion)
}

// Stop ports OpeningDevice.stop.
func (n *Blade) Stop(ctx context.Context, waitForCompletion bool) error {
	return n.stop(ctx, n, waitForCompletion)
}

// SetPositionLimitations ports OpeningDevice.set_position_limitations.
func (n *Blade) SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error {
	return n.setPositionLimitations(ctx, n, positionMin, positionMax)
}

// ClearPositionLimitations ports OpeningDevice.clear_position_limitations.
func (n *Blade) ClearPositionLimitations(ctx context.Context) error {
	return n.clearPositionLimitations(ctx, n)
}

// GetLimitation ports OpeningDevice.get_limitation.
func (n *Blade) GetLimitation(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitation(ctx)
}

// GetLimitationMax ports OpeningDevice.get_limitation_max.
func (n *Blade) GetLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitationMax(ctx)
}

// String ports OpeningDevice.__str__.
func (n *Blade) String() string { return openingDeviceString("Blade", &n.OpeningDevice) }

var _ Node = (*Blade)(nil)

func init() {
	RegisterNodeType(protocol.NodeTypeWithSubtypeBladeOpener, func(base BaseNode, info NodeInfo) Node {
		return &Blade{OpeningDevice: newOpeningDevice(base, info.CurrentPosition, info.Target)}
	})
}

// ============================================================
// Blind
// ============================================================

// Blind is a blind with both position and slat orientation (pyvlx
// opening_device.Blind). Orientation is driven through functional parameter 3
// (fp3) of the command frame. Ported in full including the position/orientation
// interplay.
type Blind struct {
	OpeningDevice

	orientation       protocol.Position
	targetOrientation protocol.Position

	openOrientationTarget  int
	closeOrientationTarget int
}

// AfterUpdate delivers the concrete Blind to observers.
func (n *Blind) AfterUpdate() { n.BaseNode.AfterUpdate(n) }

// SetOrientationState stores a new slat orientation without issuing a command.
// It is called by the node updater from a status-request notification (FP3),
// mirroring pyvlx's node.orientation = orientation assignment. It does not fire
// AfterUpdate; the updater does that after all fields are set.
func (n *Blind) SetOrientationState(orient protocol.Position) { n.orientation = orient }

// Orientation returns the blind's current slat orientation.
func (n *Blind) Orientation() protocol.Position { return n.orientation }

// TargetOrientation returns the blind's target slat orientation.
func (n *Blind) TargetOrientation() protocol.Position { return n.targetOrientation }

// SetPositionAndOrientation sets the blind position and, optionally, the slat
// orientation in the same request. Ported from
// Blind.set_position_and_orientation.
//
// orientation may be nil (meaning "not specified"): in that case, if the target
// position is 0% the orientation fp3 is forced to 0%, otherwise fp3 is Ignore.
func (n *Blind) SetPositionAndOrientation(ctx context.Context, position protocol.Position, waitForCompletion bool, orientation *protocol.Position) error {
	n.targetPosition = position
	n.position = position

	zero, err := positionPercent(0)
	if err != nil {
		return err
	}

	var fp3 protocol.Position
	switch {
	case orientation != nil:
		fp3 = *orientation
	case n.targetPosition.Raw == zero.Raw:
		fp3 = zero
	default:
		fp3 = protocol.NewIgnorePosition()
	}

	fps := map[int]protocol.Parameter{3: fp3.Parameter}
	if err := n.Client().CommandSend(ctx, uint8(n.NodeID()), position.Parameter, fps, waitForCompletion); err != nil {
		return err
	}
	n.AfterUpdate()
	return nil
}

// SetPosition sets the blind to the desired position (delegating to
// SetPositionAndOrientation with no explicit orientation). Ported from
// Blind.set_position.
func (n *Blind) SetPosition(ctx context.Context, position protocol.Position, waitForCompletion bool) error {
	return n.SetPositionAndOrientation(ctx, position, waitForCompletion, nil)
}

// Open opens the blind (position 0%). Ported from Blind.open.
func (n *Blind) Open(ctx context.Context, waitForCompletion bool) error {
	pos, err := positionPercent(0)
	if err != nil {
		return err
	}
	return n.SetPosition(ctx, pos, waitForCompletion)
}

// Close closes the blind (position 100%). Ported from Blind.close.
func (n *Blind) Close(ctx context.Context, waitForCompletion bool) error {
	pos, err := positionPercent(100)
	if err != nil {
		return err
	}
	return n.SetPosition(ctx, pos, waitForCompletion)
}

// Stop stops the blind, keeping the current position but re-asserting the target
// orientation via fp3. Ported from Blind.stop.
func (n *Blind) Stop(ctx context.Context, waitForCompletion bool) error {
	orientation := n.targetOrientation
	return n.SetPositionAndOrientation(ctx, protocol.NewCurrentPosition(), waitForCompletion, &orientation)
}

// SetOrientation sets the blind slat orientation without moving the blind
// position (main parameter is Ignore, fp3 carries the orientation). Ported from
// Blind.set_orientation, including the KLF200 quirk that the orientation is set
// directly here rather than via GW_NODE_STATE_POSITION_CHANGED_NTF.
func (n *Blind) SetOrientation(ctx context.Context, orientation protocol.Position, waitForCompletion bool) error {
	n.targetOrientation = orientation
	n.orientation = orientation

	zero, err := positionPercent(0)
	if err != nil {
		return err
	}
	fp3 := n.targetOrientation
	if n.targetPosition.Raw == zero.Raw {
		fp3 = zero
	}

	fps := map[int]protocol.Parameter{3: fp3.Parameter}
	if err := n.Client().CommandSend(ctx, uint8(n.NodeID()), protocol.NewIgnorePosition().Parameter, fps, waitForCompletion); err != nil {
		return err
	}
	n.AfterUpdate()
	return nil
}

// OpenOrientation opens the blind slats (orientation open target, 50% by
// default). Ported from Blind.open_orientation.
func (n *Blind) OpenOrientation(ctx context.Context, waitForCompletion bool) error {
	pos, err := positionPercent(n.openOrientationTarget)
	if err != nil {
		return err
	}
	return n.SetOrientation(ctx, pos, waitForCompletion)
}

// CloseOrientation closes the blind slats (orientation close target, 100% by
// default). Ported from Blind.close_orientation.
func (n *Blind) CloseOrientation(ctx context.Context, waitForCompletion bool) error {
	pos, err := positionPercent(n.closeOrientationTarget)
	if err != nil {
		return err
	}
	return n.SetOrientation(ctx, pos, waitForCompletion)
}

// StopOrientation stops the blind slats (CURRENT orientation sentinel). Ported
// from Blind.stop_orientation.
func (n *Blind) StopOrientation(ctx context.Context, waitForCompletion bool) error {
	return n.SetOrientation(ctx, protocol.NewCurrentPosition(), waitForCompletion)
}

// SetPositionLimitations ports OpeningDevice.set_position_limitations.
func (n *Blind) SetPositionLimitations(ctx context.Context, positionMin, positionMax protocol.Position) error {
	return n.setPositionLimitations(ctx, n, positionMin, positionMax)
}

// ClearPositionLimitations ports OpeningDevice.clear_position_limitations.
func (n *Blind) ClearPositionLimitations(ctx context.Context) error {
	return n.clearPositionLimitations(ctx, n)
}

// GetLimitation ports OpeningDevice.get_limitation.
func (n *Blind) GetLimitation(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitation(ctx)
}

// GetLimitationMax ports OpeningDevice.get_limitation_max.
func (n *Blind) GetLimitationMax(ctx context.Context) (*LimitationResult, error) {
	return n.getLimitationMax(ctx)
}

// String ports OpeningDevice.__str__.
func (n *Blind) String() string { return openingDeviceString("Blind", &n.OpeningDevice) }

var _ Node = (*Blind)(nil)

func init() {
	// Blinds have position and orientation (fp3). Ported from node_helper: the
	// EXTERIOR_VENETIAN_BLIND, ADJUSTABLE_SLUTS_ROLLING_SHUTTER and LOUVER_BLIND
	// subtypes map to Blind.
	blindCtor := func(base BaseNode, info NodeInfo) Node {
		b := &Blind{
			OpeningDevice:          newOpeningDevice(base, info.CurrentPosition, info.Target),
			orientation:            mustPositionPercent(0),
			targetOrientation:      protocol.NewTargetPosition(),
			openOrientationTarget:  50,
			closeOrientationTarget: 100,
		}
		// Blind.__init__ overrides target_position to the TARGET sentinel.
		b.targetPosition = protocol.NewTargetPosition()
		return b
	}
	RegisterNodeType(protocol.NodeTypeWithSubtypeExteriorVenetianBlind, blindCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeAdjustableSlutsRollingShutter, blindCtor)
	RegisterNodeType(protocol.NodeTypeWithSubtypeLouverBlind, blindCtor)
}

// mustPositionPercent builds a Position from a percentage, panicking on the
// (impossible for 0..100) error. Used only for compile-time-known constants.
func mustPositionPercent(percent int) protocol.Position {
	p, err := protocol.NewPosition(nil, nil, &percent)
	if err != nil {
		panic(err)
	}
	return p
}

// openingDeviceString renders the shared OpeningDevice.__str__ representation
// with the concrete type name.
func openingDeviceString(typeName string, d *OpeningDevice) string {
	return fmt.Sprintf("<%s name=%q node_id=%d serial_number=%v position=%q/>",
		typeName, d.Name(), d.NodeID(), d.SerialNumber(), d.position.String())
}
