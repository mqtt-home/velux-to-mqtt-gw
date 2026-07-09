// Package protocol — auth/system frames:
// password_enter, password_change, reboot, factory_default,
// leave_learn_state, get_version, get_protocol_version, set_utc, get_local_time.
package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers shared within this file
// ---------------------------------------------------------------------------

// stringToBytes encodes s as UTF-8 and right-pads with NUL to size bytes.
// Mirrors pyvlx string_helper.string_to_bytes.
func stringToBytes(s string, size int) ([]byte, error) {
	encoded := []byte(s)
	if len(encoded) > size {
		return nil, fmt.Errorf("stringToBytes: string too long (%d > %d)", len(encoded), size)
	}
	out := make([]byte, size)
	copy(out, encoded)
	return out, nil
}

// bytesToString decodes a NUL-terminated (or NUL-padded) byte slice to a
// string.  Mirrors pyvlx string_helper.bytes_to_string.
func bytesToString(raw []byte) string {
	for i, b := range raw {
		if b == 0x00 {
			return string(raw[:i])
		}
	}
	return string(raw)
}

// ---------------------------------------------------------------------------
// password_enter
// ---------------------------------------------------------------------------

// FramePasswordEnterRequest implements Frame for GW_PASSWORD_ENTER_REQ.
type FramePasswordEnterRequest struct {
	// Password is the gateway password (max 32 characters).
	Password string
}

func (f *FramePasswordEnterRequest) Command() Command { return GW_PASSWORD_ENTER_REQ }

func (f *FramePasswordEnterRequest) MarshalPayload() ([]byte, error) {
	if f.Password == "" {
		return nil, fmt.Errorf("FramePasswordEnterRequest: password is empty")
	}
	return stringToBytes(f.Password, 32)
}

func (f *FramePasswordEnterRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 32 {
		return fmt.Errorf("FramePasswordEnterRequest: invalid payload len %d, want 32", len(payload))
	}
	f.Password = bytesToString(payload)
	return nil
}

var _ Frame = (*FramePasswordEnterRequest)(nil)

// FramePasswordEnterConfirmation implements Frame for GW_PASSWORD_ENTER_CFM.
type FramePasswordEnterConfirmation struct {
	// Status: 0 = Successful, 1 = Failed.
	Status uint8
}

func (f *FramePasswordEnterConfirmation) Command() Command { return GW_PASSWORD_ENTER_CFM }

func (f *FramePasswordEnterConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{f.Status}, nil
}

func (f *FramePasswordEnterConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 1 {
		return fmt.Errorf("FramePasswordEnterConfirmation: invalid payload len %d, want 1", len(payload))
	}
	f.Status = payload[0]
	return nil
}

var _ Frame = (*FramePasswordEnterConfirmation)(nil)

// ---------------------------------------------------------------------------
// password_change
// ---------------------------------------------------------------------------

// FramePasswordChangeRequest implements Frame for GW_PASSWORD_CHANGE_REQ.
type FramePasswordChangeRequest struct {
	// Currentpassword is the current gateway password (max 32 characters).
	Currentpassword string
	// Newpassword is the desired new password (max 32 characters).
	Newpassword string
}

func (f *FramePasswordChangeRequest) Command() Command { return GW_PASSWORD_CHANGE_REQ }

func (f *FramePasswordChangeRequest) MarshalPayload() ([]byte, error) {
	if f.Currentpassword == "" {
		return nil, fmt.Errorf("FramePasswordChangeRequest: currentpassword is empty")
	}
	if f.Newpassword == "" {
		return nil, fmt.Errorf("FramePasswordChangeRequest: newpassword is empty")
	}
	cur, err := stringToBytes(f.Currentpassword, 32)
	if err != nil {
		return nil, fmt.Errorf("FramePasswordChangeRequest: currentpassword: %w", err)
	}
	nw, err := stringToBytes(f.Newpassword, 32)
	if err != nil {
		return nil, fmt.Errorf("FramePasswordChangeRequest: newpassword: %w", err)
	}
	return append(cur, nw...), nil
}

func (f *FramePasswordChangeRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 64 {
		return fmt.Errorf("FramePasswordChangeRequest: invalid payload len %d, want 64", len(payload))
	}
	f.Currentpassword = bytesToString(payload[0:32])
	f.Newpassword = bytesToString(payload[32:])
	return nil
}

var _ Frame = (*FramePasswordChangeRequest)(nil)

// FramePasswordChangeConfirmation implements Frame for GW_PASSWORD_CHANGE_CFM.
type FramePasswordChangeConfirmation struct {
	// Status: 0 = Successful, 1 = Failed.
	Status uint8
}

func (f *FramePasswordChangeConfirmation) Command() Command { return GW_PASSWORD_CHANGE_CFM }

func (f *FramePasswordChangeConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{f.Status}, nil
}

func (f *FramePasswordChangeConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 1 {
		return fmt.Errorf("FramePasswordChangeConfirmation: invalid payload len %d, want 1", len(payload))
	}
	f.Status = payload[0]
	return nil
}

var _ Frame = (*FramePasswordChangeConfirmation)(nil)

// FramePasswordChangeNotification implements Frame for GW_PASSWORD_CHANGE_NTF.
type FramePasswordChangeNotification struct {
	// Newpassword is the new password after a change (max 32 characters).
	Newpassword string
}

func (f *FramePasswordChangeNotification) Command() Command { return GW_PASSWORD_CHANGE_NTF }

func (f *FramePasswordChangeNotification) MarshalPayload() ([]byte, error) {
	if f.Newpassword == "" {
		return nil, fmt.Errorf("FramePasswordChangeNotification: newpassword is empty")
	}
	return stringToBytes(f.Newpassword, 32)
}

func (f *FramePasswordChangeNotification) UnmarshalPayload(payload []byte) error {
	if len(payload) != 32 {
		return fmt.Errorf("FramePasswordChangeNotification: invalid payload len %d, want 32", len(payload))
	}
	f.Newpassword = bytesToString(payload)
	return nil
}

var _ Frame = (*FramePasswordChangeNotification)(nil)

// ---------------------------------------------------------------------------
// reboot
// ---------------------------------------------------------------------------

// FrameGatewayRebootRequest implements Frame for GW_REBOOT_REQ.
type FrameGatewayRebootRequest struct{}

func (f *FrameGatewayRebootRequest) Command() Command { return GW_REBOOT_REQ }

func (f *FrameGatewayRebootRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGatewayRebootRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGatewayRebootRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGatewayRebootRequest)(nil)

// FrameGatewayRebootConfirmation implements Frame for GW_REBOOT_CFM.
type FrameGatewayRebootConfirmation struct{}

func (f *FrameGatewayRebootConfirmation) Command() Command { return GW_REBOOT_CFM }

func (f *FrameGatewayRebootConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGatewayRebootConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGatewayRebootConfirmation: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGatewayRebootConfirmation)(nil)

// ---------------------------------------------------------------------------
// factory_default
// ---------------------------------------------------------------------------

// FrameGatewayFactoryDefaultRequest implements Frame for GW_SET_FACTORY_DEFAULT_REQ.
type FrameGatewayFactoryDefaultRequest struct{}

func (f *FrameGatewayFactoryDefaultRequest) Command() Command { return GW_SET_FACTORY_DEFAULT_REQ }

func (f *FrameGatewayFactoryDefaultRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGatewayFactoryDefaultRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGatewayFactoryDefaultRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGatewayFactoryDefaultRequest)(nil)

// FrameGatewayFactoryDefaultConfirmation implements Frame for GW_SET_FACTORY_DEFAULT_CFM.
type FrameGatewayFactoryDefaultConfirmation struct{}

func (f *FrameGatewayFactoryDefaultConfirmation) Command() Command {
	return GW_SET_FACTORY_DEFAULT_CFM
}

func (f *FrameGatewayFactoryDefaultConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGatewayFactoryDefaultConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGatewayFactoryDefaultConfirmation: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGatewayFactoryDefaultConfirmation)(nil)

// ---------------------------------------------------------------------------
// leave_learn_state
// ---------------------------------------------------------------------------

// FrameLeaveLearnStateRequest implements Frame for GW_LEAVE_LEARN_STATE_REQ.
type FrameLeaveLearnStateRequest struct{}

func (f *FrameLeaveLearnStateRequest) Command() Command { return GW_LEAVE_LEARN_STATE_REQ }

func (f *FrameLeaveLearnStateRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameLeaveLearnStateRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameLeaveLearnStateRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameLeaveLearnStateRequest)(nil)

// FrameLeaveLearnStateConfirmation implements Frame for GW_LEAVE_LEARN_STATE_CFM.
type FrameLeaveLearnStateConfirmation struct {
	// Status is the result of the leave-learn-state operation.
	Status LeaveLearnStateConfirmationStatus
}

func (f *FrameLeaveLearnStateConfirmation) Command() Command { return GW_LEAVE_LEARN_STATE_CFM }

func (f *FrameLeaveLearnStateConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{uint8(f.Status)}, nil
}

func (f *FrameLeaveLearnStateConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 1 {
		return fmt.Errorf("FrameLeaveLearnStateConfirmation: invalid payload len %d, want 1", len(payload))
	}
	f.Status = LeaveLearnStateConfirmationStatus(payload[0])
	return nil
}

var _ Frame = (*FrameLeaveLearnStateConfirmation)(nil)

// ---------------------------------------------------------------------------
// get_version
// ---------------------------------------------------------------------------

// FrameGetVersionRequest implements Frame for GW_GET_VERSION_REQ.
type FrameGetVersionRequest struct{}

func (f *FrameGetVersionRequest) Command() Command { return GW_GET_VERSION_REQ }

func (f *FrameGetVersionRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGetVersionRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetVersionRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetVersionRequest)(nil)

// FrameGetVersionConfirmation implements Frame for GW_GET_VERSION_CFM.
// Payload: 6 bytes software version + 1 byte hardware version + 1 byte product
// group + 1 byte product type = 9 bytes total.
type FrameGetVersionConfirmation struct {
	// SoftwareVersion holds the 6 individual version number bytes.
	SoftwareVersion [6]byte
	// HardwareVersion is the hardware revision byte.
	HardwareVersion uint8
	// ProductGroup identifies the product family (14 = KLF 200).
	ProductGroup uint8
	// ProductType identifies the product type within the group (3 = KLF 200).
	ProductType uint8
}

func (f *FrameGetVersionConfirmation) Command() Command { return GW_GET_VERSION_CFM }

// SoftwareVersionString returns the software version as "a.b.c.d.e.f".
func (f *FrameGetVersionConfirmation) SoftwareVersionString() string {
	parts := make([]string, 6)
	for i, b := range f.SoftwareVersion {
		parts[i] = fmt.Sprintf("%d", b)
	}
	return strings.Join(parts, ".")
}

// Product returns a human-readable product name.
func (f *FrameGetVersionConfirmation) Product() string {
	if f.ProductGroup == 14 && f.ProductType == 3 {
		return "KLF 200"
	}
	return fmt.Sprintf("Unknown Product: %d:%d", f.ProductGroup, f.ProductType)
}

func (f *FrameGetVersionConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 9)
	copy(payload[0:6], f.SoftwareVersion[:])
	payload[6] = f.HardwareVersion
	payload[7] = f.ProductGroup
	payload[8] = f.ProductType
	return payload, nil
}

func (f *FrameGetVersionConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 9 {
		return fmt.Errorf("FrameGetVersionConfirmation: invalid payload len %d, want 9", len(payload))
	}
	copy(f.SoftwareVersion[:], payload[0:6])
	f.HardwareVersion = payload[6]
	f.ProductGroup = payload[7]
	f.ProductType = payload[8]
	return nil
}

var _ Frame = (*FrameGetVersionConfirmation)(nil)

// ---------------------------------------------------------------------------
// get_protocol_version
// ---------------------------------------------------------------------------

// FrameGetProtocolVersionRequest implements Frame for GW_GET_PROTOCOL_VERSION_REQ.
type FrameGetProtocolVersionRequest struct{}

func (f *FrameGetProtocolVersionRequest) Command() Command { return GW_GET_PROTOCOL_VERSION_REQ }

func (f *FrameGetProtocolVersionRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGetProtocolVersionRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetProtocolVersionRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetProtocolVersionRequest)(nil)

// FrameGetProtocolVersionConfirmation implements Frame for GW_GET_PROTOCOL_VERSION_CFM.
// Payload: 2-byte big-endian major + 2-byte big-endian minor = 4 bytes.
type FrameGetProtocolVersionConfirmation struct {
	// MajorVersion is the protocol major version number.
	MajorVersion uint16
	// MinorVersion is the protocol minor version number.
	MinorVersion uint16
}

func (f *FrameGetProtocolVersionConfirmation) Command() Command { return GW_GET_PROTOCOL_VERSION_CFM }

func (f *FrameGetProtocolVersionConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], f.MajorVersion)
	binary.BigEndian.PutUint16(payload[2:4], f.MinorVersion)
	return payload, nil
}

func (f *FrameGetProtocolVersionConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 4 {
		return fmt.Errorf("FrameGetProtocolVersionConfirmation: invalid payload len %d, want 4", len(payload))
	}
	f.MajorVersion = binary.BigEndian.Uint16(payload[0:2])
	f.MinorVersion = binary.BigEndian.Uint16(payload[2:4])
	return nil
}

var _ Frame = (*FrameGetProtocolVersionConfirmation)(nil)

// ---------------------------------------------------------------------------
// set_utc
// ---------------------------------------------------------------------------

// FrameSetUTCRequest implements Frame for GW_SET_UTC_REQ.
// Payload: 4-byte big-endian Unix timestamp.
type FrameSetUTCRequest struct {
	// Timestamp is the Unix timestamp (seconds since epoch) to set on the gateway.
	Timestamp uint32
}

func (f *FrameSetUTCRequest) Command() Command { return GW_SET_UTC_REQ }

func (f *FrameSetUTCRequest) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, f.Timestamp)
	return payload, nil
}

func (f *FrameSetUTCRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 4 {
		return fmt.Errorf("FrameSetUTCRequest: invalid payload len %d, want 4", len(payload))
	}
	f.Timestamp = binary.BigEndian.Uint32(payload[0:4])
	return nil
}

var _ Frame = (*FrameSetUTCRequest)(nil)

// FrameSetUTCConfirmation implements Frame for GW_SET_UTC_CFM.
type FrameSetUTCConfirmation struct{}

func (f *FrameSetUTCConfirmation) Command() Command { return GW_SET_UTC_CFM }

func (f *FrameSetUTCConfirmation) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameSetUTCConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameSetUTCConfirmation: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameSetUTCConfirmation)(nil)

// ---------------------------------------------------------------------------
// get_local_time
// ---------------------------------------------------------------------------

// LocalTime holds the KLF200 local-time data object.
// Wire layout (15 bytes), ported from pyvlx DtoLocalTime:
//
//	[0:4]  UTCTime   — big-endian Unix timestamp
//	[4]    Second
//	[5]    Minute
//	[6]    Hour
//	[7]    Day
//	[8]    Month
//	[9:11] Year      — big-endian, years since 1900
//	[11]   Weekday   — 1=Monday … 7=Sunday (0 wraps to Sunday=6 in Python)
//	[12:14] DayOfYear — big-endian
//	[14]   IsDST     — signed byte (-1 = unknown, 0 = no, 1 = yes)
type LocalTime struct {
	// UTCTime is the UTC instant.
	UTCTime time.Time
	// LocalTime is the local instant with calendar fields.
	LocalTime time.Time
}

// FrameGetLocalTimeRequest implements Frame for GW_GET_LOCAL_TIME_REQ.
type FrameGetLocalTimeRequest struct{}

func (f *FrameGetLocalTimeRequest) Command() Command { return GW_GET_LOCAL_TIME_REQ }

func (f *FrameGetLocalTimeRequest) MarshalPayload() ([]byte, error) {
	return []byte{}, nil
}

func (f *FrameGetLocalTimeRequest) UnmarshalPayload(payload []byte) error {
	if len(payload) != 0 {
		return fmt.Errorf("FrameGetLocalTimeRequest: invalid payload len %d, want 0", len(payload))
	}
	return nil
}

var _ Frame = (*FrameGetLocalTimeRequest)(nil)

// FrameGetLocalTimeConfirmation implements Frame for GW_GET_LOCAL_TIME_CFM.
// Payload is 15 bytes encoding a LocalTime (ported from DtoLocalTime).
type FrameGetLocalTimeConfirmation struct {
	// Time holds the UTC and local time returned by the gateway.
	Time LocalTime
}

func (f *FrameGetLocalTimeConfirmation) Command() Command { return GW_GET_LOCAL_TIME_CFM }

func (f *FrameGetLocalTimeConfirmation) MarshalPayload() ([]byte, error) {
	payload := make([]byte, 15)

	utcSec := f.Time.UTCTime.Unix()
	binary.BigEndian.PutUint32(payload[0:4], uint32(utcSec))

	lt := f.Time.LocalTime
	payload[4] = byte(lt.Second())
	payload[5] = byte(lt.Minute())
	payload[6] = byte(lt.Hour())
	payload[7] = byte(lt.Day())
	// Month: wire is 0-based "Months since January" (spec §6.4.6.6, range 0-11);
	// Go's time.Month is 1-based, so subtract 1 at the wire boundary.
	payload[8] = byte(int(lt.Month()) - 1)
	yearsOff := lt.Year() - 1900
	binary.BigEndian.PutUint16(payload[9:11], uint16(yearsOff))

	// Weekday: Monday=1 … Saturday=6, Sunday=0 (matches pyvlx: weekday()==6 -> 0, else weekday()+1)
	wd := lt.Weekday() // time.Sunday=0, time.Monday=1 … time.Saturday=6
	if wd == time.Sunday {
		payload[11] = 0
	} else {
		payload[11] = byte(wd) // Monday=1 … Saturday=6
	}

	yday := lt.YearDay()
	binary.BigEndian.PutUint16(payload[12:14], uint16(yday))

	// IsDST: encoded as signed byte. Go doesn't expose DST flag directly; use 0.
	// For the marshal direction we encode 0 (not in DST) as the zero value.
	payload[14] = 0

	return payload, nil
}

func (f *FrameGetLocalTimeConfirmation) UnmarshalPayload(payload []byte) error {
	if len(payload) != 15 {
		return fmt.Errorf("FrameGetLocalTimeConfirmation: invalid payload len %d, want 15", len(payload))
	}

	utcSec := int64(binary.BigEndian.Uint32(payload[0:4]))
	f.Time.UTCTime = time.Unix(utcSec, 0).UTC()

	// local calendar fields — ported byte-for-byte from DtoLocalTime.from_payload
	second := int(payload[4])
	minute := int(payload[5])
	hour := int(payload[6])
	day := int(payload[7])
	// Month: wire is 0-based "Months since January" (spec §6.4.6.6, range 0-11);
	// Go's time.Month is 1-based, so add 1 at the wire boundary.
	month := time.Month(int(payload[8]) + 1)
	year := int(binary.BigEndian.Uint16(payload[9:11])) + 1900

	// weekday: on-wire 1=Monday … 7=Sunday, 0 wraps to Sunday (Python: weekday-1, -1 -> 6=Sunday)
	// We reconstruct from the calendar date so we don't need to parse the wire weekday explicitly.

	isDST := int8(payload[14]) // signed: -1 unknown, 0 no, 1 yes — informational only

	// Build local time.  pyvlx uses time.mktime with the local timezone;
	// we use time.Local to mirror that behaviour.
	_ = isDST // stored in UTCTime/LocalTime indirectly via location
	f.Time.LocalTime = time.Date(year, month, day, hour, minute, second, 0, time.Local)

	return nil
}

var _ Frame = (*FrameGetLocalTimeConfirmation)(nil)

// ---------------------------------------------------------------------------
// init — register all frames
// ---------------------------------------------------------------------------

func init() {
	RegisterFrame(GW_PASSWORD_ENTER_REQ, func() Frame { return &FramePasswordEnterRequest{} })
	RegisterFrame(GW_PASSWORD_ENTER_CFM, func() Frame { return &FramePasswordEnterConfirmation{} })
	RegisterFrame(GW_PASSWORD_CHANGE_REQ, func() Frame { return &FramePasswordChangeRequest{} })
	RegisterFrame(GW_PASSWORD_CHANGE_CFM, func() Frame { return &FramePasswordChangeConfirmation{} })
	RegisterFrame(GW_PASSWORD_CHANGE_NTF, func() Frame { return &FramePasswordChangeNotification{} })
	RegisterFrame(GW_REBOOT_REQ, func() Frame { return &FrameGatewayRebootRequest{} })
	RegisterFrame(GW_REBOOT_CFM, func() Frame { return &FrameGatewayRebootConfirmation{} })
	RegisterFrame(GW_SET_FACTORY_DEFAULT_REQ, func() Frame { return &FrameGatewayFactoryDefaultRequest{} })
	RegisterFrame(GW_SET_FACTORY_DEFAULT_CFM, func() Frame { return &FrameGatewayFactoryDefaultConfirmation{} })
	RegisterFrame(GW_LEAVE_LEARN_STATE_REQ, func() Frame { return &FrameLeaveLearnStateRequest{} })
	RegisterFrame(GW_LEAVE_LEARN_STATE_CFM, func() Frame { return &FrameLeaveLearnStateConfirmation{} })
	RegisterFrame(GW_GET_VERSION_REQ, func() Frame { return &FrameGetVersionRequest{} })
	RegisterFrame(GW_GET_VERSION_CFM, func() Frame { return &FrameGetVersionConfirmation{} })
	RegisterFrame(GW_GET_PROTOCOL_VERSION_REQ, func() Frame { return &FrameGetProtocolVersionRequest{} })
	RegisterFrame(GW_GET_PROTOCOL_VERSION_CFM, func() Frame { return &FrameGetProtocolVersionConfirmation{} })
	RegisterFrame(GW_SET_UTC_REQ, func() Frame { return &FrameSetUTCRequest{} })
	RegisterFrame(GW_SET_UTC_CFM, func() Frame { return &FrameSetUTCConfirmation{} })
	RegisterFrame(GW_GET_LOCAL_TIME_REQ, func() Frame { return &FrameGetLocalTimeRequest{} })
	RegisterFrame(GW_GET_LOCAL_TIME_CFM, func() Frame { return &FrameGetLocalTimeConfirmation{} })
}
