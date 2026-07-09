// Package protocol - frames for gateway state, network setup, error notification,
// and activation log updated. Ported from pyvlx frame_get_state.py,
// frame_get_network_setup.py, frame_error_notification.py,
// frame_activation_log_updated.py.
package protocol

import (
	"fmt"
	"net"
)

// ---------------------------------------------------------------------------
// GW_GET_STATE_REQ (0x000C) — request gateway state
// ---------------------------------------------------------------------------

// FrameGetStateRequest implements Frame for GW_GET_STATE_REQ.
// Payload is empty (PAYLOAD_LEN = 0).
type FrameGetStateRequest struct{}

// Command returns the constant command code for this frame.
func (f *FrameGetStateRequest) Command() Command { return GW_GET_STATE_REQ }

// MarshalPayload returns an empty payload.
func (f *FrameGetStateRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

// UnmarshalPayload validates that the payload is empty.
func (f *FrameGetStateRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetStateRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetStateRequest)(nil)

func init() {
	RegisterFrame(GW_GET_STATE_REQ, func() Frame { return &FrameGetStateRequest{} })
}

// ---------------------------------------------------------------------------
// GW_GET_STATE_CFM (0x000D) — confirmation of gateway state request
// ---------------------------------------------------------------------------

// FrameGetStateConfirmation implements Frame for GW_GET_STATE_CFM.
// Payload: gateway_state (1 byte), gateway_sub_state (1 byte), 4 reserved bytes.
// PAYLOAD_LEN = 6.
type FrameGetStateConfirmation struct {
	GatewayState    GatewayState
	GatewaySubState GatewaySubState
}

// Command returns the constant command code for this frame.
func (f *FrameGetStateConfirmation) Command() Command { return GW_GET_STATE_CFM }

// MarshalPayload returns the 6-byte payload.
// Bytes 0: GatewayState, byte 1: GatewaySubState, bytes 2-5: reserved zeros.
func (f *FrameGetStateConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 6)
	payload[0] = uint8(f.GatewayState)
	payload[1] = uint8(f.GatewaySubState)
	// bytes 2-5: state date, reserved for future use (zeros)
	return payload, nil
}

// UnmarshalPayload populates the frame from a 6-byte payload.
func (f *FrameGetStateConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 6 {
		return fmt.Errorf("FrameGetStateConfirmation: invalid payload len %d, want 6", len(payload))
	}
	f.GatewayState = GatewayState(payload[0])
	f.GatewaySubState = GatewaySubState(payload[1])
	return nil
}

var _ Frame = (*FrameGetStateConfirmation)(nil)

func init() {
	RegisterFrame(GW_GET_STATE_CFM, func() Frame { return &FrameGetStateConfirmation{} })
}

// ---------------------------------------------------------------------------
// GW_GET_NETWORK_SETUP_REQ (0x00E0) — request network setup
// ---------------------------------------------------------------------------

// FrameGetNetworkSetupRequest implements Frame for GW_GET_NETWORK_SETUP_REQ.
// Payload is empty (PAYLOAD_LEN = 0).
type FrameGetNetworkSetupRequest struct{}

// Command returns the constant command code for this frame.
func (f *FrameGetNetworkSetupRequest) Command() Command { return GW_GET_NETWORK_SETUP_REQ }

// MarshalPayload returns an empty payload.
func (f *FrameGetNetworkSetupRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

// UnmarshalPayload validates that the payload is empty.
func (f *FrameGetNetworkSetupRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetNetworkSetupRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetNetworkSetupRequest)(nil)

func init() {
	RegisterFrame(GW_GET_NETWORK_SETUP_REQ, func() Frame { return &FrameGetNetworkSetupRequest{} })
}

// ---------------------------------------------------------------------------
// GW_GET_NETWORK_SETUP_CFM (0x00E1) — confirmation of network setup request
// ---------------------------------------------------------------------------

// FrameGetNetworkSetupConfirmation implements Frame for GW_GET_NETWORK_SETUP_CFM.
// Payload: ipaddress[4], netmask[4], gateway[4], dhcp[1]. PAYLOAD_LEN = 13.
// IPAddress, Netmask, Gateway are stored as 4-byte arrays (big-endian / network order).
type FrameGetNetworkSetupConfirmation struct {
	IPAddress [4]byte
	Netmask   [4]byte
	Gateway   [4]byte
	DHCP      DHCPParameter
}

// Command returns the constant command code for this frame.
func (f *FrameGetNetworkSetupConfirmation) Command() Command { return GW_GET_NETWORK_SETUP_CFM }

// IPAddressString returns the IP address as a dotted-decimal string.
func (f *FrameGetNetworkSetupConfirmation) IPAddressString() string {
	return net.IP(f.IPAddress[:]).String()
}

// NetmaskString returns the netmask as a dotted-decimal string.
func (f *FrameGetNetworkSetupConfirmation) NetmaskString() string {
	return net.IP(f.Netmask[:]).String()
}

// GatewayString returns the gateway as a dotted-decimal string.
func (f *FrameGetNetworkSetupConfirmation) GatewayString() string {
	return net.IP(f.Gateway[:]).String()
}

// MarshalPayload returns the 13-byte payload.
// Layout: ipaddress[0:4], netmask[4:8], gateway[8:12], dhcp[12].
func (f *FrameGetNetworkSetupConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 13)
	copy(payload[0:4], f.IPAddress[:])
	copy(payload[4:8], f.Netmask[:])
	copy(payload[8:12], f.Gateway[:])
	payload[12] = uint8(f.DHCP)
	return payload, nil
}

// UnmarshalPayload populates the frame from a 13-byte payload.
func (f *FrameGetNetworkSetupConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 13 {
		return fmt.Errorf("FrameGetNetworkSetupConfirmation: invalid payload len %d, want 13", len(payload))
	}
	copy(f.IPAddress[:], payload[0:4])
	copy(f.Netmask[:], payload[4:8])
	copy(f.Gateway[:], payload[8:12])
	f.DHCP = DHCPParameter(payload[12])
	return nil
}

var _ Frame = (*FrameGetNetworkSetupConfirmation)(nil)

func init() {
	RegisterFrame(GW_GET_NETWORK_SETUP_CFM, func() Frame { return &FrameGetNetworkSetupConfirmation{} })
}

// ---------------------------------------------------------------------------
// GW_ERROR_NTF (0x0000) — error notification
// ---------------------------------------------------------------------------

// ErrorType is the error type enum from FrameErrorNotification.
// Values match pyvlx ErrorType and const.ErrorNumber.
type ErrorType = ErrorNumber

const (
	ErrorTypeNotFurtherDefined ErrorType = ErrorNumberUndefined
	ErrorTypeUnknownCommand    ErrorType = ErrorNumberWrongCommand
	ErrorTypeErrorOnFrame      ErrorType = ErrorNumberFrameError
	ErrorTypeBusBusy           ErrorType = ErrorNumberBusy
	ErrorTypeBadSystemTable    ErrorType = ErrorNumberBadSystableIndex
	ErrorTypeNotAuthenticated  ErrorType = ErrorNumberNoAuth
)

// FrameErrorNotification implements Frame for GW_ERROR_NTF.
// Payload: error_type (1 byte). PAYLOAD_LEN = 1.
type FrameErrorNotification struct {
	ErrorType ErrorType
}

// Command returns the constant command code for this frame.
func (f *FrameErrorNotification) Command() Command { return GW_ERROR_NTF }

// MarshalPayload returns the 1-byte payload.
func (f *FrameErrorNotification) MarshalPayload() ([]byte, error) {
	return []byte{uint8(f.ErrorType)}, nil
}

// UnmarshalPayload populates the frame from a 1-byte payload.
func (f *FrameErrorNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 1 {
		return fmt.Errorf("FrameErrorNotification: invalid payload len %d, want 1", len(payload))
	}
	f.ErrorType = ErrorType(payload[0])
	return nil
}

var _ Frame = (*FrameErrorNotification)(nil)

func init() {
	RegisterFrame(GW_ERROR_NTF, func() Frame { return &FrameErrorNotification{} })
}

// ---------------------------------------------------------------------------
// GW_ACTIVATION_LOG_UPDATED_NTF (0x0506) — activation log updated notification
// ---------------------------------------------------------------------------

// FrameActivationLogUpdatedNotification implements Frame for GW_ACTIVATION_LOG_UPDATED_NTF.
// Payload is empty (PAYLOAD_LEN = 0).
type FrameActivationLogUpdatedNotification struct{}

// Command returns the constant command code for this frame.
func (f *FrameActivationLogUpdatedNotification) Command() Command {
	return GW_ACTIVATION_LOG_UPDATED_NTF
}

// MarshalPayload returns an empty payload.
func (f *FrameActivationLogUpdatedNotification) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

// UnmarshalPayload validates that the payload is empty.
func (f *FrameActivationLogUpdatedNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameActivationLogUpdatedNotification: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameActivationLogUpdatedNotification)(nil)

func init() {
	RegisterFrame(GW_ACTIVATION_LOG_UPDATED_NTF, func() Frame { return &FrameActivationLogUpdatedNotification{} })
}
