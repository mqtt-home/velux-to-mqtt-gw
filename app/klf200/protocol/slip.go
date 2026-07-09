package protocol

import "bytes"

// SLIP (Serial Line Internet Protocol, RFC-1055) constants.
// Ported from slip.py.
const (
	SlipEnd    = 0xC0
	SlipEsc    = 0xDB
	SlipEscEnd = 0xDC
	SlipEscEsc = 0xDD
)

// IsSlip reports whether raw begins a SLIP packet (starts with END and contains
// a further END terminating the packet). Ported from slip.is_slip.
func IsSlip(raw []byte) bool {
	if len(raw) < 2 {
		return false
	}
	return raw[0] == SlipEnd && bytes.IndexByte(raw[1:], SlipEnd) != -1
}

// SlipDecode unescapes a SLIP message body. Ported from slip.decode.
func SlipDecode(raw []byte) []byte {
	out := bytes.ReplaceAll(raw, []byte{SlipEsc, SlipEscEnd}, []byte{SlipEnd})
	out = bytes.ReplaceAll(out, []byte{SlipEsc, SlipEscEsc}, []byte{SlipEsc})
	return out
}

// SlipEncode escapes a raw message body for SLIP transport. Ported from slip.encode.
func SlipEncode(raw []byte) []byte {
	out := bytes.ReplaceAll(raw, []byte{SlipEsc}, []byte{SlipEsc, SlipEscEsc})
	out = bytes.ReplaceAll(out, []byte{SlipEnd}, []byte{SlipEsc, SlipEscEnd})
	return out
}

// GetNextSlip extracts the next complete SLIP packet from a raw byte stream.
// It returns the decoded packet plus the remaining (unconsumed) stream. If no
// complete packet is present, it returns (nil, raw) unchanged, so callers can
// buffer more data and retry — this handles fragmentation. Ported from
// slip.get_next_slip.
func GetNextSlip(raw []byte) (packet []byte, remaining []byte) {
	if !IsSlip(raw) {
		return nil, raw
	}
	length := bytes.IndexByte(raw[1:], SlipEnd)
	slipPacket := SlipDecode(raw[1 : length+1])
	newRaw := raw[length+2:]
	return slipPacket, newRaw
}

// SlipPack wraps a raw message in SLIP framing (END + escaped body + END).
// Ported from slip.slip_pack.
func SlipPack(raw []byte) []byte {
	out := make([]byte, 0, len(raw)+2)
	out = append(out, SlipEnd)
	out = append(out, SlipEncode(raw)...)
	out = append(out, SlipEnd)
	return out
}
