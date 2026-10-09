# FINDINGS — KLF200 firmware v2.0.0.71 (static analysis, blocked)

## TL;DR

Static disassembly of the network/TLS subsystem is **blocked** on the
2.0.0.71 image: the ~1.1 MB EFM32 image is **encrypted**, so it is not
directly loadable. The hardware power-cycle mitigation already implemented
in this repo remains the appropriate response.

**Update (follow-up analysis — see [`HEADER-ANALYSIS.md`](HEADER-ANALYSIS.md)
and [`TRUST-CHAIN.md`](TRUST-CHAIN.md)):** the container and its protection
model are now fully reversed, which refines the dead-end from "high-entropy,
cause unknown" to a precise statement:

- The file is a container `[header 0x1A9][segment table][payload][CRC-32]`.
  The payload-length field (`0x3E21FC` @ `0x11D`) and a trailing **CRC-32 (BE)
  `0xCADE2565`** over `[0..EOF-4]` are verified to the byte.
- Region 0 (`0x1A9..0x117000`, the EFM32 network/TLS image) is **encrypted in
  ECB mode with a 128-bit block** (AES-sized; AES very likely but not proven):
  uniform ~8.0 entropy across independent 128 KB blocks, and a clear
  phase-0 block-collision peak (1941 repeated 16-B blocks). Region 1 is
  **compressed** (proprietary framing); the web UI is plaintext.
- **No cryptographic signature exists at the container level** — the only
  whole-file check is a CRC-32 (integrity, not authenticity). Whether the
  on-device bootloader verifies a signature *after* decryption is not decidable
  from the file (**Beleg fehlt**).

Reproduce the refined findings:
```bash
python3 container_probe.py samples/<sample>.bin
python3 header_probe.py    samples/<sample>.bin
```

This remains the dead-end path anticipated by the design's "encryption gate
first, fail loud" decision (D2): static disassembly needs the key, which lives
in device flash, not in the file.

## Analysed sample

| Field  | Value                                              |
|--------|----------------------------------------------------|
| File   | `ID10124554_149884-01_KLF200-v2.0.0.71.bin`        |
| Size   | 4 072 361 bytes (~3.88 MiB)                        |
| SHA-256| `364c3b734fe14e4f91f3a079321635f24c358d2f87e217b66f359baecc26e49d` |
| Source | Locally supplied. Not committed to this repo.      |

Reproduce:
```bash
python3 triage.py <path-to-sample>.bin --out out
```

## Container

At file offset `0x0002` the image carries the ASCII banner
`"VELUX FIRMWARE"` followed by the product tag `"KLF200 TLS"`. The real
header is **0x1A9 (425) bytes** — not ~8 KB; the "~8 KB" figure in older
triage output was a scan-limit heuristic. The header holds a segment table
and a payload-length field, then transitions into the payload at `0x1A9`.
The payload is a bundle of an encrypted MCU image (region 0) + a compressed
section (region 1) + plaintext web-UI assets. Full header reversal is in
[`HEADER-ANALYSIS.md`](HEADER-ANALYSIS.md).

## Region map

Emitted by `triage.py` (14 regions, whole-file):

| # | Range                    | Size    | Ent. | Kind          | Notes |
|--:|--------------------------|--------:|-----:|---------------|-------|
| 0 | `0x0000000..0x0117000`   | 1 116 KB | 7.94 | high_entropy  | Matches EFM32GG990 flash size (1 MB). **Very likely the encrypted network/TLS image.** |
| 1 | `0x0117000..0x01a1000`   |   552 KB | 3.90 | text_or_ui\*  | Byte histogram dominated by `0x98` — compressed with a skewed distribution, not text and not encrypted. Format unidentified. |
| 2 | `0x01a1000..0x01bd000`   |   112 KB | 6.79 | code_or_binary| Mid-entropy binary; adjacent to region 3's plaintext strings. |
| 3 | `0x01bd000..0x01c2000`   |    20 KB | 4.52 | text_or_ui    | **Contains Broadcom WiFi driver strings** (`wl%d.%d: %s: scb key id %d does not match %d`) and generic ARM Cortex-M exception-dump format strings (`TRAP %x(%x): pc %x, lr %x, sp %x, psr %x, xpsr %x`). |
| 4 | `0x01c2000..0x01d5000`   |    76 KB | 6.22 | code_or_binary| Mid-entropy binary, no vector table. |
| 5 | `0x01d5000..0x02aa000`   |   852 KB | 4.98 | text_or_ui    | AngularJS + CSS + HTML (KLF200 web UI). |
| 6 | `0x02aa000..0x02ad000`   |    12 KB | 7.80 | high_entropy  | Small opaque blob (key? certificate? compressed sub-asset?). |
| 7 –13 | (see `out/triage.txt`) | ~1 300 KB | 4–5 | text_or_ui + small binary interludes | Localised UI strings (Portuguese, Danish, Norwegian, Finnish, Polish, Spanish, Italian, Lithuanian, Slovak, Turkish, Czech, …), JS assets. |

\* "text_or_ui" is a coarse entropy-based label; region 1 is actually
compressed data, not text — see the histogram note.

**Total high-entropy bytes: 1 183 744 (29.1% of payload).**

## Evidence supporting the dead-end

Every independent signal points the same way:

1. **No ARM Cortex-M vector table anywhere.** The triage script scans
   every word-aligned offset in every non-high-entropy region for a
   candidate whose word[0] is a word-aligned SP inside a known RAM
   window (`0x2000_0000..0x2004_0000` for either MCU) and whose word[1]
   is a Thumb-marked address inside a known flash window
   (`0x0800_0000..0x0820_0000` for STM32F427, `0x0000_0000..0x0010_0000`
   for EFM32GG990) at least `0x40` bytes past the load base. Zero hits.
2. **No compiler banner.** No `GCC: (GNU) …`, no IAR copyright marker,
   no Keil/armcc marker anywhere in the file — for any of the ~4 MB.
3. **No TLS-library banner.** No `mbedTLS`, `PolarSSL`, `wolfSSL`,
   `CyaSSL`, `OpenSSL`, `axTLS` string anywhere. A plaintext TLS stack
   would surface at least a version banner in `.rodata`.
4. **No RTOS or network-stack strings.** No `FreeRTOS`, `CMSIS-RTOS`,
   `ThreadX`, `lwIP`, `WIZnet`, `W5500` string, despite the FCC BOM
   confirming a W5500 is on the board.
5. **Size of region 0 matches EFM32GG990 flash (1 MB) with a small
   wrapper**, consistent with region 0 being the encrypted network-MCU
   image. The freeze lives on the EFM32, per the freeze-symptom evidence
   in the design doc — so the one image we most needed to see is the one
   region that is opaque.

The only fingerprint-worthy plaintext hints found — `wl%d.%d: %s:` and
`TRAP %x` in region 3 — are debug format strings from a Broadcom WiFi
driver and a generic ARM exception handler, respectively. Both are
consistent with STM32-side host code, but do not indicate the TLS stack
and do not open a path to the freeze root cause.

## What this rules in and out

- **Rules in:** version 2.0.0.71 (the "security hardening" release,
  2018-09-27) ships MCU firmware in an encrypted or heavily-compressed
  container. Static analysis of this image alone cannot answer the
  freeze root-cause question without either the decryption key or a
  reversed container/compression format.
- **Rules out:** the freeze cannot be attributed to a specific TLS-stack
  bug (mbedTLS timer wraparound, wolfSSL handshake edge, PolarSSL
  session-cache growth, etc.) from this evidence. Any such attribution
  would be speculation.
- **Neither rules in nor rules out:** whether an in-protocol lighter
  workaround exists (e.g. a specific reset-sequence over SLIP that
  wakes the TLS state machine without a mains cycle). That question
  requires either a plaintext image or dynamic observation on-device.

## Recommendation

1. **Keep the hardware power-cycle mitigation.** The static-analysis
   path was worth attempting once; its dead-end result validates the
   mitigation as the pragmatic answer for this class of freeze.
2. **Do not repeat this analysis on v2.0.0.71.** The pipeline output is
   deterministic; re-running produces the same report. The next useful
   check is a *different* firmware version: if an older, pre-2.0.0.71
   image is obtainable, its earlier (possibly plaintext) TLS code may
   reveal what the current version obscures.
3. **Optional out-of-scope follow-ups**, only if the freeze becomes a
   higher-priority problem:
   - Live traffic capture: MITM the KLF200's outbound TLS handshakes at
     freeze time to see how the state machine stalls at the record
     layer.
   - Hardware debug: SWD pads on the EFM32GG990 (if left populated on
     revision B PCBs) would allow dumping unencrypted flash directly.
     This is intrusive and voids warranty; not recommended.
4. **No follow-up in-protocol workaround proposed** at this time (per
   task 4.4): the evidence does not point at one.

## Method / tool versions (for reproducibility)

- OS: macOS 24.6.0 (Darwin), Python 3.9+
- `triage.py`: stdlib only, no third-party dependencies
- Pipeline invocation: `python3 triage.py <sample>.bin --out out`
- Report artifacts checked into VCS: none of `out/` is committed (see
  `.gitignore`); to reproduce, an analyst supplies their own `.bin`.
