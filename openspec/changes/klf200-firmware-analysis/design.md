## Context

Hardware is known from FCC filing XSG832160 (BOM + internal photos):

- **Application/radio MCU**: STM32F427IIH6 — ARM Cortex-M4, 2 MB flash @ `0x0800_0000`, RAM @ `0x2000_0000`; paired with a 256 Mbit NOR flash, an SP-7DZX WiFi chip, and a 2.4 GHz radio (Atmel AT86RF233 + TI CC2590 front-end).
- **Network/API MCU**: SiLabs EFM32GG990F1024 "Giant Gecko" — ARM Cortex-M3, 1 MB flash @ `0x0000_0000`, RAM @ `0x2000_0000`; paired with a WIZnet W5500 hardwired-TCP/IP controller and a second 256 Mbit NOR flash. This subsystem runs the TLS/SLIP server on port 51200 and is the prime suspect for the freeze.

Firmware `.bin` is distributed via `updates2.velux.com`; version 2.0.0.71 (2018-09-27) hardened security, so the image may be encrypted or signed. The analysis must degrade gracefully into a documented dead-end if so.

## Goals / Non-Goals

**Goals:**
- A deterministic, re-runnable path from a firmware `.bin` to a Ghidra-loadable, decompilable project for each MCU image.
- Automated triage that answers: encrypted or not? how many images and where? which core/base address? which compiler and TLS stack?
- A findings document that ties evidence to the freeze hypothesis (network/TLS subsystem) or records why analysis was blocked.

**Non-Goals:**
- No firmware modification, re-flashing, or custom-firmware build (that is a separate, higher-risk effort).
- No redistribution of Velux firmware or any derived binary/segment in the repo.
- No change to the gateway runtime; this is analysis tooling only.
- No live hardware exploitation, JTAG glitching, or key extraction — static analysis of an already-obtained image only.

## Decisions

### D1: Keep it out of the Go module, in `research/klf200-firmware/`
The tooling is developer-only (shell/Python around `binwalk`, `arm-none-eabi-binutils`, Ghidra headless). It must not enter the Go build, `go.mod`, or the Docker image. A dedicated `research/` directory with its own README keeps prerequisites and provenance separate from the shipped product.

### D2: Encryption gate first, fail loud
The pipeline runs an entropy check (`binwalk -E`) as step 1 and classifies the image as plaintext vs. encrypted/compressed before any disassembly work. If high-entropy/uniform, it stops and writes a dead-end finding with the entropy evidence — no wasted effort, no false expectations.

### D3: Recover load address from the Cortex-M vector table, don't guess
For each candidate image, read word[0] (initial SP → must point into `0x2000_xxxx` RAM) and word[1] (reset handler → odd, points into flash). This validates the base address (`0x0800_0000` for STM32, `0x0000_0000` for EFM32) and locates image boundaries when two images are concatenated. Split with `dd` at detected offsets.

### D4: Compiler & TLS-stack fingerprint via strings, recorded as facts
`strings` grep for `GCC: (GNU)` (arm-none-eabi-gcc), IAR copyright markers (IAR EWARM), Keil/armcc, plus `mbedtls`/`wolfSSL`/`PolarSSL` banners. The TLS stack identity narrows the freeze search dramatically. Results captured verbatim in FINDINGS.md rather than asserted from memory.

### D5: Ghidra as primary decompiler, recipe-driven
Provide a documented Ghidra import recipe (language `ARM:LE:32:Cortex`, per-image base address, load vector table as pointer array, run SVD-Loader with the STM32F427 / EFM32GG990 SVD from the vendor CMSIS packs to name peripheral registers — especially the W5500 SPI path and TLS timers). radare2/objdump documented as fallbacks. No proprietary tool required.

### D6: Findings target the network subsystem first
Analysis prioritizes the EFM32 image (network/TLS/W5500), since the freeze manifests as an SSL-handshake stall. The STM32/radio image is secondary.

## Risks / Trade-offs

- **Firmware encrypted/signed (likely post-2.0.0.71)** → Mitigation: D2 gate; a documented dead-end is a valid, useful outcome (tells us not to keep trying statically).
- **Cannot cleanly obtain the `.bin`** → Mitigation: document the capture method (MITM the device's update fetch from `updates2.velux.com`); acquisition is a prerequisite task, not part of the tooling guarantees.
- **Legal exposure** → Mitigation: scope to owned-device interoperability/defect research (EU 2009/24/EC art. 6 interoperability + national research exemptions); never commit or redistribute firmware or derived binaries; findings describe behavior, not reproduce code.
- **Effort may not yield an actionable fix** → Mitigation: even a negative result (confirming the freeze is in non-patchable code) validates the hardware power-cycle mitigation and closes the question; treat the findings doc as the deliverable, not a code patch.
- **Tooling rot** → Mitigation: pin tool versions and record exact commands in the recipe so results are reproducible.

## Open Questions

- Is the 2.0.0.71 image plaintext? (Answered by the first pipeline run; determines whether the rest of the change is executable.)
- Which firmware version to analyze — the latest, or the specific one on the user's device? (Default: the version currently on the device, captured from its own update fetch.)
- Should confirmed findings feed back into a lighter-weight in-protocol workaround proposal (avoiding the mains power-cycle)? (Deferred until findings exist.)

## References

Hardware identification (dual-MCU, network subsystem):

- FCC filing XSG832160 (IC 8642A-832160) — index: https://fcc.report/FCC-ID/XSG832160
- FCC Bill of Materials (part numbers: STM32F427IIH6, EFM32GG990F1024, W5500, AT86RF233, CC2590, …): https://fcc.report/FCC-ID/XSG832160/3937659.pdf
- FCC internal photos (PCB top/bottom): https://fcc.report/FCC-ID/XSG832160/3937686.pdf

Firmware distribution & versions:

- Velux KLF200 product/API page: https://www.velux.com/api/klf200
- Firmware update host (device-facing; not a public download): https://updates2.velux.com/
- HA community — firmware 2.0.0.71 (released 2018-09-27, security hardening, HTTP→SLIP): https://community.home-assistant.io/t/velux-component-for-klf-200-doesnt-support-the-new-api-with-firmware-2-0-0-71/75641
- Julius2342 gist — KLF200 security flaw + firmware version/filename evidence (e.g. `KLF200-v1.1.0.44.bin`): https://gist.github.com/Julius2342/6282ded9f527e762ea50f42c2c439a1a
- MiSchroe/klf-200-api — new-firmware API notes: https://github.com/MiSchroe/klf-200-api

Freeze root-cause context (why the analysis targets the network/TLS subsystem):

- pyvlx #30: https://github.com/Julius2342/pyvlx/issues/30
- HA core #23748: https://github.com/home-assistant/core/issues/23748
