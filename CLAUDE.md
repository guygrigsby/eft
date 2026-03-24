# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Go library for creating ANSI/NIST-ITL (EFT/EBTS) biometric transaction files. Primary use case: generating ATF eForms-compatible `.eft` files from scanned FD-258 fingerprint cards for Form 1/Form 4 NFA submissions.

## Commands

```bash
go build ./...        # Build all packages
go test ./...         # Run all tests
go test ./... -v      # Verbose test output
go test -run TestFoo  # Run a single test
```

## Architecture

### Encoding layer
- `nist.go` — Core types (`Field`, `Record`, `Transaction`). Tagged field encoding with auto-computed LEN/CNT. Binary records use `rawBinary` field to bypass tagged encoding. → [docs/encoding.md](docs/encoding.md)

### Compression
- `compress.go` — `Compressor` interface with `WSQCompressor` (default, 0.75 bitrate) and `NoneCompressor`. WSQ fails on uniform images; real fingerprints work fine. → [docs/wsq-compression.md](docs/wsq-compression.md)

### Record types
- `type1.go` — Type-1 header. VER defaults `0502`; ATF overrides to `0200`.
- `type2.go` — Type-2 demographics. Arbitrary fields via `map[int][]byte`.
- `type4.go` — Type-4 binary fingerprint (18-byte header + image). Rolled prints. → [docs/type4-binary-format.md](docs/type4-binary-format.md)
- `type14.go` — Type-14 tagged fingerprint. Variable resolution. Slap/flat prints.

### High-level APIs
- `eft.go` — `CreateTransaction()`: generic API, decodes PNG/JPEG, builds Type-14 records.
- `atf.go` — `CreateATFTransaction()`: ATF-specific, crops FD-258 card, builds Type-4 + Type-14 with hardcoded ATF values. → [docs/atf-eforms.md](docs/atf-eforms.md)
- `fd258.go` — FD-258 card layout with fractional crop regions. `CropFD258()` extracts 13 prints. → [docs/fd258-layout.md](docs/fd258-layout.md)

## ATF Constants

| Field | Value | Note |
|-------|-------|------|
| TOT | `FAUF` | Federal Applicant User Fee |
| DAI | `WVIAFIS0Z` | Destination agency |
| ORI | `WVATF0800` | Originating agency |
| VER | `0200` | ANSI/NIST-ITL 1-2000 |
| RFP | `Firearms` | Reason fingerprinted |
| Max size | 12 MB | ATF upload limit |
| WSQ | 0.75 bitrate | FBI standard for 500 ppi |
| Name format | `Last,First Middle` | Type-2 field 2.018 |

## Dependencies

- `github.com/jtejido/go-wsq` v0.0.3-beta — Pure Go WSQ codec (port of NBIS). No explicit license.

## Testing Notes

- Test images use random noise (`rand.NewSource(42)`) because WSQ fails on smooth/uniform content.
- ATF integration test uses `NoneCompressor` to avoid WSQ overhead; card size kept ≤1000×1000 to stay under 12MB.

## Key Specifications

- EBTS v11.1 (in repo: `EBTS v11.1_Final_508.pdf`)
- NIST SP 500-290 (ANSI/NIST-ITL 1-2011)
- WSQ Specification v3.1

Full source list and design rationale: → [docs/sources.md](docs/sources.md)

## Detailed Documentation

| Topic | File |
|-------|------|
| ANSI/NIST-ITL encoding (separators, LEN, binary vs tagged) | [docs/encoding.md](docs/encoding.md) |
| ATF eForms requirements (constants, Type-2 fields, community sources) | [docs/atf-eforms.md](docs/atf-eforms.md) |
| FD-258 card crop regions (fractional coordinates, layout) | [docs/fd258-layout.md](docs/fd258-layout.md) |
| WSQ compression (go-wsq, Compressor interface, limitations) | [docs/wsq-compression.md](docs/wsq-compression.md) |
| Type-4 binary format (18-byte header, when to use vs Type-14) | [docs/type4-binary-format.md](docs/type4-binary-format.md) |
| Sources and references (specs, software, ATF research, test data) | [docs/sources.md](docs/sources.md) |
