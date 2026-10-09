# TRUST-CHAIN — KLF200 Vertrauenskette & Sideloading-Analyse

Ergänzung zu [`FINDINGS.md`](FINDINGS.md). Während `FINDINGS.md` den
statischen Disassembly-Versuch an v2.0.0.71 als Dead-End dokumentiert,
rekonstruiert dieses Dokument **die Vertrauenskette vom Firmware-Image bis
zur ausgeführten Software** und bewertet, ob/wie sich das auf eigener
Hardware weiter untersuchen lässt.

**Methodik & Grenzen**

- Dateibezogene Aussagen stammen aus `triage.py` **und** `container_probe.py`,
  beide reproduzierbar gegen `samples/…KLF200-v2.0.0.71.bin`
  (SHA-256 `364c3b73…e49d`). Stand: Container-Header + CRC + Verschlüsselungs-
  nachweis **verifiziert** (siehe §1a).
- SoC-bezogene Aussagen stammen aus der Hersteller-Dokumentation (FCC-BOM +
  ST/SiLabs App-Notes; siehe Quellen).
- Jeder Schluss von einem Hersteller-Referenzdesign auf *dieses* Gerät ist
  ausdrücklich als **Analogie** markiert. Unbelegtes steht als **„Beleg
  fehlt"**.
- Scope: Interoperabilitäts-/Defektforschung an **eigener** Hardware
  (EU 2009/24/EC Art. 6). Keine Flash-/Bypass-Anleitung — die wäre erst
  relevant, wenn die Tests 1–5 (unten) eine prüfende Instanz nachweisen.

---

## 1a. Container & Payload — VERIFIZIERT

Reproduzierbar gegen das Sample (`364c3b73…e49d`). Zwei Skripte, zwei Fragen:

- **Container-Rahmen** → [`HEADER-ANALYSIS.md`](HEADER-ANALYSIS.md) /
  `header_probe.py`: realer Header **425 B (`0x1A9`)** (nicht 8 KB — alte
  Triage-Heuristik), Form `[Header][Segment-Tabelle][Payload][CRC32]`,
  Payload-Längenfeld `0x3E21FC` @`0x11D` (= Datei − Header − CRC, auf das Byte),
  Trailer **CRC-32 (BE) `0xCADE2565`** über `[0..EOF-4]`. **Kein**
  Signaturblock auf Container-Ebene.
- **Payload-Klassifikation** → `container_probe.py` (die von HEADER-ANALYSIS
  offengelassene Frage „verschlüsselt oder nur gepackt?"):

**Die drei Kernfragen — jetzt mit Beweis:**

1. **Region 0 (`0x1A9..0x117000`, ~1,1 MB) ist VERSCHLÜSSELT, nicht
   komprimiert.** Entropie **7,994–7,999 über jeden der 8 unabhängigen
   128-KB-Blöcke** (9. Block 7,52 = Übergang), flaches Histogramm (~0,43 %/
   Byte), kein Dekompressor akzeptiert den Stream. Gleichmäßige ~8,0 über
   *unabhängige* Blöcke ⇒ Block-/Stream-Chiffre. Größe ≈ EFM32GG990-Flash
   (1 MB). → **H1 bestätigt.**
   - **Modus: ECB, 128-Bit-Block (belegt).** Phasentest: bei Phase 0
     (aligned zu `0x1A9`) **1941 wiederholte 16-B-Blöcke** gegenüber ~997 bei
     allen anderen Phasen; ein Block `22408ce7…` 379× (ECB eines konstanten
     Plaintext-Laufs, z. B. leerer Flash). Verkettete/Counter-Modi würden
     solche Wiederholungen zerstören. In 71k Zufallsblöcken wären ~0 Kollisionen
     zu erwarten (≈10⁻²⁹). → kein IV/kein Chaining.
   - **Algorithmus: sehr wahrscheinlich AES (nicht bewiesen).** Begründung:
     EFM32GG990 hat einen HW-AES-Block (128/256), SiLabs-Referenz AN0060 nutzt
     AES. ECB belegt nur Modus+Blockgröße (128 bit), nicht den Algorithmus.
   - **Sicherheitsfolge:** ECB + *eine* Datei für alle Geräte ⇒ mit hoher
     Wahrscheinlichkeit **ein globaler Schlüssel ohne IV**. Dann entschlüsselt
     der Key aus *einem* Gerät *alle* Updates → macht den „Key im Flash"-Pfad
     zum hochwertigen Ziel.
   - **Brute-Force ausgeschlossen:** 2¹²⁸-Schlüsselraum ist physikalisch
     unknackbar. Der Weg zum Klartext führt über den **Schlüssel** (im
     Bootloader-Flash) oder den **Decryptor** (Bootloader reversen), nicht über
     Raten.
2. **Region 1 (`0x117000..0x1a1000`) ist KOMPRIMIERT, nicht verschlüsselt** —
   Entropie 3,97, Histogramm von `0x98` (12,4 %) dominiert, Framing
   proprietär/unbekannt (kein stdlib-Dekompressor greift).
3. **KEINE kryptografische Signatur im Container.** Einziges Integritäts-
   artefakt ist eine **CRC-32** (von jedem neu berechenbar → keine
   Authentizität); Details & Negativ-Belege in `HEADER-ANALYSIS.md`.
   → **H2 für den Container beantwortet: nein.**

**Konsequenz für die Vertrauenskette:** Die Schutzwirkung des *Update-Pakets*
ruht allein auf **Vertraulichkeit (Verschlüsselung Region 0) + CRC-Integrität**
— nicht auf einer Signatur. Die einzige kryptografische Hürde im Container ist
die **Entschlüsselung (Schlüssel nötig)**.

> **Vorbehalt (Beleg fehlt):** Dies betrifft nur das *Containerformat*. Ob der
> **Bootloader im Gerät** beim Flashen zusätzlich signiert/verifiziert, ist aus
> der Datei allein nicht entscheidbar — eine Signatur könnte *innerhalb* der
> entschlüsselten MCU-Payload liegen. Container-seitig ist das einzige
> Auth-Artefakt eine CRC.

Reproduktion: `python3 container_probe.py samples/…v2.0.0.71.bin`

---

## 1. Firmware-Artefakt: ein heterogener Container, nicht ein Image

Das von `updates2.velux.com` ausgelieferte Objekt ist ein **Container-Bündel**
(Magic `"VELUX FIRMWARE"` @ `0x0002`, Tag `"KLF200 TLS"`, ~8 KB Header).
Nur **29,1 %** sind hochentropisch — das erlaubt die Trennung
Verschlüsselung / Kompression / Klartext bereits auf Rohdatenebene:

| Region | Bytes | Entropie | Einordnung (belegt) |
|---|---|---:|---|
| 0 `0x0…0x117000` | 1116 KB | **7.94** | Hochentropisch, Größe ≈ EFM32GG990-Flash (1 MB) → **wahrscheinlich verschlüsseltes Netzwerk/TLS-Image** |
| 1 `0x117000…` | 552 KB | **3.90** | Histogramm von `0x98` dominiert → **komprimiert, NICHT verschlüsselt** (AES wäre ~8.0 & flach) |
| 3 | 20 KB | 4.52 | Klartext: Broadcom-WiFi-Treiber-Strings + ARM-`TRAP %x`-Exception-Strings |
| 5 | 852 KB | 4.98 | Klartext: AngularJS/CSS/HTML Web-UI |
| 6 / 12 | 12 / 28 KB | 7.80 / 7.83 | Kleine opake Blobs → Schlüssel? Zertifikat? gepacktes Sub-Asset? (**Beleg fehlt**) |
| 7–13 | ~1.3 MB | 4–5 | Klartext: lokalisierte UI-Strings, JS-Assets |

**Status signiert / verschlüsselt / komprimiert:**

- **Komprimiert:** ja, belegt (Region 1; Teile der UI).
- **Verschlüsselt:** teilweise plausibel (Region 0 + Mini-Blobs) — **nicht** das
  ganze Image, keine Gesamt-Verschleierung.
- **Signiert:** **unbekannt — Beleg fehlt.** Entropie sagt nichts über eine
  Signatur. Eine RSA/ECDSA-Signatur wäre ein kleiner Anhang und in der groben
  Regionenkarte nicht von „Schlüssel/Zertifikat" unterscheidbar. → offene,
  testbare Frage (§3, Test 1).

Update laut Julius2342-Gist **löscht alle Szenen** → deutet auf
**Vollimage-Austausch mit Flash-Neulayout** hin, nicht differentielles Patch.
Separates Recovery-/Bootloader-Image im Paket: **Beleg fehlt.**

---

## 2. Update-/Sideload-Prozess

**Belegt:**
- Gerät zieht Firmware selbst von `updates2.velux.com` (kein Public Download).
- v2.0.0.71 (2018-09-27, „Security Hardening"): API-Wechsel **HTTP → SLIP**
  über TLS:51200. Älterer Auth-Bypass (Login ohne Passwort → gültiges Token)
  dort geschlossen — betrifft die **API**, nicht die Firmware-Verifikation.

**Beleg fehlt** (genau die gesuchte Kette):
- Auslöser des Updates, prüfende Komponente (Bootloader? Application? Updater?),
  ob überhaupt eine Signatur-/Hash-Prüfung des Pakets stattfindet.
- Alternative Pfade: **kein** USB/SD/eMMC laut BOM. Realistischer Alt-Pfad =
  **SWD/JTAG + serieller ROM-Bootloader der MCUs**, kein dokumentierter
  Recovery-Modus.

---

## 3. Entschlüsselung vs. Signaturprüfung (Hypothesen + Falsifikation)

**H1 — Region 0 ist verschlüsselt (nicht nur gepackt). → BESTÄTIGT (§1a).**
- Beweis: ~8,0 Entropie über 8 unabhängige 128-KB-Blöcke, flaches Histogramm,
  kein Dekompressor akzeptiert den Stream. Der Falsifikationstest
  (Dekompression zu gültiger Vektortabelle) **scheiterte** → bleibt
  verschlüsselt.
- **Analogie (weiterhin nur Analogie):** SiLabs liefert für diese Chipfamilie
  die Referenz **AN0060 „Bootloader with AES Encryption"** (`.EBL`-Format). Ob
  Velux dies oder eine eigene Chiffre nutzt: **Beleg fehlt** — die Chiffre
  (AES-Modus? Blockgröße? IV pro 128-KB-Segment?) ist nicht identifiziert.

**H2 — Signaturprüfung im Container. → FÜR DEN CONTAINER WIDERLEGT (§1a).**
Der 429-B-Header ist vollständig zerlegt (Banner + Manifest), das Dateiende
trägt eine **reine CRC-32** (`0xcade2565`, verifiziert). Kein signaturgroßer
Hochentropie-Block vorhanden. ⇒ Das *Paket* ist nicht signiert.
**Offen bleibt nur:** ob der **Bootloader im Gerät** beim Flashen separat
verifiziert — das ist aus der Datei nicht entscheidbar (→ §5 Schritte 3/5).

**Schlüssel-Lokalisierung (Hypothese):** EFM32GG Series 0 und STM32F427 haben
**kein TrustZone/TEE, kein Secure Element, kein eFuse-Keystore**. Ein etwaiger
AES-Schlüssel läge daher **im Bootloader-Code im Flash** (AN0060-Modell). →
**Wenn** verschlüsselt, ist der Schlüssel mit hoher Wahrscheinlichkeit
auslesbar, sofern der Flash lesbar ist (→ §4 Debug-Lock/RDP). Konkrete
Velux-Implementierung: **Beleg fehlt.**

**Tatsächlich geschützt:** nach Evidenz nur EFM32-Image (Region 0) + zwei
Mini-Blobs. Web-UI/WiFi-Treiber/Lokalisierung liegen im Klartext.

---

## 4. Boot-Chain / „Secure Boot" (Zwei-MCU-Design)

Keine klassische ROM→U-Boot→Kernel-Kette. Die Vertrauensgrenze ist pro MCU ein
**Debug-/RDP-Fuse** plus *möglicherweise* eine Bootloader-Signaturprüfung —
**kein** krypto-RoT (kein signiertes Boot-ROM, kein eFuse-Key).

**EFM32GG990 (Netzwerk/TLS — primäres Ziel):**
1. Boot-ROM (SiLabs, unveränderlich): verbindet beim Reset zuerst den **AAP
   (Authentication Access Port)**, liest ein Lock-Wort. *Belegt.*
2. **AAP-Zustand = eigentliche Grenze.** Nicht gelockt → AAP öffnet AHB-AP →
   **voller SWD-Zugriff** (Dump + Replace trivial). Gelockt → nur AAP, dessen
   einzige nennenswerte Fähigkeit **Mass-Erase** ist. Unterscheidbar am IDR:
   **AHB-AP `0x24770011` vs. AAP `0x16E60001`** — direkt messbar. *Belegt.*
3. Serieller Bootloader (AN0003 UART / XMODEM-CRC, bzw. AN0060 AES): aktiviert?
   über welche Pins? → **Beleg fehlt.**
4. Applikation (TLS/SLIP-Server, treibt W5500 über SPI) = Freeze-Verdächtiger.

**STM32F427 (Applikation/Radio):** Grenze = **RDP**. *Belegt:* RDP-Level-2
(`0xCC`) **irreversibel**, deaktiviert JTAG/SWD dauerhaft, sperrt Boot aus
RAM/System-Memory. RDP-Level-1 erlaubt Debug, **löst aber Mass-Erase bei
Debug-Connect aus**. Gesetzter Level auf dem Gerät: **Beleg fehlt.**

**Konsequenz Manipulation:** Reiner Image-Austausch ist **nur** blockiert,
*wenn* der Bootloader eine Signatur prüft. Prüft er nur CRC (AN0003-Default) →
keine kryptografische Hürde, nur Format/Entschlüsselung. ⇒ **H2 ist der
kritische Unbekannte.**

---

## 5. Logic-/Digital-Analyzer — realistischer Nutzen

Nützlich, aber indirekt und begrenzt:

- **SPI EFM32↔externes NOR (256 Mbit):** größter Hebel — Flash passiv auslesen
  (eigener Inhalt). Grenze: liegt der Code im MCU-internen 1-MB-Flash, sieht der
  SPI-Bus davon nichts.
- **SPI EFM32↔W5500:** TLS-Record-Fluss + CS-Sequenz → relevant fürs
  *Freeze*-Thema, nicht fürs Sideloading.
- **UART:** Debug-Banner / Bootloader-Version / Fehlercodes — lohnendstes
  Low-Cost-Ziel.
- **SWD (Debug-Probe, kein LA):** IDR-Lesen → „Debug-Lock ja/nein" direkt.
- **eMMC/SD/QSPI:** laut BOM **nicht vorhanden** → entfällt.

**Grenze (explizit):** Ein Logic-Analyzer liest **keinen geheimen Schlüssel
„aus"**. Beobachtbar sind nur: Timing/Sequenz von Prüf-/Entschlüsselungs-
schritten, Fehlercodes, Challenge/Response-Muster, Key-*Handles* (nicht der
Key), Abbruch vor/nach einer Prüfung. Der AES-Key wäre nur über einen
**Flash-Dump** erreichbar, nicht über Bus-Sniffing.

---

## 6. Hardware (FCC XSG832160)

**Belegt:**
- STM32F427IIH6 (Cortex-M4, 2 MB intern) + 256-Mbit-NOR + SP-7DZX-WiFi +
  AT86RF233-Radio.
- EFM32GG990F1024 (Cortex-M3, 1 MB intern) + 256-Mbit-NOR + WIZnet W5500.
- **Kein** Secure Element, eMMC/NAND, TEE/TrustZone.

**Beleg fehlt:** Lage/Pinout von SWD-/UART-Pads, Boot-Strap-Widerständen.
SWD-Pads am EFM32 „auf Rev-B evtl. bestückt" — unbestätigt.

---

## 7. Forschungsflächen — priorisiert

| Prio | Fläche | Hardware | Software | Gewinn | Brick-Risiko | Nächster Test |
|---|---|---|---|---|---|---|
| ✅ erledigt | Header zerlegen | keine | `container_probe.py` | Layout + CRC + „keine Signatur" | null | §1a |
| ✅ erledigt | Dekompression gg. Region 0 & 1 | keine | `container_probe.py` | Region0=verschlüsselt, Region1=komprimiert | null | §1a |
| **A** | EFM32 Debug-Lock (IDR) | SWD-Probe | OpenOCD | ob SWD-Dump überhaupt möglich | ~null (nur IDR lesen) | §5 Schritt 3 |
| **B** | UART-Debug beim Boot/Update | LA/USB-UART | sigrok | Bootloader-Banner, Fehlercodes, Flow | null | — |
| **B** | Älteres Pre-2.0.0.71-Image | keine | triage.py | evtl. Klartext-TLS-Code | null | — |
| **A** | Externes NOR passiv auslesen | SOIC-8-Clip + Programmer | flashrom | evtl. unverschlüsselter Code/Key | niedrig | nach IDR-Test |
| **C** | Internen Flash via SWD dumpen | J-Link | OpenOCD | Vollimage + evtl. Key | mittel (Mass-Erase bei Lock) | nur nach IDR-Test |
| **D** | Bypass, falls RDP2/Lock **und** signierender Bootloader | — | — | — | hoch | — (durch Fuse+Krypto blockiert) |

---

## 8. Recherche-Stand

Gefunden: Chip-Doku (STM32 RDP; EFM32 AAP/AN0003/AN0060), FCC-Teardown,
Julius2342-Gist, HA/openHAB-Threads. **Nicht gefunden** (existiert öffentlich
offenbar nicht): KLF200-spezifischer Firmware-Dump, Custom-Firmware-Projekt,
Teardown mit Pinout, RE des Update-Containers. ⇒ unerschlossenes Terrain; alle
Container-/Bootloader-Fragen müssen am eigenen Gerät erstbeantwortet werden.

---

## 9. Ergebnis

**Wahrscheinlichste Architektur:** heterogener Velux-Container; EFM32-Image
(Region 0) verschlüsselt oder stark gepackt, UI/WiFi im Klartext. Die
Vertrauensgrenze beider MCUs ist **kein krypto-RoT**, sondern **Debug-Lock
(EFM32 AAP) bzw. RDP (STM32)** plus eine **unbewiesene** Bootloader-
Signaturprüfung. Kein Secure Element/TEE/eFuse — ein etwaiger AES-Key läge im
Flash-Code.

> **Wahrscheinlichster Erkenntnisweg:** nicht über das Update-Paket, sondern
> über den **Debug-Lock-/RDP-Status der beiden MCUs** und das **passive
> Auslesen des eigenen externen NOR-Flashs**. Nicht gelockt → Image lesbar,
> Bootloader analysierbar, Signaturfrage (H2) endgültig entscheidbar — ohne
> Flash-Versuch. Gelockt → das *ist* die Antwort: statisch blockiert,
> konsistent mit dem bestehenden `FINDINGS.md`-Dead-End.

**Nächste Schritte (Gewinn/Risiko-optimiert):**

- ✅ **Header zerlegt** (§1a): Layout + Payload-Länge + CRC-32 verifiziert,
  keine Container-Signatur. `container_probe.py`.
- ✅ **Dekompression getestet** (§1a): Region 0 verschlüsselt, Region 1
  komprimiert (proprietär).
1. **EFM32 Debug-Lock-Test (Risiko ~0):** SWD-Probe, **nur IDR lesen**
   (`0x24770011` offen / `0x16E60001` gelockt). Kein Schreiben. Entscheidet, ob
   der Flash (und damit ein etwaiger Entschlüsselungs-Schlüssel im
   Bootloader-Code) überhaupt lesbar ist.
2. **STM32 RDP-Level lesen (Risiko ~0, nicht senken):** Option-Byte-Status.
3. **Externes NOR passiv auslesen (Risiko niedrig):** Clip-On-Read, Inhalt mit
   `triage.py`/`container_probe.py` klassifizieren.
4. **Optional, rein analytisch:** Region 1 gegen embedded-typische Kompressoren
   (heatshrink, LZO, LZSS-Varianten) testen; das `0x98`-dominierte Histogramm
   als Fingerabdruck nutzen. Risiko 0, Gewinn: Zugriff auf die Klartext-
   Sektion ohne Hardware.
5. **Chiffre-Identifikation (nur falls Schritt 1/3 einen Flash-Dump liefern):**
   im Bootloader-Code nach AES-Tabellen/Schlüsselplan suchen. Das ist
   *Reverse Engineering des eigenen Dumps*, kein Bypass.

**Aktivitätsklassen (sauber getrennt):**
- *Reines RE:* §1–§4, Schritte 1–2 — Analyse eigener/öffentlicher Daten.
- *Auslesen eigener Geräte-/Firmwaredaten:* Schritte 3–5.
- *Debugging:* UART-Mitschnitt, SWD-Lesezugriff (deckt sich mit Freeze-Thema).
- *Umgehen von Signatur-/Secure-Boot-Prüfungen:* **eigene, höher-riskante
  Kategorie** — erst relevant, wenn Schritte 1–5 eine prüfende Instanz
  nachweisen; hier bewusst **keine** Durchführung vor geklärter Architektur.

---

## Quellen

- FCC XSG832160 (BOM + interne Fotos) — siehe `design.md`
- [Julius2342 — KLF200 security flaw gist](https://gist.github.com/Julius2342/6282ded9f527e762ea50f42c2c439a1a)
- [SiLabs AN0060 — Bootloader with AES Encryption (EFM32GG Series 0)](https://www.silabs.com/documents/public/application-notes/an0060-bootloader-with-aes-encryption.pdf)
- [SiLabs AN0003 — EFM32 UART Bootloader (XMODEM-CRC)](https://www.silabs.com/documents/public/application-notes/an0003-efm32-uart-bootloader.pdf)
- [SiLabs — EFM32 Series 0 Debug Bootloaders / AAP-Lock](https://jp.silabs.com/documents/public/training/mcu/efm32-series-0-debug-bootloaders.pdf)
- [ST Community — STM32F4 RDP Level 2 (irreversibel, JTAG-Disable)](https://community.st.com/t5/stm32-mcus-products/stm32f407-protection-level-2/td-p/571995)
- [Velux firmware update host](https://updates2.velux.com/)

---

## Anhang A — OpenOCD read-only Gate-Test (Schritt 1 & 2)

**Zweck:** Nur *feststellen*, ob die MCUs debug-/lesesperrbar sind — **ohne
jeden Schreibzugriff**. Entscheidet, ob die Entschlüsselungsfrage über einen
Flash-Dump überhaupt angreifbar ist. Alles hier ist **lesend**; die
gefährlichen Schreib-/Erase-Sequenzen sind am Ende explizit als „nicht tippen"
aufgeführt.

### Voraussetzungen

- SWD-Debug-Probe: ST-Link v2/v3, J-Link oder CMSIS-DAP.
- OpenOCD ≥ 0.11 (Befehle `dap apid` / `dap info` sind versionsabhängig —
  Output prüfen, nicht blind vertrauen).
- Verkabelung: **SWCLK, SWDIO, GND**, und **VREF/VTref an 3V3 des Geräts**
  (nur Referenz-Spannung mitlesen, die MCU nicht fremdspeisen). SWD-Pads/
  Pinout am KLF200-PCB: **Beleg fehlt** — vor dem Anschluss per Durchgangs-
  messung gegen die MCU-Pins (EFM32 SWCLK/SWDIO, STM32 PA14/PA13) verifizieren.
- Kein SRST nötig; **nicht** unter Reset halten, nicht resetten.

### Erwartete Diskriminator-Werte (belegt, SiLabs/ARM)

| MCU | offen | gesperrt |
|---|---|---|
| EFM32GG990 | AHB-AP IDR **`0x24770011`** | AAP IDR **`0x16E60001`** |
| STM32F427 | SWD verbindet, Option-Bytes lesbar, RDP 0/1 | RDP2 → SWD verbindet **gar nicht** |

### Schritt 1 — EFM32GG990 Debug-Lock (AP-IDR lesen)

```bash
# Nur AP#0-IDR lesen, keine Flash-/Target-Operation.
openocd \
  -f interface/stlink.cfg \
  -c "transport select swd" \
  -c "adapter speed 100" \
  -c "swj_newdap efm32 cpu -expected-id 0x2ba01477" \
  -c "dap create efm32.dap -chain-position efm32.cpu" \
  -c "init" \
  -c "echo [format {EFM32 AP0 IDR = 0x%08x} [efm32.dap apid 0]]" \
  -c "shutdown"
```

**Interpretation:**
- `0x24770011` → **offen**: der AHB-AP antwortet, Flash via SWD dumpbar
  (→ Schritt 3 / Bootloader reversen → Chiffre + evtl. Schlüssel).
- `0x16E60001` → **gesperrt**: nur der AAP antwortet. Der einzige Ausweg wäre
  ein AAP-Mass-Erase — **das zerstört den Flash und beendet jede statische
  Analyse**; nicht ausführen.
- Verbindet gar nichts → Verkabelung/Pinout prüfen (nicht automatisch „locked").

Alternative, falls `apid` in der OpenOCD-Version fehlt: `-c "dap info 0"`
nach `init` — gibt die AP-Liste inkl. IDR aus.

### Schritt 2 — STM32F427 RDP-Level (Option-Bytes lesen)

```bash
# stm32f2x-Treiber deckt STM32F4 ab. options_read ist rein lesend.
openocd \
  -f interface/stlink.cfg \
  -f target/stm32f4x.cfg \
  -c "init" \
  -c "stm32f2x options_read 0" \
  -c "shutdown"
```

**Interpretation:**
- Gibt `RDP`/`user`-Optionen aus → **RDP 0** (voll lesbar) oder **RDP 1**
  (Debug ok, Flash-Lesen via Debug gesperrt, **kein** Dump ohne RDP-Downgrade).
- `init`/Examine schlägt fehl, keine SWD-Verbindung trotz korrekter
  Verkabelung → **RDP 2** (Debug dauerhaft aus; das ist die Antwort).
- Rein lesend und ungefährlich. **Nicht** `options_write` / RDP-Downgrade
  ausführen: ein RDP1→RDP0-Wechsel löst einen **Mass-Erase** aus.

### NICHT tippen (destruktiv / Vertrauensgrenze verletzend)

Diese Befehle sind kein Teil des Gate-Tests und würden den Flash löschen oder
über reines Lesen hinausgehen:

- EFM32 AAP Mass-Erase: Schreiben auf AAP `CMDKEY`/`CMD` (`DEVICEERASE`) —
  irreversibler Flash+SRAM-Löscher.
- `flash erase_sector` / `flash erase_address` / `stm32f2x mass_erase`.
- STM32 `stm32f2x options_write …` mit geändertem RDP (RDP1→RDP0 = Mass-Erase;
  RDP→Level 2 = irreversibel).
- Jeder `mww`/`write_memory` auf Option-Byte- oder AAP-Registeradressen.

### Einordnung (Aktivitätsklassen, vgl. §9)

Schritt 1 + 2 sind **Auslesen eigener Gerätezustände** (Debug-Register/Option-
Bytes) — reines Lesen, kein Bypass einer Prüfung. Erst ein tatsächlicher
Flash-Dump (nur bei offenem Debug-Port) wäre „Auslesen eigener Firmwaredaten";
dessen Analyse bleibt Reverse Engineering. Das Umgehen einer gesetzten Sperre
(Mass-Erase, Glitching, RDP-Downgrade) ist eine **andere, destruktive
Kategorie** und hier bewusst ausgeschlossen.
