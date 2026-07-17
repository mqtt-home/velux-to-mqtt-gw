## 1. Scaffolding & provenance

- [ ] 1.1 Create `research/klf200-firmware/` with a README stating scope (owned device, interoperability/defect research), the hardware map (STM32F427 Cortex-M4 @ 0x08000000; EFM32GG990 Cortex-M3 @ 0x00000000; W5500), and prerequisites (`binwalk`, `arm-none-eabi-binutils`, Ghidra)
- [ ] 1.2 Add `.gitignore` entries so firmware `.bin` files and extracted binary segments are never committed
- [ ] 1.3 Document the firmware acquisition method (capture the device's update fetch from `updates2.velux.com`); do not ship the binary

## 2. Triage pipeline

- [ ] 2.1 Write `triage.sh`/`triage.py`: run `binwalk -E` and classify plaintext vs. encrypted/compressed by entropy; exit early with a recorded dead-end result if high-entropy
- [ ] 2.2 Add vector-table/load-address recovery: read word[0] (SP→RAM) and word[1] (reset handler→flash, Thumb bit), validate against known bases, detect one or two images
- [ ] 2.3 Emit per-image byte ranges and `dd` split commands for concatenated images
- [ ] 2.4 Add compiler + TLS-stack fingerprinting via `strings` (GCC/IAR/Keil; mbedTLS/wolfSSL/PolarSSL), captured verbatim to an output report
- [ ] 2.5 Have the pipeline write a machine-readable triage report (entropy, images, bases, compiler, TLS stack)

## 3. Decompiler recipe

- [ ] 3.1 Document a Ghidra import recipe per image: language `ARM:LE:32:Cortex`, correct base address, define vector table as pointer array, define reset handler
- [ ] 3.2 Document loading the STM32F427 and EFM32GG990 SVD (from vendor CMSIS packs) via SVD-Loader to name peripherals (W5500 SPI path, TLS timers)
- [ ] 3.3 Document radare2/`arm-none-eabi-objdump` fallback commands for raw disassembly

## 4. Investigation & findings

- [ ] 4.1 Run the pipeline on the target firmware; record triage output
- [ ] 4.2 If plaintext: load the EFM32 (network/TLS) image in Ghidra, locate the W5500 SPI driver and the TLS handshake state machine, follow the handshake-stall path
- [ ] 4.3 Write `FINDINGS.md`: either evidence of the freeze's responsible subsystem/code (addresses, functions, strings — no proprietary code reproduced) or a documented dead-end with entropy/encryption evidence
- [ ] 4.4 If findings suggest an in-protocol workaround (lighter than a mains power-cycle), note it as a follow-up for a separate runtime proposal

## 5. Verification

- [ ] 5.1 Confirm the pipeline is re-runnable and deterministic (pinned tool versions, exact commands recorded) and produces identical triage output on the same input
- [ ] 5.2 Confirm no firmware binary or derived binary segment is tracked by git; confirm the Go build and Docker image are unaffected
