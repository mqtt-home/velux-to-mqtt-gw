# HEADER-ANALYSIS — KLF200 Container-Header (erste 8 KiB)

Byteweise und strukturelle Analyse der ersten 8 KiB (`0x0000`–`0x1FFF`) des
Images `ID10124554_149884-01_KLF200-v2.0.0.71.bin`.

- **Sample:** 4 072 361 Bytes (`0x3E23A9`), SHA256
  `364c3b734fe14e4f91f3a079321635f24c358d2f87e217b66f359baecc26e49d`
  (identisch mit der in [`out/triage.json`](out/triage.json) aufgezeichneten).
- **Reproduzierbar:** `python3 header_probe.py samples/<image>.bin`
  (stdlib only). Dieses Dokument enthält **keine Payload-Bytes** — nur die
  Container-Rahmenstruktur (Magic/Tag/Metadaten-Tabelle) und abgeleitete Werte.
- Die Aussagen stammen aus einem **frischen Dump** (nicht nur aus der alten
  Triage) und **korrigieren** deren Header-Schätzung (s. §5).

---

## Befund (Kurzfassung)

1. **Der Header ist 0x1A9 (425) Bytes lang, nicht 8192.** Die alte Triage
   nahm ~8 KiB an (reiner Scan-Limit-Default); der tatsächliche Header endet
   bei `0x01A8`, danach beginnt bei `0x01A9` die hochentropische Payload.
2. **Der Container ist durch eine CRC32 (nicht-kryptografisch) integritäts-
   geschützt** — ein 4-Byte-Trailer am Dateiende. **Belegt** (exakter Treffer).
3. **Ein Längenfeld im Header beziffert die Payload exakt** — `0x3E21FC` bei
   `0x011D`. **Belegt** (Header + Payload + CRC = Dateigröße, auf das Byte).
4. **Kein Signaturblock auf Container-Ebene.** Das einzige den Gesamt-Container
   abdeckende Prüffeld ist die CRC32. Vor dem CRC-Trailer steht Klartext
   (UI-JavaScript), kein RSA/ECDSA/PKCS#7-Block. → Hypothese „Image signiert"
   auf **Container-Ebene: unwahrscheinlich**; gesamthaft: **unklar** (eine
   Signatur könnte *innerhalb* der verschlüsselten MCU-Payload liegen, die der
   Header nicht sichtbar macht — s. §7).

---

## Offset-Tabelle (Container-Rahmen)

| Offset | Länge | Bytes / Interpretation | Hypothese | Sicherheit |
|---|---:|---|---|---|
| `0x0000` | 2 | `00 01` | Format-/Versions-Marker vor dem Magic (BE `0x0001`) | mittel |
| `0x0002` | 14 | ASCII `VELUX FIRMWARE` | Container-Magic | **belegt** |
| `0x0010` | 10 | ASCII `KLF200 TLS` | Produkt-/Ziel-Tag | **belegt** |
| `0x001A` | 246 | `00…00` | Null-Padding (Zero-Run `0x1A`–`0x111`) | **belegt** |
| `0x0110` | 2 | `00 00` | Padding / Tabellen-Vorlauf | mittel |
| `0x0112` | ~11 | `01 88 00 00 00 21 …` | Tabellen-Kopf (Typ/Count-Felder) | niedrig |
| `0x011D` | 3 | `3E 21 FC` (BE = 4 071 932) | **Payload-Länge** = Datei − Header(0x1A9) − CRC(4) | **belegt** |
| `0x0120`–`0x01A8` | ~137 | Records mit absteigenden IDs `0x47,46,45,44,43,3E,3D,3C,2D,2C,29,…,1B`, je `…ID 00 00 0n` | **Segment-/Regionen-Tabelle** (ID + Typ/Count) | mittel |
| `0x01A9` | — | Entropie springt auf ~7.2 | **Beginn der Payload** (verschlüsseltes/gepacktes EFM32-Image, „Region 0") | hoch |
| `len−4` | 4 | `CA DE 25 65` (BE) | **CRC32-Trailer** über Bytes `[0 … len−5]` | **belegt** |

### Entropie der ersten 8 KiB (256-B-Fenster)
`0x0000`=0.86 (Rahmen) → `0x0100`=4.40 (Tabelle) → `0x0200`=6.26 → `0x0300`=6.99
→ ab `0x0400` konstant **~7.1–7.25** bis `0x1FFF`. Der Anstieg markiert den
Übergang Rahmen → Payload bei ≈`0x01A9`.

---

## Belege

### CRC32-Trailer (bestätigt)
```
crc32(blob[:-4])            = 0xCADE2565      (zlib/IEEE, poly 0x04C11DB7)
letzte 4 Bytes, big-endian  = 0xCADE2565      → EXAKTER TREFFER
letzte 4 Bytes, little-endian = 0x6525DECA     → kein Treffer
```
Der Container endet also mit einer **CRC32 im Big-Endian-Format** über die
gesamte Datei außer den letzten 4 Bytes. CRC32 ist ein reiner
**Fehlererkennungs-Code, kein kryptografischer Nachweis** — nach jeder Änderung
trivial neu berechenbar, liefert *keine* Authentizität.

### Payload-Längenfeld (bestätigt, selbstkonsistent)
```
Feld @0x011D (3 B BE)       = 0x3E21FC = 4 071 932
Header (0x00…0x1A8)         = 0x0001A9 = 425
CRC-Trailer                 = 4
Summe                       = 0x1A9 + 0x3E21FC + 4 = 0x3E23A9 = Dateigröße ✓
```
Header-Länge **und** CRC-Abdeckung ergeben unabhängig voneinander denselben
Payload-Start `0x01A9` — die Struktur ist damit auf das Byte geschlossen.

### CRC/Checksum im Header-Vorlauf (negativ, aussagekräftig)
Kein 16-/32-Bit-Feld in `0x0000`–`0x003F` entspricht einer CRC16/CRC32
(CCITT, MODBUS, IEEE) über Datei/Header/Payload. Das ist erwartungskonform:
Die ersten 64 Byte sind nach dem Tag Null — das Prüffeld sitzt am **Dateiende**,
nicht im Kopf. Nicht-Treffer schließt ein führendes Checksum-Feld aus.

### Kein Signaturblock (negativ)
- Die 512 Byte vor dem CRC-Trailer sind **Klartext** (AngularJS-UI,
  `ng-show=…`, `ctrl.next()`), Entropie ~5.0 — **kein** hochentropischer
  256/512-B-Block, wie ihn eine RSA-2048/4096- oder ECDSA-Signatur zeigen würde.
- Kein ASN.1/DER-Präfix (`30 82 …`), kein PEM-Marker, keine PKCS#7-Struktur im
  Header oder Trailer.

---

## Wahrscheinlichste Header-Struktur

```
[0x0000] 00 01                         Format-/Versions-Marker
[0x0002] "VELUX FIRMWARE"              Magic (14 B)
[0x0010] "KLF200 TLS"                  Produkt-Tag (10 B)
[0x001A] 00 … 00                       Null-Padding (246 B)
[0x0110] <Tabellen-Kopf>               u.a. Payload-Länge 0x3E21FC @0x011D
[0x0120] <Segment-/Regionen-Tabelle>   absteigende IDs 0x47…0x1B + Typ/Count
[0x01A9] <Payload>                     verschl./gepacktes EFM32-Image (Region 0)
   …
[len-4]  CA DE 25 65                   CRC32 (BE) über [0 … len-5]
```

→ Form: `[Header] [Metadaten-Tabelle] [Payload] [CRC32]`.
**Kein** eigener Signatur-/Zertifikatsabschnitt auf Container-Ebene.

---

## Bewertung der Hypothese „Das Firmware-Image ist signiert"

**Container-Ebene: unwahrscheinlich.** Das einzige den Gesamt-Container
abdeckende Prüffeld ist eine CRC32 — Integrität, nicht Authentizität. Wäre der
Container signiert, läge zusätzlich zur CRC ein Signaturblock vor; es findet
sich keiner.

**Gesamthaft: unklar.** Die Payload (`0x01A9…`, „Region 0") ist
verschlüsselt/gepackt (Entropie ~7.2–7.94). Eine **Signatur könnte innerhalb
des entschlüsselten MCU-Images liegen** und erst vom Bootloader nach der
Entschlüsselung geprüft werden — das ist aus Header/Container prinzipiell nicht
sichtbar. Diese Frage bleibt offen (deckt sich mit
[`TRUST-CHAIN.md`](TRUST-CHAIN.md) §1/§3).

---

## Nächster sinnvoller Analyse-Schritt

1. **Segment-Tabelle `0x0120`–`0x01A8` dekodieren:** die absteigenden IDs
   (`0x47…0x1B`) + Typ/Count-Felder gegen die 14 Entropie-Regionen aus
   `out/triage.txt` mappen → prüfen, ob die Tabelle Offsets/Längen der späteren
   Klartext-Regionen (UI, WiFi-Treiber) liefert. Entschlüsselt dann die
   Container-Geometrie vollständig.
2. **Region 0 (`0x01A9…0x117000`) klassifizieren:** AES-verschlüsselt vs. nur
   gepackt — über Block-Wiederholungen (ECB-Signatur) und Byte-Histogramm-
   Flachheit. Erst damit ist die Signatur-Frage *innerhalb* der Payload
   angreifbar.
3. **Kleine opake Blobs (Region 6/12, `0x2AA000`/`0x2EB000`, Entropie ~7.8)**
   als Signatur-/Schlüssel-Kandidaten gezielt dumpen und auf ASN.1/feste Länge
   (256/384/512 B) prüfen.
