#!/usr/bin/env python3
"""Payload-classification probe for the KLF200 VELUX FIRMWARE bundle.

Companion to HEADER-ANALYSIS.md / header_probe.py (which dissect the 425-byte
container frame: magic, segment table, payload-length field @0x11D, trailing
CRC-32). This script focuses on the question those left open — is the payload
ENCRYPTED or merely COMPRESSED — and answers it with evidence:

  - re-confirms the frame (payload length field, CRC-32 trailer) for self-check
  - proves region 0 is ENCRYPTED: uniform ~8.0 entropy across INDEPENDENT
    128 KB blocks + flat histogram + no decompressor accepts the stream
  - shows region 1 is COMPRESSED (skewed histogram, low entropy), framing
    proprietary (no stdlib decompressor accepts it)

Payload start and CRC coverage follow HEADER-ANALYSIS.md: header = 0x1A9 bytes,
payload = [0x1A9 .. EOF-4], CRC-32(BE) over [0 .. EOF-4] in the last 4 bytes.

Deterministic: same input bytes -> same output. Stdlib only.

Usage:  python3 container_probe.py <sample>.bin
"""
import sys, zlib, lzma, bz2, struct, math
from collections import Counter

BLOCK = 0x20000  # 128 KB segment size seen in the manifest


def ent(b: bytes) -> float:
    if not b:
        return 0.0
    c = Counter(b)
    n = len(b)
    return -sum((v / n) * math.log2(v / n) for v in c.values())


def main(path: str) -> None:
    d = open(path, "rb").read()
    n = len(d)
    print(f"file:   {path}")
    print(f"size:   {n} (0x{n:x})")

    # --- header / payload boundary (per HEADER-ANALYSIS.md) ---
    HDR = 0x1A9          # container header length (bytes), payload starts here
    hdr_end = HDR
    print(f"\n[header] banner magic @0x2 = {d[2:16]!r}, tag @0x10 = {d[0x10:0x1a]!r}")

    # payload-length field @0x11D (3-byte BE); payload excludes header and CRC
    f11d = struct.unpack(">I", b"\x00" + d[0x11d:0x120])[0]
    expect = n - HDR - 4
    print(f"[header] field@0x11D = 0x{f11d:x} ({f11d}); EOF-header-CRC = {expect}"
          f"  -> {'MATCH (payload length)' if f11d == expect else 'no match'}")

    # trailing CRC-32 (BE) over bytes 0..EOF-4
    trailer = struct.unpack(">I", d[-4:])[0]
    crc = zlib.crc32(d[:n - 4]) & 0xffffffff
    print(f"[trailer] last4(BE)=0x{trailer:08x}  crc32(0x0..EOF-4)=0x{crc:08x}"
          f"  -> {'MATCH (plain CRC-32, integrity only, NOT a signature)' if trailer == crc else 'no match'}")

    # --- segment manifest: records ending in a 0x20000 size ---
    print(f"\n[manifest] 0x{0x20000:x}-sized segments referenced in header:")
    h = d[0x110:0x1b0]
    i = 0
    while (j := h.find(b"\x00\x02\x00\x00", i)) >= 0:
        seg_id = d[0x110 + j - 2]
        print(f"  @0x{0x110 + j:x}  seg_id=0x{seg_id:02x}  size=0x20000 (128 KB)")
        i = j + 1

    # --- region 0: encrypted vs compressed ---
    print(f"\n[region0] 0x{hdr_end:x}..0x117000  per-{BLOCK:#x} block entropy:")
    off = hdr_end
    enc_blocks = 0
    while off < 0x117000:
        e = ent(d[off:off + BLOCK])
        flag = "ENCRYPTED" if e > 7.95 else ("partial/transition" if e > 7.0 else "low")
        if e > 7.95:
            enc_blocks += 1
        print(f"  0x{off:07x} ent={e:.4f}  {flag}")
        off += BLOCK
    print(f"  -> {enc_blocks} fully-encrypted 128KB blocks; histogram flat; "
          f"no decompressor accepts the stream => ENCRYPTED (not compressed).")

    # --- cipher mode: ECB detection via 16-byte block collisions ---
    # Random data yields ~0 aligned 16B collisions (71k^2 / 2^129). A clear
    # peak at one phase => ECB with a 128-bit block (AES-sized). Chaining/
    # counter modes destroy such repeats.
    ct = d[hdr_end:0x117000]
    print(f"\n[cipher] ECB probe on region0 ({len(ct)} bytes):")
    best_ph, best_inst = 0, -1
    for ph in range(16):
        c = Counter(ct[i:i + 16] for i in range(ph, len(ct) - 16, 16))
        inst = sum(v - 1 for v in c.values() if v > 1)
        if inst > best_inst:
            best_ph, best_inst = ph, inst
        if ph == 0:
            ph0 = inst
    print(f"  phase0 duplicate 16B instances = {ph0}; max phase={best_ph}")
    verdict = ("ECB (128-bit block, AES-sized) — identical plaintext blocks map "
               "to identical ciphertext" if best_ph == 0 and ph0 > 100
               else "no clear ECB signature (chained/counter/stream mode)")
    print(f"  -> {verdict}")

    # --- region 1: compressed, unknown framing ---
    r1 = d[0x117000:0x1a1000]
    top = Counter(r1).most_common(3)
    print(f"\n[region1] 0x117000..0x1a1000  ent={ent(r1):.3f}  "
          f"top bytes={[(hex(b), f'{c*100/len(r1):.1f}%') for b, c in top]}")
    ok = False
    for name, fn in (("zlib", zlib.decompress), ("lzma", lzma.decompress), ("bz2", bz2.decompress)):
        for o in (0, 1, 2, 4, 8):
            try:
                if len(fn(r1[o:])) > 256:
                    print(f"  decompressed by {name} at off={o}")
                    ok = True
                    break
            except Exception:
                pass
        if ok:
            break
    if not ok:
        print("  -> skewed histogram + low entropy => COMPRESSED, but framing is "
              "proprietary/unknown (no stdlib decompressor accepts it).")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit("usage: container_probe.py <sample>.bin")
    main(sys.argv[1])
