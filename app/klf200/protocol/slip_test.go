package protocol

import (
	"bytes"
	"testing"
)

// TestSlipPackTokenizeRoundTrip verifies pack -> GetNextSlip recovers the
// original body for a range of payloads, including ones containing the
// special END / ESC bytes.
func TestSlipPackTokenizeRoundTrip(t *testing.T) {
	cases := [][]byte{
		{},
		{0x00},
		{0x01, 0x02, 0x03},
		{SlipEnd},                      // payload is a bare END byte
		{SlipEsc},                      // payload is a bare ESC byte
		{SlipEnd, SlipEsc},             // both specials adjacent
		{SlipEsc, SlipEnd, SlipEsc},    // interleaved
		{0xC0, 0xDB, 0xDC, 0xDD, 0xC0}, // END, ESC, ESC_END, ESC_ESC, END
		{0x11, SlipEnd, 0x22, SlipEsc, 0x33},
	}
	for _, orig := range cases {
		packed := SlipPack(orig)
		// Framed packet must begin and end with END.
		if len(packed) < 2 || packed[0] != SlipEnd || packed[len(packed)-1] != SlipEnd {
			t.Fatalf("SlipPack(%v) not END-framed: %v", orig, packed)
		}
		got, remaining := GetNextSlip(packed)
		if !bytes.Equal(got, orig) {
			t.Errorf("round-trip mismatch: SlipPack->GetNextSlip(%v) = %v", orig, got)
		}
		if len(remaining) != 0 {
			t.Errorf("expected no remaining bytes for %v, got %v", orig, remaining)
		}
	}
}

// TestSlipEncodeDecodeRoundTrip checks encode/decode escaping directly.
func TestSlipEncodeDecodeRoundTrip(t *testing.T) {
	cases := [][]byte{
		{},
		{SlipEnd},
		{SlipEsc},
		{SlipEnd, SlipEsc, SlipEnd},
		{0x00, 0xC0, 0xDB, 0xDC, 0xDD, 0xFF},
	}
	for _, orig := range cases {
		enc := SlipEncode(orig)
		// Encoded body must not contain a raw END byte.
		if bytes.IndexByte(enc, SlipEnd) != -1 {
			t.Errorf("SlipEncode(%v) contains raw END: %v", orig, enc)
		}
		dec := SlipDecode(enc)
		if !bytes.Equal(dec, orig) {
			t.Errorf("encode/decode mismatch: %v -> %v -> %v", orig, enc, dec)
		}
	}
}

// TestSlipEncodeKnownVectors pins the exact escape sequences (ported from slip.py).
func TestSlipEncodeKnownVectors(t *testing.T) {
	// END -> ESC ESC_END
	if got := SlipEncode([]byte{SlipEnd}); !bytes.Equal(got, []byte{SlipEsc, SlipEscEnd}) {
		t.Errorf("encode END = %v, want %v", got, []byte{SlipEsc, SlipEscEnd})
	}
	// ESC -> ESC ESC_ESC
	if got := SlipEncode([]byte{SlipEsc}); !bytes.Equal(got, []byte{SlipEsc, SlipEscEsc}) {
		t.Errorf("encode ESC = %v, want %v", got, []byte{SlipEsc, SlipEscEsc})
	}
	// SlipPack of empty body = END END
	if got := SlipPack(nil); !bytes.Equal(got, []byte{SlipEnd, SlipEnd}) {
		t.Errorf("pack empty = %v, want [C0 C0]", got)
	}
}

// TestGetNextSlipFragmentedStream verifies streaming/fragmentation handling:
// partial data yields (nil, raw) unchanged; multiple packets are extracted one
// at a time; leftover bytes are returned as remaining.
func TestGetNextSlipFragmentedStream(t *testing.T) {
	p1 := SlipPack([]byte{0x01, 0x02, SlipEnd}) // contains a special byte
	p2 := SlipPack([]byte{0xAA, 0xBB})

	full := append(append([]byte{}, p1...), p2...)

	// Feed the stream one byte at a time. Only when a complete packet is
	// buffered should GetNextSlip return it.
	var buf []byte
	var packets [][]byte
	for _, b := range full {
		buf = append(buf, b)
		for {
			pkt, rem := GetNextSlip(buf)
			if pkt == nil {
				break
			}
			packets = append(packets, pkt)
			buf = rem
		}
	}
	if len(buf) != 0 {
		t.Errorf("expected buffer fully consumed, leftover %v", buf)
	}
	if len(packets) != 2 {
		t.Fatalf("expected 2 packets, got %d", len(packets))
	}
	if !bytes.Equal(packets[0], []byte{0x01, 0x02, SlipEnd}) {
		t.Errorf("packet 0 = %v", packets[0])
	}
	if !bytes.Equal(packets[1], []byte{0xAA, 0xBB}) {
		t.Errorf("packet 1 = %v", packets[1])
	}

	// Incomplete packet (no terminating END) returns unchanged.
	partial := []byte{SlipEnd, 0x01, 0x02}
	pkt, rem := GetNextSlip(partial)
	if pkt != nil {
		t.Errorf("expected nil packet for partial stream, got %v", pkt)
	}
	if !bytes.Equal(rem, partial) {
		t.Errorf("expected unchanged remaining, got %v", rem)
	}
}
