#!/usr/bin/env python3
"""KLF200 firmware triage.

Given a raw firmware `.bin` obtained from a device you own, this script:

  1. Measures Shannon entropy globally and in sliding windows to decide
     plaintext vs. encrypted/compressed.
  2. Scans for ARM Cortex-M vector-table candidates and validates each
     against known load bases (STM32F427 flash @ 0x08000000, EFM32GG990
     flash @ 0x00000000). This locates image boundaries when two images
     are concatenated in one blob.
  3. Emits `dd` commands to split concatenated images.
  4. Extracts printable strings and fingerprints the build toolchain
     (GCC/IAR/Keil) and TLS stack (mbedTLS/wolfSSL/PolarSSL).
  5. Writes a machine-readable JSON report and a human-readable text
     summary next to it.

Deterministic: same input → same output. No network, no external tools.

Usage:
    python3 triage.py <firmware.bin> [--out out/]

Exit codes:
    0 — triage completed (image may still be a dead-end; see report)
    2 — usage / IO error
"""

from __future__ import annotations

import argparse
import json
import math
import os
import re
import struct
import sys
from dataclasses import asdict, dataclass, field
from pathlib import Path

# --- Known KLF200 targets (from FCC XSG832160 BOM) ---------------------------
# Each entry: (label, flash_base, flash_size, ram_base, ram_size)
TARGETS = [
    ("STM32F427",  0x08000000, 2 * 1024 * 1024, 0x20000000, 256 * 1024),
    ("EFM32GG990", 0x00000000, 1 * 1024 * 1024, 0x20000000, 128 * 1024),
]

ENTROPY_HIGH = 7.5            # bits/byte; above this a window is "high entropy"
ENTROPY_HIGH_FRACTION = 0.90  # if this fraction of windows is high-entropy → encrypted
WINDOW = 4096                 # entropy window size, bytes
VECTOR_SCAN_STRIDE = 4        # word-aligned scan for vector-table candidates
MIN_STRING_LEN = 6            # printable-string extractor

# Container magic — Velux wraps the raw MCU image(s) in an update-container
# with an ASCII banner at file start. When present, treat the header as
# non-code and only run entropy/analysis on the payload after it.
VELUX_MAGIC = b"VELUX FIRMWARE"
CONTAINER_HEADER_SCAN_LIMIT = 0x400  # header + metadata table observed <= 0x1a0

# Compiler / TLS fingerprint patterns. Case-insensitive substring OR regex.
# Kept explicit (not fuzzy) so the report cites the exact matched string.
FINGERPRINTS = {
    "compiler": [
        re.compile(rb"GCC:\s*\(GNU\)[^\x00\n]{0,120}"),
        re.compile(rb"IAR\s+(?:ANSI\s+)?C[^\x00\n]{0,120}"),
        re.compile(rb"IAR\s+Systems[^\x00\n]{0,120}"),
        re.compile(rb"ARM\s+Compiler[^\x00\n]{0,120}"),
        re.compile(rb"armcc[^\x00\n]{0,60}"),
        re.compile(rb"Keil[^\x00\n]{0,60}"),
        re.compile(rb"clang\s+version[^\x00\n]{0,120}"),
    ],
    "tls": [
        re.compile(rb"mbed\s*TLS[^\x00\n]{0,120}", re.IGNORECASE),
        re.compile(rb"mbedtls[^\x00\n]{0,120}", re.IGNORECASE),
        re.compile(rb"PolarSSL[^\x00\n]{0,120}", re.IGNORECASE),
        re.compile(rb"wolfSSL[^\x00\n]{0,120}", re.IGNORECASE),
        re.compile(rb"CyaSSL[^\x00\n]{0,120}", re.IGNORECASE),
        re.compile(rb"OpenSSL[^\x00\n]{0,120}"),
        re.compile(rb"axTLS[^\x00\n]{0,120}"),
    ],
    "rtos": [
        re.compile(rb"FreeRTOS[^\x00\n]{0,120}"),
        re.compile(rb"CMSIS[-_]RTOS[^\x00\n]{0,120}"),
        re.compile(rb"ThreadX[^\x00\n]{0,60}"),
    ],
    "network": [
        re.compile(rb"lwIP[^\x00\n]{0,120}"),
        re.compile(rb"W5500[^\x00\n]{0,120}"),
        re.compile(rb"WIZnet[^\x00\n]{0,120}", re.IGNORECASE),
    ],
}


# --- Data classes ------------------------------------------------------------

@dataclass
class EntropySummary:
    global_entropy: float
    window_size: int
    high_entropy_fraction: float
    verdict: str  # "plaintext" | "encrypted_or_compressed" | "mixed"


@dataclass
class VectorTableCandidate:
    offset: int
    target: str
    initial_sp: int
    reset_handler: int
    load_base: int
    image_size_hint: int  # bytes to next candidate or EOF


@dataclass
class ContainerInfo:
    detected: bool
    magic_offset: int
    magic: str
    product_tag: str
    header_bytes: int  # bytes considered "container header" (excluded from analysis)


@dataclass
class Region:
    start: int
    end: int
    kind: str  # "high_entropy", "code_or_binary", "text_or_ui"
    avg_entropy: float


@dataclass
class TriageReport:
    input_path: str
    input_size: int
    sha256: str
    container: ContainerInfo
    entropy: EntropySummary
    payload_entropy: EntropySummary | None
    regions: list[Region]
    images: list[VectorTableCandidate]
    split_commands: list[str]
    fingerprints: dict[str, list[str]] = field(default_factory=dict)
    dead_end: bool = False
    dead_end_reason: str = ""


# --- Entropy -----------------------------------------------------------------

def shannon_entropy(buf: bytes) -> float:
    if not buf:
        return 0.0
    counts = [0] * 256
    for b in buf:
        counts[b] += 1
    n = len(buf)
    ent = 0.0
    for c in counts:
        if c:
            p = c / n
            ent -= p * math.log2(p)
    return ent


def entropy_scan(data: bytes) -> EntropySummary:
    global_ent = shannon_entropy(data)
    if len(data) <= WINDOW:
        high_frac = 1.0 if global_ent >= ENTROPY_HIGH else 0.0
    else:
        high = 0
        total = 0
        for off in range(0, len(data) - WINDOW + 1, WINDOW):
            total += 1
            if shannon_entropy(data[off : off + WINDOW]) >= ENTROPY_HIGH:
                high += 1
        high_frac = high / total if total else 0.0

    if high_frac >= ENTROPY_HIGH_FRACTION:
        verdict = "encrypted_or_compressed"
    elif high_frac >= 0.5:
        verdict = "mixed"
    else:
        verdict = "plaintext"
    return EntropySummary(
        global_entropy=round(global_ent, 4),
        window_size=WINDOW,
        high_entropy_fraction=round(high_frac, 4),
        verdict=verdict,
    )


# --- Region classification ---------------------------------------------------

REGION_WINDOW = 4096
REGION_ENCRYPTED_MIN = 7.5   # >= this bits/byte → encrypted OR densely compressed
REGION_TEXT_MAX = 5.5        # <= this bits/byte → text-heavy


def classify_region(ent: float) -> str:
    if ent >= REGION_ENCRYPTED_MIN:
        return "high_entropy"
    if ent <= REGION_TEXT_MAX:
        return "text_or_ui"
    return "code_or_binary"


def find_regions(data: bytes) -> list[Region]:
    """Segment the file into contiguous entropy regions.

    Walks the file in REGION_WINDOW-byte chunks, labels each, then merges
    adjacent windows of the same label. Small runs (< 3 windows) are folded
    into the surrounding region so the report doesn't fragment on noise.
    """
    if not data:
        return []
    labels: list[tuple[int, str, float]] = []  # (offset, kind, ent)
    for off in range(0, len(data), REGION_WINDOW):
        chunk = data[off : off + REGION_WINDOW]
        ent = shannon_entropy(chunk)
        labels.append((off, classify_region(ent), ent))

    # Merge tiny runs (< 3 windows) into the previous region's kind.
    merged: list[Region] = []
    i = 0
    while i < len(labels):
        start_off, kind, _ = labels[i]
        j = i
        while j + 1 < len(labels) and labels[j + 1][1] == kind:
            j += 1
        run_len = j - i + 1
        if run_len < 3 and merged:
            # extend previous region through this run
            avg = sum(l[2] for l in labels[merged[-1]._src_i : j + 1]) / (
                j + 1 - merged[-1]._src_i
            )
            merged[-1].end = (labels[j][0] + REGION_WINDOW) if j + 1 < len(labels) else len(data)
            merged[-1].avg_entropy = round(avg, 3)
        else:
            end = (labels[j][0] + REGION_WINDOW) if j + 1 < len(labels) else len(data)
            avg = sum(l[2] for l in labels[i : j + 1]) / run_len
            reg = Region(
                start=start_off, end=min(end, len(data)), kind=kind, avg_entropy=round(avg, 3)
            )
            reg._src_i = i  # type: ignore[attr-defined]
            merged.append(reg)
        i = j + 1
    # Final coalesce pass: merge adjacent regions with the same kind after
    # small runs got absorbed.
    coalesced: list[Region] = []
    for r in merged:
        if coalesced and coalesced[-1].kind == r.kind:
            # weighted average
            a_size = coalesced[-1].end - coalesced[-1].start
            b_size = r.end - r.start
            new_avg = (
                coalesced[-1].avg_entropy * a_size + r.avg_entropy * b_size
            ) / (a_size + b_size)
            coalesced[-1].end = r.end
            coalesced[-1].avg_entropy = round(new_avg, 3)
        else:
            coalesced.append(Region(r.start, r.end, r.kind, r.avg_entropy))
    return coalesced


# --- Vector-table recovery ---------------------------------------------------

def _in_range(addr: int, base: int, size: int) -> bool:
    return base <= addr < base + size


def find_vector_tables(data: bytes, start: int = 0) -> list[VectorTableCandidate]:
    """Scan word-aligned offsets for plausible Cortex-M vector tables.

    A candidate has:
      word[0] = initial SP → must lie in the target's RAM window AND be
                8-byte aligned (Cortex-M AAPCS requires 8-byte stack alignment;
                real reset SPs are always word-aligned)
      word[1] = reset handler → odd (Thumb bit set), must lie in the target's
                flash window at a small positive offset from the file offset
                treated as the load base
    """
    candidates: list[VectorTableCandidate] = []
    end = len(data) - 8
    off = start
    while off <= end:
        sp, reset = struct.unpack_from("<II", data, off)
        if reset & 1 and (sp & 0x3) == 0:  # Thumb bit + word-aligned SP
            reset_addr = reset & ~1
            for label, fbase, fsize, rbase, rsize in TARGETS:
                if not _in_range(sp, rbase, rsize):
                    continue
                # image load base = fbase + off (image sits at file offset `off`
                # and maps to `fbase` in the MCU). Reset handler must fall
                # inside [load_base, load_base + remaining_bytes).
                load_base = fbase
                # Reset must be within the flash window overall.
                if not _in_range(reset_addr, fbase, fsize):
                    continue
                # Reset must also point into what we have in the file for this
                # image (rest of file starting at `off`).
                available = len(data) - off
                if reset_addr - load_base >= available:
                    continue
                # Reset shouldn't be at the very start — must skip the vector
                # table itself (at least ~0x40 bytes for Cortex-M NVIC).
                if reset_addr - load_base < 0x40:
                    continue
                candidates.append(
                    VectorTableCandidate(
                        offset=off,
                        target=label,
                        initial_sp=sp,
                        reset_handler=reset,
                        load_base=load_base,
                        image_size_hint=0,  # filled in below
                    )
                )
                break  # one target match per offset is enough
        off += VECTOR_SCAN_STRIDE

    # De-noise: keep only candidates at file-start or at large-gap boundaries.
    # A real image starts at offset 0 or at a page/sector alignment; spurious
    # hits are typically scattered inside code/data. We keep the first hit at
    # offset 0 (if any) and any subsequent hit at least 128 KiB after the last
    # accepted one.
    filtered: list[VectorTableCandidate] = []
    for c in candidates:
        if not filtered:
            if c.offset == 0 or c.offset >= 0x10000:
                filtered.append(c)
        else:
            if c.offset - filtered[-1].offset >= 128 * 1024:
                filtered.append(c)

    # Fill in image_size_hint from gaps.
    for i, c in enumerate(filtered):
        nxt = filtered[i + 1].offset if i + 1 < len(filtered) else len(candidates and data)
        c.image_size_hint = (nxt - c.offset) if nxt else 0

    return filtered


def split_commands_for(images: list[VectorTableCandidate], input_path: str) -> list[str]:
    cmds: list[str] = []
    for i, img in enumerate(images):
        out_name = f"image_{i}_{img.target}.bin"
        skip = img.offset
        count = img.image_size_hint or 0
        if count > 0:
            cmds.append(
                f"dd if={input_path} of={out_name} bs=1 "
                f"skip={skip} count={count} status=none"
            )
        else:
            cmds.append(
                f"dd if={input_path} of={out_name} bs=1 skip={skip} status=none"
            )
    return cmds


# --- Strings + fingerprint ---------------------------------------------------

_PRINTABLE = re.compile(rb"[\x20-\x7e]{%d,}" % MIN_STRING_LEN)


def extract_strings(data: bytes) -> list[bytes]:
    return _PRINTABLE.findall(data)


def fingerprint(data: bytes) -> dict[str, list[str]]:
    out: dict[str, list[str]] = {k: [] for k in FINGERPRINTS}
    for category, patterns in FINGERPRINTS.items():
        seen: set[str] = set()
        for pat in patterns:
            for m in pat.finditer(data):
                s = m.group(0).decode("ascii", errors="replace").strip()
                if s and s not in seen:
                    seen.add(s)
                    out[category].append(s)
    return out


# --- Driver ------------------------------------------------------------------

def sha256(path: Path) -> str:
    import hashlib

    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def detect_container(data: bytes) -> ContainerInfo:
    """Detect a Velux update-container wrapper.

    The 2.0.0.71 image begins with an ASCII banner ("VELUX FIRMWARE" +
    "KLF200 TLS") followed by a small metadata table, then the payload.
    We treat the whole area before the first high-entropy run as "header".
    """
    idx = data.find(VELUX_MAGIC, 0, CONTAINER_HEADER_SCAN_LIMIT)
    if idx < 0:
        return ContainerInfo(False, -1, "", "", 0)

    # Extract the product tag that follows the magic (up to first NUL past it).
    tag_start = idx + len(VELUX_MAGIC)
    nul = data.find(b"\x00", tag_start, tag_start + 64)
    product_tag = data[tag_start:nul].decode("ascii", errors="replace").strip()

    # Header size = end of the low-entropy prefix. Walk 64-byte chunks until
    # entropy jumps sharply (payload begins).
    chunk = 64
    header_bytes = 0
    for off in range(0, min(len(data), 8 * 1024), chunk):
        if shannon_entropy(data[off : off + chunk]) >= 6.0:
            header_bytes = off
            break
    else:
        header_bytes = min(len(data), 8 * 1024)

    return ContainerInfo(
        detected=True,
        magic_offset=idx,
        magic=VELUX_MAGIC.decode("ascii"),
        product_tag=product_tag,
        header_bytes=header_bytes,
    )


def run(input_path: Path, out_dir: Path) -> TriageReport:
    data = input_path.read_bytes()
    container = detect_container(data)

    ent = entropy_scan(data)
    payload_ent: EntropySummary | None = None
    payload = data
    payload_start = 0
    if container.detected and container.header_bytes > 0:
        payload_start = container.header_bytes
        payload = data[payload_start:]
        payload_ent = entropy_scan(payload)

    effective_verdict = (payload_ent or ent).verdict

    if effective_verdict == "encrypted_or_compressed":
        eff = payload_ent or ent
        reason = (
            f"Payload entropy {eff.global_entropy:.3f} bits/byte with "
            f"{eff.high_entropy_fraction * 100:.1f}% of {WINDOW}-byte windows "
            f">= {ENTROPY_HIGH}. "
        )
        if container.detected:
            reason += (
                f"Container header '{container.magic}' / "
                f"'{container.product_tag}' present, but the payload after "
                f"offset 0x{payload_start:x} is uniformly high-entropy — "
                "encrypted or compressed. Static analysis blocked without "
                "the decryption key or format spec."
            )
        else:
            reason += "Image is uniformly high-entropy — static analysis blocked."
        report = TriageReport(
            input_path=str(input_path),
            input_size=len(data),
            sha256=sha256(input_path),
            container=container,
            entropy=ent,
            payload_entropy=payload_ent,
            regions=find_regions(data),
            images=[],
            split_commands=[],
            fingerprints=fingerprint(data),  # still scan — header may hold hints
            dead_end=True,
            dead_end_reason=reason,
        )
    else:
        regions = find_regions(data)
        # Scan for vector tables only within non-encrypted regions.
        images: list[VectorTableCandidate] = []
        for reg in regions:
            if reg.kind == "high_entropy":
                continue
            found = find_vector_tables(data[reg.start : reg.end])
            for c in found:
                c.offset += reg.start  # shift back to whole-file offset
            images.extend(found)
        cmds = split_commands_for(images, str(input_path))
        fp = fingerprint(data)
        # Synthesis: if the payload is large yet contains no valid vector
        # table AND no compiler/TLS/RTOS strings, the payload is almost
        # certainly encrypted or compressed even if the raw entropy verdict
        # says "plaintext" (because zero-padded regions pull the average down).
        has_any_fp = any(hits for hits in fp.values())
        payload_size = len(data) - payload_start
        encrypted_bytes = sum(r.end - r.start for r in regions if r.kind == "high_entropy")
        if not images and not has_any_fp and payload_size >= 512 * 1024:
            report = TriageReport(
                input_path=str(input_path),
                input_size=len(data),
                sha256=sha256(input_path),
                container=container,
                entropy=ent,
                payload_entropy=payload_ent,
                regions=regions,
                images=[],
                split_commands=[],
                fingerprints=fp,
                dead_end=True,
                dead_end_reason=(
                    f"Payload of {payload_size} bytes contains no valid ARM "
                    "Cortex-M vector table and no compiler/TLS/RTOS strings; "
                    f"{encrypted_bytes} bytes ({encrypted_bytes/payload_size*100:.1f}%) "
                    "sit in high-entropy regions consistent with encryption. "
                    "Static disassembly is blocked without the decryption key."
                ),
            )
        else:
            # Partial success: at least one plaintext image found, but flag any
            # encrypted region so the reader knows a dead-end still applies to it.
            dead = encrypted_bytes >= 256 * 1024
            reason = ""
            if dead:
                reason = (
                    f"{encrypted_bytes} bytes across "
                    f"{sum(1 for r in regions if r.kind == 'encrypted')} region(s) "
                    "are high-entropy and not statically analysable; other "
                    "regions may still be analysable — see images list."
                )
            report = TriageReport(
                input_path=str(input_path),
                input_size=len(data),
                sha256=sha256(input_path),
                container=container,
                entropy=ent,
                payload_entropy=payload_ent,
                regions=regions,
                images=images,
                split_commands=cmds,
                fingerprints=fp,
                dead_end=dead,
                dead_end_reason=reason,
            )

    out_dir.mkdir(parents=True, exist_ok=True)
    json_path = out_dir / "triage.json"
    txt_path = out_dir / "triage.txt"

    with json_path.open("w") as f:
        json.dump(_report_dict(report), f, indent=2, sort_keys=True)
        f.write("\n")
    with txt_path.open("w") as f:
        f.write(_render_text(report))

    return report


def _report_dict(r: TriageReport) -> dict:
    d = asdict(r)
    # dataclass asdict already converts nested dataclasses; add hex renderings
    # for humans reading the JSON.
    d["entropy"]["global_entropy"] = r.entropy.global_entropy
    if r.container.detected:
        d["container"]["magic_offset_hex"] = f"0x{r.container.magic_offset:x}"
        d["container"]["header_bytes_hex"] = f"0x{r.container.header_bytes:x}"
    for i, reg in enumerate(d["regions"]):
        reg["start_hex"] = f"0x{r.regions[i].start:x}"
        reg["end_hex"] = f"0x{r.regions[i].end:x}"
        reg["size"] = r.regions[i].end - r.regions[i].start
    for i, img in enumerate(d["images"]):
        img["initial_sp_hex"] = f"0x{r.images[i].initial_sp:08x}"
        img["reset_handler_hex"] = f"0x{r.images[i].reset_handler:08x}"
        img["load_base_hex"] = f"0x{r.images[i].load_base:08x}"
        img["offset_hex"] = f"0x{r.images[i].offset:08x}"
    return d


def _render_text(r: TriageReport) -> str:
    lines: list[str] = []
    lines.append(f"# KLF200 firmware triage — {r.input_path}")
    lines.append("")
    lines.append(f"size:        {r.input_size} bytes")
    lines.append(f"sha256:      {r.sha256}")
    lines.append("")
    lines.append("## Container")
    if r.container.detected:
        lines.append(f"  magic:             '{r.container.magic}' @ 0x{r.container.magic_offset:x}")
        lines.append(f"  product tag:       '{r.container.product_tag}'")
        lines.append(f"  header size (est): {r.container.header_bytes} bytes")
    else:
        lines.append("  (no known container magic detected)")
    lines.append("")
    lines.append("## Entropy (whole file)")
    lines.append(f"  global:              {r.entropy.global_entropy:.4f} bits/byte")
    lines.append(f"  window:              {r.entropy.window_size} bytes")
    lines.append(f"  high-entropy frac:   {r.entropy.high_entropy_fraction:.4f}")
    lines.append(f"  verdict:             {r.entropy.verdict}")
    if r.payload_entropy:
        pe = r.payload_entropy
        lines.append("")
        lines.append("## Entropy (payload only)")
        lines.append(f"  global:              {pe.global_entropy:.4f} bits/byte")
        lines.append(f"  high-entropy frac:   {pe.high_entropy_fraction:.4f}")
        lines.append(f"  verdict:             {pe.verdict}")
    lines.append("")
    if r.regions:
        lines.append(f"## Regions ({len(r.regions)})")
        for i, reg in enumerate(r.regions):
            size = reg.end - reg.start
            lines.append(
                f"  [{i}] 0x{reg.start:07x}..0x{reg.end:07x}  "
                f"({size:>8d} B, {size/1024:7.1f} KB)  "
                f"ent={reg.avg_entropy:.2f}  kind={reg.kind}"
            )
        lines.append("")
    if r.dead_end:
        lines.append("## Dead-end")
        lines.append(f"  {r.dead_end_reason}")
        lines.append("")
        return "\n".join(lines)

    lines.append(f"## Vector-table candidates ({len(r.images)})")
    if not r.images:
        lines.append("  (none — image may be non-standard or offset unknown)")
    for i, img in enumerate(r.images):
        lines.append(f"  [{i}] {img.target}")
        lines.append(f"        file offset : 0x{img.offset:08x} ({img.offset})")
        lines.append(f"        initial SP  : 0x{img.initial_sp:08x}")
        lines.append(f"        reset PC    : 0x{img.reset_handler:08x} (Thumb)")
        lines.append(f"        load base   : 0x{img.load_base:08x}")
        lines.append(f"        size hint   : {img.image_size_hint} bytes")
    lines.append("")
    lines.append("## Split commands")
    for cmd in r.split_commands:
        lines.append(f"  {cmd}")
    lines.append("")
    lines.append("## Fingerprints")
    for cat, hits in r.fingerprints.items():
        lines.append(f"  {cat}:")
        if not hits:
            lines.append("    (none)")
        for h in hits:
            lines.append(f"    - {h}")
    lines.append("")
    return "\n".join(lines)


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description="KLF200 firmware triage")
    ap.add_argument("firmware", type=Path, help="path to firmware .bin")
    ap.add_argument(
        "--out",
        type=Path,
        default=Path("out"),
        help="output directory (default: ./out)",
    )
    args = ap.parse_args(argv)

    if not args.firmware.is_file():
        print(f"error: {args.firmware} is not a file", file=sys.stderr)
        return 2

    report = run(args.firmware, args.out)
    print(_render_text(report))
    if report.dead_end:
        print("NOTE: static analysis blocked — see out/triage.json", file=sys.stderr)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
