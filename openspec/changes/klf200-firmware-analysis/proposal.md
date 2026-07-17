## Why

The KLF200's ~24 h TLS-handshake freeze is the single biggest reliability problem for this bridge, and every mitigation so far (soft `GW_REBOOT_REQ`, reconnect, and the proposed hardware power-cycle) treats the symptom without knowing the cause. Statically analysing the device firmware — which runs on a known dual-MCU platform (STM32F427 Cortex-M4 + SiLabs EFM32GG990 Cortex-M3 + WIZnet W5500, per FCC filing XSG832160) — could pinpoint the actual defect in the network/TLS subsystem and reveal whether a lighter-weight in-protocol workaround exists (e.g. a keepalive/reset sequence) instead of cutting mains power. This is a research/interoperability effort on hardware we own.

## What Changes

- Add a **reproducible reverse-engineering pipeline** (scripts + documentation) under a new `research/klf200-firmware/` directory that takes a firmware `.bin` and produces triage output: entropy/encryption check, image-boundary detection, Cortex-M vector-table + load-address recovery, and compiler/TLS-stack fingerprinting.
- Add **loader recipes** for Ghidra (base address + processor variant per image) so an analyst can go from raw `.bin` to a decompilable project deterministically.
- Add a **findings document** capturing the root-cause analysis of the freeze (or, if the firmware is encrypted/signed, a documented dead-end with evidence).
- The firmware binary itself is **never committed** (copyright); only tooling, recipes, and derived findings live in the repo.
- No change to the runtime gateway binary or its behavior.

## Capabilities

### New Capabilities
- `klf200-firmware-analysis`: a reproducible toolchain and documented method for acquiring, triaging, disassembling, and decompiling KLF200 firmware images to investigate device-side defects.

### Modified Capabilities
<!-- none: no existing runtime capability changes -->

## Impact

- **New files**: `research/klf200-firmware/` — triage script(s) (entropy, `binwalk` wrapper, vector-table/load-address extractor, `strings`-based compiler/TLS fingerprint), a Ghidra loader recipe, and `FINDINGS.md`.
- **Tooling dependencies** (developer-only, not shipped): `binwalk`, `arm-none-eabi-binutils`, Ghidra (and optionally radare2/Cutter). Documented as prerequisites; not added to the Go module or Docker image.
- **No runtime impact**: the gateway build, image, and behavior are untouched.
- **Legal/ethical**: analysis restricted to firmware for a device the operator owns, for interoperability/defect research (EU software-directive interoperability exemption); no redistribution of Velux firmware or derived binaries.
