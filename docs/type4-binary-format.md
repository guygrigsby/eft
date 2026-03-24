# Type-4 Binary Record Format

## Overview

Type-4 records store grayscale fingerprint images using a fixed binary header (no tagged fields). They are used for rolled fingerprint impressions at 500 ppi.

## Binary Header Layout (18 bytes)

| Offset | Size | Field | Description |
|--------|------|-------|-------------|
| 0–3 | 4 bytes | LEN | Total record length (big-endian uint32) |
| 4 | 1 byte | IDC | Information Designation Character (1–99) |
| 5 | 1 byte | IMP | Impression type |
| 6–11 | 6 bytes | FGP | Finger position (byte 0 = position, bytes 1–5 = 0xFF padding) |
| 12 | 1 byte | ISR | Scanning resolution (0 = 500 ppi native) |
| 13–14 | 2 bytes | HLL | Horizontal line length (big-endian uint16) |
| 15–16 | 2 bytes | VLL | Vertical line length (big-endian uint16) |
| 17 | 1 byte | GCA | Compression algorithm (0=none, 1=WSQ, 2=JPEGB, 3=JPEGL, 4=JP2, 5=JP2L) |
| 18+ | variable | DATA | Compressed image data |

## Implementation Notes

- `NewType4Record()` returns both a `*Record` (for use with `Transaction.AddRecord()`) and the raw `[]byte` (for direct use).
- The `Record.rawBinary` field stores the complete binary blob. When `Transaction.Encode()` encounters a record with `rawBinary` set, it emits the bytes directly instead of building tagged fields.
- LEN includes the 18-byte header: `totalLen = 18 + len(imageData)`.
- ISR is always 0 (500 ppi native resolution). Higher resolutions require Type-14 records.

## When to Use Type-4 vs Type-14

| Feature | Type-4 | Type-14 |
|---------|--------|---------|
| Encoding | Binary (fixed header) | Tagged fields |
| Resolution | Fixed 500 ppi | Variable (any ppi) |
| Typical use | Rolled prints | Flat/slap prints |
| ATF usage | IDC 1–10 (rolled) | IDC 11–13 (slaps) |

## References

- EBTS v11.1 §5.4 (Type-4 Record)
- NIST SP 500-290 §8.4
