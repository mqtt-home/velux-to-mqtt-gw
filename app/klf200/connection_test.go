package klf200

import (
	"crypto/sha1" //nolint:gosec // fingerprint, not signature.
	"strings"
	"testing"
)

func TestVerifyPinnedFingerprint_Match(t *testing.T) {
	raw := []byte("fake DER bytes for a KLF-200 leaf certificate")
	pinned := sha1.Sum(raw) //nolint:gosec // fingerprint, not signature.
	if err := verifyPinnedFingerprint([][]byte{raw}, pinned); err != nil {
		t.Fatalf("expected match, got error: %v", err)
	}
}

func TestVerifyPinnedFingerprint_Mismatch(t *testing.T) {
	raw := []byte("some other certificate")
	var pinned [20]byte // all-zero, definitely not the SHA-1 of `raw`
	err := verifyPinnedFingerprint([][]byte{raw}, pinned)
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "does not match pinned") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestVerifyPinnedFingerprint_NoCert(t *testing.T) {
	err := verifyPinnedFingerprint(nil, veluxCertFingerprintSHA1)
	if err == nil {
		t.Fatal("expected error when peer sent no cert, got nil")
	}
	if !strings.Contains(err.Error(), "no certificate") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

// TestVeluxFingerprint_Constant guards against accidental edits to the pinned
// bytes: the expected value is the SHA-1 fingerprint published in
// MiSchroe/klf-200-api PR #255 (02:8C:23:A0:...:6A).
func TestVeluxFingerprint_Constant(t *testing.T) {
	want := [20]byte{
		0x02, 0x8C, 0x23, 0xA0, 0x89, 0x2B, 0x62, 0x98,
		0xC4, 0x99, 0x00, 0x5B, 0xD2, 0xE7, 0x2E, 0x0A,
		0x70, 0x3D, 0x71, 0x6A,
	}
	if veluxCertFingerprintSHA1 != want {
		t.Fatalf("pinned fingerprint drift: got %x want %x", veluxCertFingerprintSHA1, want)
	}
}
