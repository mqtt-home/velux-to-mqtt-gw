package protocol

import (
	"encoding/binary"
	"fmt"
)

// Command is a 2-byte KLF200 GW_* command code (transmitted big-endian).
type Command uint16

// Frame is the common interface implemented by every GW_* frame.
//
// It mirrors pyvlx's FrameBase model:
//   - Command returns the frame's command code (constant per concrete type).
//   - MarshalPayload returns the frame's payload (the bytes between the command
//     and the checksum). It corresponds to FrameBase.get_payload.
//   - UnmarshalPayload populates the frame from a payload. It corresponds to
//     FrameBase.from_payload.
type Frame interface {
	Command() Command
	MarshalPayload() ([]byte, error)
	UnmarshalPayload(payload []byte) error
}

// CalcCRC computes the KLF200 checksum: XOR of all bytes.
// Ported from frame_helper.calc_crc.
func CalcCRC(raw []byte) byte {
	var crc byte
	for _, b := range raw {
		crc ^= b
	}
	return crc
}

// BuildFrame builds the raw on-wire bytes for a command and payload.
//
// Layout (ported from FrameBase.build_frame):
//
//	[0]      protocol id (always 0x00)
//	[1]      packet length = 2 + len(payload) + 1
//	[2:4]    command, big-endian
//	[4:...]  payload
//	[last]   CRC = XOR over all preceding bytes
func BuildFrame(command Command, payload []byte) []byte {
	packetLength := 2 + len(payload) + 1
	ret := make([]byte, 0, 2+2+len(payload)+1)
	ret = append(ret, 0, byte(packetLength))
	ret = binary.BigEndian.AppendUint16(ret, uint16(command))
	ret = append(ret, payload...)
	ret = append(ret, CalcCRC(ret))
	return ret
}

// MarshalFrame builds the full raw on-wire bytes for a Frame.
// It corresponds to FrameBase.__bytes__.
func MarshalFrame(frame Frame) ([]byte, error) {
	payload, err := frame.MarshalPayload()
	if err != nil {
		return nil, err
	}
	return BuildFrame(frame.Command(), payload), nil
}

// ExtractFromFrame validates a raw (de-SLIPed) frame and returns its command
// and payload. Ported from frame_helper.extract_from_frame.
func ExtractFromFrame(data []byte) (Command, []byte, error) {
	if len(data) <= 4 {
		return 0, nil, fmt.Errorf("could not extract from frame: too short (%d bytes)", len(data))
	}
	length := int(data[0])*256 + int(data[1]) - 1
	if len(data) != length+3 {
		return 0, nil, fmt.Errorf(
			"could not extract from frame: invalid length (current=%d expected=%d)",
			len(data), length+3)
	}
	if crc := CalcCRC(data[:len(data)-1]); crc != data[len(data)-1] {
		return 0, nil, fmt.Errorf(
			"could not extract from frame: invalid crc (current=0x%02X expected=0x%02X)",
			data[len(data)-1], crc)
	}
	payload := append([]byte(nil), data[4:len(data)-1]...)
	command := Command(int(data[2])*256 + int(data[3]))
	return command, payload, nil
}
