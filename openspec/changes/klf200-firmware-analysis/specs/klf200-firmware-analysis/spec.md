## ADDED Requirements

### Requirement: Firmware provenance and non-redistribution

The tooling and repository SHALL treat the KLF200 firmware binary as an owned, non-redistributable
artifact: the pipeline SHALL operate on a locally supplied `.bin` path and the repository SHALL NOT
commit the firmware binary or any binary segment derived from it. Documentation SHALL state that
analysis is limited to firmware for a device the operator owns, for interoperability/defect
research.

#### Scenario: Firmware supplied locally, not committed

- **WHEN** an analyst runs the pipeline
- **THEN** the firmware path is provided as an argument and the repository contains only tooling,
  recipes, and derived textual findings — no firmware binary or extracted binary segment

#### Scenario: Acquisition method documented

- **WHEN** an analyst needs to obtain the firmware
- **THEN** the README documents capturing it from the device's own update fetch, without shipping
  the binary

### Requirement: Encryption/plaintext triage gate

The tooling SHALL, as its first step, assess whether a firmware image is plaintext or
encrypted/compressed (e.g. via an entropy measurement) and SHALL stop with a recorded dead-end
result when the image is not statically analysable.

#### Scenario: High-entropy image halts the pipeline

- **WHEN** the entropy assessment indicates a uniformly high-entropy (encrypted/compressed) image
- **THEN** the tooling reports the image as not statically analysable and records the entropy
  evidence, without attempting disassembly

#### Scenario: Plaintext image proceeds

- **WHEN** the entropy assessment indicates structured, lower-entropy content
- **THEN** the tooling proceeds to image-boundary and load-address recovery

### Requirement: Image boundary and load-address recovery

The tooling SHALL identify the number of firmware images and their in-file offsets and SHALL
recover each image's load base address by validating the ARM Cortex-M vector table (initial stack
pointer into RAM, reset handler into flash with the Thumb bit set).

#### Scenario: Single image base address recovered

- **WHEN** a plaintext image is analysed
- **THEN** the tooling reports a load base address consistent with the vector table (e.g.
  `0x08000000` for the STM32F427 image, `0x00000000` for the EFM32GG990 image)

#### Scenario: Concatenated images split

- **WHEN** two valid vector tables are detected in one `.bin`
- **THEN** the tooling reports both offsets and produces per-image byte ranges suitable for
  splitting

### Requirement: Compiler and TLS-stack fingerprinting

The tooling SHALL scan a plaintext image for build-toolchain and cryptographic-library signatures
and SHALL record the findings verbatim rather than inferring them.

#### Scenario: Toolchain identified

- **WHEN** the image contains a recognizable compiler signature (e.g. `GCC: (GNU) …`, IAR, or Keil
  markers)
- **THEN** the tooling reports the matched compiler string

#### Scenario: TLS stack identified

- **WHEN** the image contains a recognizable TLS-library banner (e.g. mbedTLS, wolfSSL)
- **THEN** the tooling reports the matched library, since it scopes the handshake-freeze
  investigation

### Requirement: Deterministic decompiler loading recipe

The repository SHALL document a deterministic recipe to load each recovered image into a
free decompiler (Ghidra) with the correct processor variant and base address, so analysis is
reproducible.

#### Scenario: Ghidra recipe reproducible

- **WHEN** an analyst follows the documented Ghidra recipe for an image
- **THEN** the project loads with language `ARM:LE:32:Cortex`, the recovered base address, and a
  defined vector table, yielding a decompilable reset handler

### Requirement: Root-cause findings document

The repository SHALL contain a findings document that records the investigation outcome for the
network/TLS-handshake freeze — either evidence pointing at the responsible subsystem/code, or a
documented dead-end with the reason analysis was blocked.

#### Scenario: Findings recorded on success

- **WHEN** analysis yields evidence about the freeze
- **THEN** the findings document describes the responsible subsystem and the supporting evidence
  (addresses, functions, strings) without reproducing proprietary code

#### Scenario: Findings recorded on dead-end

- **WHEN** analysis is blocked (e.g. encrypted image)
- **THEN** the findings document records the blocker and its evidence so the effort is not repeated
  blindly
