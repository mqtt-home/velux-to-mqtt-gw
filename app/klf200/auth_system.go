package klf200

// auth_system.go — api-call wrappers for auth/system operations.
// Ported from pyvlx/api: password_enter.py, get_version.py,
// get_protocol_version.py, get_network_setup.py, get_local_time.py,
// set_utc.py, leave_learn_state.py, reboot.py, factory_default.py,
// set_node_name.py.

import (
	"context"
	"fmt"
	"time"

	"github.com/mqtt-home/velux-to-mqtt-gw/klf200/protocol"
)

// PasswordEnter authenticates the client with the gateway. It issues
// GW_PASSWORD_ENTER_REQ and waits for GW_PASSWORD_ENTER_CFM. Returns an error
// if the gateway rejects the password (status != 0). Ported from
// pyvlx/api/password_enter.py PasswordEnter.
func (c *Client) PasswordEnter(ctx context.Context, password string) error {
	req := &protocol.FramePasswordEnterRequest{Password: password}
	var cfm *protocol.FramePasswordEnterConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FramePasswordEnterConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return fmt.Errorf("klf200: password enter: %w", err)
	}
	if cfm.Status != 0 {
		return fmt.Errorf("klf200: password enter: authentication failed (status %d)", cfm.Status)
	}
	return nil
}

// GatewayVersion holds the version information returned by GW_GET_VERSION_CFM.
// Ported from pyvlx DtoVersion.
type GatewayVersion struct {
	SoftwareVersion [6]byte
	HardwareVersion uint8
	ProductGroup    uint8
	ProductType     uint8
}

// SoftwareVersionString returns the software version formatted as "a.b.c.d.e.f".
func (v GatewayVersion) SoftwareVersionString() string {
	return fmt.Sprintf("%d.%d.%d.%d.%d.%d",
		v.SoftwareVersion[0], v.SoftwareVersion[1], v.SoftwareVersion[2],
		v.SoftwareVersion[3], v.SoftwareVersion[4], v.SoftwareVersion[5])
}

// GetVersion retrieves the firmware version from the gateway via
// GW_GET_VERSION_REQ / GW_GET_VERSION_CFM. Ported from
// pyvlx/api/get_version.py GetVersion.
func (c *Client) GetVersion(ctx context.Context) (GatewayVersion, error) {
	req := &protocol.FrameGetVersionRequest{}
	var cfm *protocol.FrameGetVersionConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FrameGetVersionConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return GatewayVersion{}, fmt.Errorf("klf200: get version: %w", err)
	}
	return GatewayVersion{
		SoftwareVersion: cfm.SoftwareVersion,
		HardwareVersion: cfm.HardwareVersion,
		ProductGroup:    cfm.ProductGroup,
		ProductType:     cfm.ProductType,
	}, nil
}

// ProtocolVersion holds the protocol version returned by
// GW_GET_PROTOCOL_VERSION_CFM. Ported from pyvlx DtoProtocolVersion.
type ProtocolVersion struct {
	MajorVersion uint16
	MinorVersion uint16
}

// String returns the protocol version as "major.minor".
func (v ProtocolVersion) String() string {
	return fmt.Sprintf("%d.%d", v.MajorVersion, v.MinorVersion)
}

// GetProtocolVersion retrieves the KLF200 protocol version via
// GW_GET_PROTOCOL_VERSION_REQ / GW_GET_PROTOCOL_VERSION_CFM. Ported from
// pyvlx/api/get_protocol_version.py GetProtocolVersion.
func (c *Client) GetProtocolVersion(ctx context.Context) (ProtocolVersion, error) {
	req := &protocol.FrameGetProtocolVersionRequest{}
	var cfm *protocol.FrameGetProtocolVersionConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FrameGetProtocolVersionConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return ProtocolVersion{}, fmt.Errorf("klf200: get protocol version: %w", err)
	}
	return ProtocolVersion{
		MajorVersion: cfm.MajorVersion,
		MinorVersion: cfm.MinorVersion,
	}, nil
}

// NetworkSetup holds the network configuration returned by
// GW_GET_NETWORK_SETUP_CFM. Ported from pyvlx DtoNetworkSetup.
type NetworkSetup struct {
	IPAddress [4]byte
	Netmask   [4]byte
	Gateway   [4]byte
	DHCP      protocol.DHCPParameter
}

// IPAddressString returns the IP address as a dotted-decimal string.
func (ns NetworkSetup) IPAddressString() string {
	return fmt.Sprintf("%d.%d.%d.%d", ns.IPAddress[0], ns.IPAddress[1], ns.IPAddress[2], ns.IPAddress[3])
}

// NetmaskString returns the netmask as a dotted-decimal string.
func (ns NetworkSetup) NetmaskString() string {
	return fmt.Sprintf("%d.%d.%d.%d", ns.Netmask[0], ns.Netmask[1], ns.Netmask[2], ns.Netmask[3])
}

// GatewayString returns the gateway as a dotted-decimal string.
func (ns NetworkSetup) GatewayString() string {
	return fmt.Sprintf("%d.%d.%d.%d", ns.Gateway[0], ns.Gateway[1], ns.Gateway[2], ns.Gateway[3])
}

// GetNetworkSetup retrieves the gateway network configuration via
// GW_GET_NETWORK_SETUP_REQ / GW_GET_NETWORK_SETUP_CFM. Ported from
// pyvlx/api/get_network_setup.py GetNetworkSetup.
func (c *Client) GetNetworkSetup(ctx context.Context) (NetworkSetup, error) {
	req := &protocol.FrameGetNetworkSetupRequest{}
	var cfm *protocol.FrameGetNetworkSetupConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FrameGetNetworkSetupConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return NetworkSetup{}, fmt.Errorf("klf200: get network setup: %w", err)
	}
	return NetworkSetup{
		IPAddress: cfm.IPAddress,
		Netmask:   cfm.Netmask,
		Gateway:   cfm.Gateway,
		DHCP:      cfm.DHCP,
	}, nil
}

// GetLocalTime retrieves the gateway local time via
// GW_GET_LOCAL_TIME_REQ / GW_GET_LOCAL_TIME_CFM. Returns the decoded
// LocalTime value from the protocol frame. Ported from
// pyvlx/api/get_local_time.py GetLocalTime.
func (c *Client) GetLocalTime(ctx context.Context) (protocol.LocalTime, error) {
	req := &protocol.FrameGetLocalTimeRequest{}
	var cfm *protocol.FrameGetLocalTimeConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FrameGetLocalTimeConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return protocol.LocalTime{}, fmt.Errorf("klf200: get local time: %w", err)
	}
	return cfm.Time, nil
}

// SetUTC sets the UTC clock on the gateway via GW_SET_UTC_REQ / GW_SET_UTC_CFM.
// If t is the zero value, time.Now() is used (matching pyvlx's default of
// time.time()). Ported from pyvlx/api/set_utc.py SetUTC.
func (c *Client) SetUTC(ctx context.Context, t time.Time) error {
	if t.IsZero() {
		t = time.Now()
	}
	req := &protocol.FrameSetUTCRequest{Timestamp: uint32(t.Unix())}
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		_, ok := frame.(*protocol.FrameSetUTCConfirmation)
		return ok
	})
	if err != nil {
		return fmt.Errorf("klf200: set utc: %w", err)
	}
	return nil
}

// GatewayState holds the gateway state returned by GW_GET_STATE_CFM.
type GatewayState struct {
	State    protocol.GatewayState
	SubState protocol.GatewaySubState
}

// GetState retrieves the current gateway state via
// GW_GET_STATE_REQ / GW_GET_STATE_CFM. Ported from
// pyvlx/api/get_state.py GetState.
func (c *Client) GetState(ctx context.Context) (GatewayState, error) {
	req := &protocol.FrameGetStateRequest{}
	var cfm *protocol.FrameGetStateConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FrameGetStateConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return GatewayState{}, fmt.Errorf("klf200: get state: %w", err)
	}
	return GatewayState{
		State:    cfm.GatewayState,
		SubState: cfm.GatewaySubState,
	}, nil
}

// LeaveLearnState commands the gateway to leave the learn state via
// GW_LEAVE_LEARN_STATE_REQ / GW_LEAVE_LEARN_STATE_CFM. Returns an error if
// the gateway reports failure. Ported from
// pyvlx/api/leave_learn_state.py LeaveLearnState.
func (c *Client) LeaveLearnState(ctx context.Context) error {
	req := &protocol.FrameLeaveLearnStateRequest{}
	var cfm *protocol.FrameLeaveLearnStateConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FrameLeaveLearnStateConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return fmt.Errorf("klf200: leave learn state: %w", err)
	}
	if cfm.Status != protocol.LeaveLearnStateConfirmationStatusSuccessful {
		return fmt.Errorf("klf200: leave learn state: failed (status %d)", cfm.Status)
	}
	return nil
}

// Reboot sends GW_REBOOT_REQ and waits for GW_REBOOT_CFM. After the
// confirmation the gateway reboots and the connection will be lost. Ported
// from pyvlx/api/reboot.py Reboot.
func (c *Client) Reboot(ctx context.Context) error {
	req := &protocol.FrameGatewayRebootRequest{}
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		_, ok := frame.(*protocol.FrameGatewayRebootConfirmation)
		return ok
	})
	if err != nil {
		return fmt.Errorf("klf200: reboot: %w", err)
	}
	return nil
}

// FactoryDefault sends GW_SET_FACTORY_DEFAULT_REQ and waits for
// GW_SET_FACTORY_DEFAULT_CFM. After confirmation the gateway resets and the
// connection will be lost. Ported from pyvlx/api/factory_default.py
// FactoryDefault.
func (c *Client) FactoryDefault(ctx context.Context) error {
	req := &protocol.FrameGatewayFactoryDefaultRequest{}
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		_, ok := frame.(*protocol.FrameGatewayFactoryDefaultConfirmation)
		return ok
	})
	if err != nil {
		return fmt.Errorf("klf200: factory default: %w", err)
	}
	return nil
}

// SetNodeName renames a node on the gateway via
// GW_SET_NODE_NAME_REQ / GW_SET_NODE_NAME_CFM. Returns an error if the
// gateway rejects the rename. Ported from
// pyvlx/api/set_node_name.py SetNodeName.
func (c *Client) SetNodeName(ctx context.Context, nodeID uint8, name string) error {
	req := &protocol.FrameSetNodeNameRequest{NodeID: nodeID, Name: name}
	var cfm *protocol.FrameSetNodeNameConfirmation
	err := c.APICall(ctx, req, func(frame protocol.Frame) bool {
		f, ok := frame.(*protocol.FrameSetNodeNameConfirmation)
		if !ok {
			return false
		}
		cfm = f
		return true
	})
	if err != nil {
		return fmt.Errorf("klf200: set node name: %w", err)
	}
	if cfm.Status != protocol.SetNodeNameConfirmationStatusOK {
		return fmt.Errorf("klf200: set node name: rejected (status %d)", cfm.Status)
	}
	return nil
}
