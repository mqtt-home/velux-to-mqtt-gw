#!/usr/bin/env python3
"""Byte-level analysis of the first 8 KiB (0x0000-0x1FFF) of the KLF200 image.
Real data only: everything printed is computed from the bytes on disk."""
import sys, struct, math, binascii, zlib, re

PATH = sys.argv[1]
with open(PATH, "rb") as f:
    blob = f.read()
hdr = blob[:0x2000]
print(f"file size      : {len(blob)} bytes")
print(f"header slice   : {len(hdr)} bytes (0x0000-0x1FFF)")

def shannon(b):
    if not b: return 0.0
    from collections import Counter
    c = Counter(b); n = len(b)
    return -sum((v/n)*math.log2(v/n) for v in c.values())

# --- 1. hexdump of the meaningful front, then survey the rest ---------------
def hexdump(b, base=0, limit=None):
    out = []
    end = len(b) if limit is None else min(limit, len(b))
    for off in range(0, end, 16):
        chunk = b[off:off+16]
        hexs = " ".join(f"{x:02x}" for x in chunk)
        asci = "".join(chr(x) if 32 <= x < 127 else "." for x in chunk)
        out.append(f"{base+off:08x}  {hexs:<47}  |{asci}|")
    return "\n".join(out)

print("\n=== HEXDUMP 0x0000-0x01FF ===")
print(hexdump(hdr, 0, 0x200))

# --- 2. ASCII strings with offsets within the 8 KiB -------------------------
print("\n=== ASCII strings (len>=4) in 0x0000-0x1FFF ===")
for m in re.finditer(rb"[\x20-\x7e]{4,}", hdr):
    print(f"  0x{m.start():04x} (+{m.end()-m.start():3d})  {m.group().decode('ascii','replace')!r}")

# --- 3. entropy map of the 8 KiB in 256-byte windows ------------------------
print("\n=== entropy per 256B window ===")
nonzero_tail = None
for off in range(0, 0x2000, 256):
    w = hdr[off:off+256]
    e = shannon(w)
    allzero = all(x == 0 for x in w)
    allff = all(x == 0xff for x in w)
    tag = " ALL-00" if allzero else (" ALL-FF" if allff else "")
    print(f"  0x{off:04x}  ent={e:5.2f}{tag}")

# --- 4. structural scan: where does the header 'content' end? ---------------
# find last non-zero byte in the 8 KiB and first long zero run
first_zero_run = None
run_start = None
for i, x in enumerate(hdr):
    if x == 0:
        if run_start is None: run_start = i
    else:
        if run_start is not None and i - run_start >= 16 and first_zero_run is None:
            first_zero_run = (run_start, i)
        run_start = None
last_nonzero = max((i for i, x in enumerate(hdr) if x != 0), default=-1)
print(f"\n=== fill / boundaries ===")
print(f"  first zero-run >=16B : {first_zero_run}")
print(f"  last non-zero byte   : 0x{last_nonzero:04x}")
# byte histogram of the whole 8KiB
from collections import Counter
hist = Counter(hdr)
top = hist.most_common(6)
print(f"  top byte values      : " + ", ".join(f"0x{b:02x}:{c}" for b,c in top))

# --- 5. candidate 16/32-bit fields around the magic & header start ----------
print("\n=== candidate numeric fields (LE/BE) in 0x0000-0x003F ===")
for off in range(0, 0x40, 2):
    u16le = struct.unpack_from("<H", hdr, off)[0]
    u16be = struct.unpack_from(">H", hdr, off)[0]
    line = f"  0x{off:04x}  u16le={u16le:5d}(0x{u16le:04x})  u16be={u16be:5d}(0x{u16be:04x})"
    if off % 4 == 0 and off+4 <= len(hdr):
        u32le = struct.unpack_from("<I", hdr, off)[0]
        u32be = struct.unpack_from(">I", hdr, off)[0]
        line += f"  u32le={u32le}(0x{u32le:08x})  u32be={u32be}(0x{u32be:08x})"
    print(line)

# --- 6. CRC / checksum tests ------------------------------------------------
# Does any 16/32-bit field in the first 64 bytes equal a CRC over a plausible range?
print("\n=== CRC / checksum probing ===")
def crc32(b): return zlib.crc32(b) & 0xffffffff
# a couple of common CRC16 variants
def crc16_ccitt(b, init=0xFFFF):
    crc = init
    for x in b:
        crc ^= x << 8
        for _ in range(8):
            crc = ((crc << 1) ^ 0x1021) & 0xffff if (crc & 0x8000) else (crc << 1) & 0xffff
    return crc
def crc16_modbus(b, init=0xFFFF):
    crc = init
    for x in b:
        crc ^= x
        for _ in range(8):
            crc = (crc >> 1) ^ 0xA001 if (crc & 1) else crc >> 1
    return crc

# collect all 16/32-bit values that appear in first 64 bytes as potential stored checksums
candidates32 = {}
for off in range(0, 0x40, 1):
    if off+4 <= len(hdr):
        candidates32[("le",off)] = struct.unpack_from("<I", hdr, off)[0]
        candidates32[("be",off)] = struct.unpack_from(">I", hdr, off)[0]
candidates16 = {}
for off in range(0, 0x40, 1):
    if off+2 <= len(hdr):
        candidates16[("le",off)] = struct.unpack_from("<H", hdr, off)[0]
        candidates16[("be",off)] = struct.unpack_from(">H", hdr, off)[0]

# ranges to checksum: whole file, header, payload-after-8k, body-after-various-offsets
ranges = {
    "whole_file": blob,
    "after_hdr(8k..end)": blob[0x2000:],
    "hdr[0..0x1fff]": hdr,
    "file_excl_last4": blob[:-4],
    "file[0x10:]": blob[0x10:],
    "file[0x20:]": blob[0x20:],
    "file[0x2000:] excl? none": blob[0x2000:],
}
hits = 0
for rname, data in ranges.items():
    c32 = crc32(data)
    c16c = crc16_ccitt(data)
    c16m = crc16_modbus(data)
    for (endian, off), val in candidates32.items():
        if val == c32 and val != 0:
            print(f"  CRC32 HIT: range={rname} crc=0x{c32:08x} stored@0x{off:04x}({endian})"); hits+=1
    for (endian, off), val in candidates16.items():
        if val == c16c and val not in (0,0xffff):
            print(f"  CRC16-CCITT HIT: range={rname} crc=0x{c16c:04x} stored@0x{off:04x}({endian})"); hits+=1
        if val == c16m and val not in (0,0xffff):
            print(f"  CRC16-MODBUS HIT: range={rname} crc=0x{c16m:04x} stored@0x{off:04x}({endian})"); hits+=1
# also report the bare CRC values for the record
print(f"  [ref] crc32(whole_file)        = 0x{crc32(blob):08x}")
print(f"  [ref] crc32(after 8k payload)  = 0x{crc32(blob[0x2000:]):08x}")
print(f"  [ref] crc32(file excl last 4)  = 0x{crc32(blob[:-4]):08x}")
print(f"  [ref] last 4 bytes of file     = {blob[-4:].hex()} (le=0x{struct.unpack('<I',blob[-4:])[0]:08x} be=0x{struct.unpack('>I',blob[-4:])[0]:08x})")
if hits == 0:
    print("  -> no CRC16/CRC32 field in first 64B matches the tested ranges.")

# --- 7. signature-block heuristic at tail (RSA/ECDSA/PKCS7) -----------------
print("\n=== tail inspection (possible signature/footer) ===")
tail = blob[-512:]
print(hexdump(tail, len(blob)-512))
print(f"  entropy(last 512B)   = {shannon(tail):.2f}")
print(f"  entropy(last 256B)   = {shannon(blob[-256:]):.2f}")
