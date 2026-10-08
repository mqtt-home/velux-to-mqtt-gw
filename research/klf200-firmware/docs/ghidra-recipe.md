# Ghidra loader recipe (per-image)

Given a per-image `.bin` produced by the `dd` split commands from
`triage.py`, this recipe loads it into Ghidra deterministically so the
decompiled reset handler is the same for any analyst who follows it.

## Prerequisites

- Ghidra 10.x or newer (11.x tested).
- Optional but recommended: the [SVD-Loader Ghidra plugin][svd-loader] and
  the following vendor SVDs from the CMSIS packs:
  - `STM32F427.svd` — from the STMicroelectronics CMSIS pack for STM32F4.
  - `EFM32GG990F1024.svd` — from the SiLabs Gecko SDK CMSIS pack.

[svd-loader]: https://github.com/leveldown-security/SVD-Loader-Ghidra

## 1. Import (per image)

Use the same recipe for each image; only the base address changes.

1. `File → Import File…` and pick the image `.bin`.
2. In the import dialog:
   - **Format:** `Raw Binary`
   - **Language:** `ARM:LE:32:Cortex` (the "Cortex" variant enables Thumb
     decoding; do not pick `ARM:LE:32:v7`)
   - **Options → Block Name:** `flash`
   - **Options → Base Address:**
     - `0x08000000` for the STM32F427 (application/radio) image
     - `0x00000000` for the EFM32GG990 (network/TLS) image
   - Leave **File Offset = 0** and **Length = (file size)**.
3. Confirm and accept the analysis defaults on first open — but *before*
   running auto-analysis, complete step 2 below.

## 2. Define the vector table

Cortex-M images start with a vector table whose first two words are the
initial stack pointer and the reset-handler pointer (odd = Thumb). Naming
these correctly stops Ghidra from misclassifying the vector table as code.

1. Jump to the image base address.
2. Right-click the first word → `Data → uint32_t` and label it `initial_SP`.
3. Second word → `Data → pointer` → `funct pointer` and label it
   `reset_Handler`. Make sure the low bit is clear when Ghidra follows it
   (Ghidra strips the Thumb bit automatically for `funct` pointers).
4. Select from `+0x08` (word[2]) up to at least `+0x40` (16 more entries;
   covers system exceptions), right-click → `Data → pointer[N]` and label
   the whole block `vector_table`.
5. Navigate to `reset_Handler` (`G`, address). Right-click → `Disassemble`.
   The disassembly must decode cleanly as Thumb-2 (`ldr`, `bl`, `mov`
   instructions, not `.word` blobs). If it decodes as garbage, the base
   address is wrong — go back and re-import.

## 3. (Optional) Load peripheral names with SVD-Loader

With SVD-Loader installed:

1. `Window → Script Manager` → find `svd-loader.py` → Run.
2. Pick the SVD for the correct MCU (`STM32F427.svd` for the STM32 image,
   `EFM32GG990F1024.svd` for the EFM32 image).
3. The peripheral memory-mapped registers (e.g. `USART1_DR`, `SPI1_SR`,
   `WWDG_CR`, EFM32 `USART0_RXDATA`, DMA/GPIO/TIMERs) get named in the
   listing. For the EFM32 image, the W5500 lives on an external SPI bus, so
   look for the SPI peripheral driving CS to the W5500 — that gives you
   the chip-select codepath.

## 4. Run auto-analysis

`Analysis → Auto Analyze…` with defaults. On a 1–2 MB image this takes
minutes. After it settles, key symbols to look for:

- Interrupt handlers named from the vector table (SysTick, PendSV, EXTIn,
  DMAn, SPIn, USARTn, TIMn, ETH — for EFM32 the network path is over SPI to
  the W5500, so USART/SPI IRQs are the network entry point).
- Any `TRAP`, `HardFault`, `assert`, `printf`/`vprintf`-style routines —
  entry points to error paths worth tracing back.

## 5. Fallback: radare2 / arm-none-eabi-objdump

If Ghidra is unavailable:

### radare2
```bash
r2 -a arm -b 16 -m 0x08000000 image_stm32.bin      # STM32 image
r2 -a arm -b 16 -m 0x00000000 image_efm32.bin      # EFM32 image
# in r2:
aaa                     # auto-analyze
s sym.reset_Handler     # or: s 0; pd 32; then follow word[1]
pdf                     # print disassembly of current function
```

### arm-none-eabi-objdump
```bash
arm-none-eabi-objdump -D -b binary -m armv7e-m -M force-thumb \
    --adjust-vma=0x00000000 image_efm32.bin | less

arm-none-eabi-objdump -D -b binary -m armv7e-m -M force-thumb \
    --adjust-vma=0x08000000 image_stm32.bin | less
```

`--adjust-vma` sets the load base. `armv7e-m` covers both Cortex-M3
(EFM32GG) and Cortex-M4 (STM32F4). `-M force-thumb` prevents the
disassembler from guessing the instruction set from ELF flags (there are
none — this is a raw binary).

## Reproducibility

The recipe is intended to be deterministic on the same input:

- Same Ghidra version + same language + same base address → same initial
  auto-analysis result.
- SVD-Loader is idempotent; running it twice on the same project is a no-op.
- `objdump`/`radare2` disassembly is byte-for-byte reproducible for a
  given tool version.

Record the exact Ghidra build (e.g. `Ghidra 11.1.2` build `20240709`) and
the tool versions of `arm-none-eabi-objdump` and `radare2` in
`FINDINGS.md` alongside any concrete findings.
