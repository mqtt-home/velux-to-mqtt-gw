# KLF200 firmware analysis

## Scope

Reverse-engineering tooling for firmware images of the **Velux KLF200** gateway,
for **interoperability and defect research on hardware we own**. Motivated by
the device's ~24 h TLS-handshake freeze on port 51200 that no in-protocol
recovery (soft reboot, reconnect) reliably clears, and which has forced this
project to fall back to a hardware power-cycle mitigation. The goal is to find
evidence of the root cause in the network subsystem — or a documented
dead-end that closes the question and validates the mitigation.

**Non-goals:** no firmware modification, no re-flashing, no key extraction, no
JTAG glitching. Static analysis of an already-obtained image only.

## Hardware map

From FCC filing XSG832160 (BOM + internal photos):

| Subsystem                | MCU                          | Flash base    | Flash size | RAM base    | Notes                                     |
|--------------------------|------------------------------|---------------|------------|-------------|-------------------------------------------|
| Application + radio      | STM32F427IIH6 (Cortex-M4)    | `0x0800_0000` | 2 MB       | `0x2000_0000` | 256 Mbit NOR + SP-7DZX WiFi + AT86RF233 (Zigbee/io-homecontrol radio) |
| Network / API / TLS      | EFM32GG990F1024 (Cortex-M3)  | `0x0000_0000` | 1 MB       | `0x2000_0000` | 256 Mbit NOR + WIZnet W5500 (hardwired TCP/IP). Runs the TLS/SLIP server on `:51200`. **Prime suspect for the freeze.** |

The WiFi module and the WIZnet Ethernet controller are separate SoCs; they
run their own firmware and are driven over SPI by the MCUs.

## Prerequisites

The pipeline itself is **pure Python 3.8+ and uses only the stdlib**, so it
runs on any dev machine without extra installs. For deeper analysis after
triage:

- `arm-none-eabi-binutils` — for `objdump`/`nm` on any recovered image
- [Ghidra](https://ghidra-sre.org/) — primary decompiler (headless works)
- radare2 (optional) — fallback disassembler
- SVD-Loader Ghidra plugin — to name peripheral registers from vendor SVDs
  (STM32F427 and EFM32GG990 SVDs come from the vendor CMSIS packs)
- `binwalk` (optional) — for cross-checking entropy plots and known-format
  signatures; not required by the pipeline

Firmware images themselves are **not committed to this repository** and never
should be — see `.gitignore` at the repo root.

## Firmware acquisition

The Velux firmware updater host is `updates2.velux.com`. Firmware is fetched
by the device itself (not offered as a public download), so acquisition means
observing the device's own update fetch:

1. Put the KLF200 on a network where you can intercept its outbound HTTPS to
   `updates2.velux.com` — for example a lab router with `tcpdump` or a
   mitmproxy transparent proxy.
2. Trigger a firmware check from the KLF200 web UI (Settings → Firmware).
3. Capture the request path (URL and any auth headers) and, if the response
   is HTTPS, install a mitmproxy CA on the device or replay the request from
   another host using the observed URL.
4. Save the downloaded `.bin` under `research/klf200-firmware/samples/`
   (git-ignored). The known filename shape is
   `ID<content-id>_<serial>-01_KLF200-v<version>.bin`.

Do not redistribute the binary. It is copyrighted Velux material; possession
of your own device's firmware for interoperability/defect research is covered
under EU 2009/24/EC art. 6.

## Usage

```bash
python3 triage.py samples/ID10124554_149884-01_KLF200-v2.0.0.71.bin --out out
```

Outputs:

- `out/triage.txt` — human-readable summary
- `out/triage.json` — machine-readable report (stable schema; safe to diff
  between runs on the same input)

The pipeline is deterministic: same input bytes → identical `triage.json`.
It does the following, in order:

1. Detects the `"VELUX FIRMWARE"` container magic and product tag.
2. Measures Shannon entropy globally and in a 4 KB sliding window.
3. Segments the file into contiguous regions labelled
   `high_entropy` (encrypted or densely compressed), `code_or_binary`, or
   `text_or_ui`.
4. In every non-`high_entropy` region, scans for ARM Cortex-M vector-table
   candidates (word-aligned initial SP into the target's RAM window; reset
   handler with the Thumb bit set into the target's flash window).
5. Extracts printable strings and fingerprints the build toolchain (GCC,
   IAR, Keil), TLS stack (mbedTLS, wolfSSL, PolarSSL, OpenSSL), RTOS
   (FreeRTOS, CMSIS-RTOS, ThreadX), and network stack (lwIP, W5500, WIZnet).
6. Emits `dd` split commands for each recovered image.
7. Applies a synthesis rule: if the payload is large yet contains no valid
   vector table AND no toolchain/TLS/RTOS strings, records a dead-end with
   the encrypted-bytes accounting so the reason for stopping is on record.

## Next steps after triage

- **Plaintext image, vector table found** → use `dd` split commands to
  extract per-image `.bin`s (git-ignored), then follow `docs/ghidra-recipe.md`.
- **Dead-end** → see `FINDINGS.md` for what the triage evidence tells us and
  what avenues remain (dynamic analysis on the device, JTAG on the debug pads,
  chip-off flash reading — all out of scope here).
