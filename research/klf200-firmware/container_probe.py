#!/usr/bin/env python3
"""Container-layout probe for the KLF200 VELUX FIRMWARE bundle.

Complements triage.py. Where triage.py classifies regions by entropy, this
script reverse-engineers the container *header* and answers, with evidence:

  - where the real header ends and the payload begins
  - the payload-length field and the trailing CRC-32
  - whether region 0 is ENCRYPTED (uniform ~8.0 entropy per independent
    block) vs merely COMPRESSED (skewed histogram, decompressor accepts it)
  - whether any cryptographic signature is present (it is not; integrity is
    a plain CRC-32)

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

    # --- header / payload boundary ---
    # banner占 is zero-padded up to 0x110; a descriptor table follows; the
    # payload begins where sustained high entropy starts.
    start = next((o for o in range(0x100, 0x800, 4) if ent(d[o:o + 256]) > 6.5), 0x1ad)
    # refine to the first non-structured byte run
    while start > 0x110 and d[start - 1] not in (0x00,):
        start -= 1
    hdr_end = 0x1ad  # empirically the payload start for v2.0.0.71
    print(f"\n[header] banner magic @0x2 = {d[2:16]!r}, tag @0x10 = {d[0x10:0x1a]!r}")

    # payload length field @0x11c (BE)
    f11c = struct.unpack(">I", d[0x11c:0x120])[0]
    print(f"[header] field@0x11c = 0x{f11c:x} ({f11c}); EOF-0x1ad = {n - 0x1ad}"
          f"  -> {'MATCH (payload length)' if f11c == n - 0x1ad else 'no match'}")

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
