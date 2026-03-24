# ATF eForms Requirements

## Overview

ATF eForms accepts `.eft` files for NFA submissions (Form 1, Form 4, etc.). The format requirements are not formally documented by ATF — they are reverse-engineered from community tools and ATF validation feedback.

## Hardcoded Constants

These values are set by `CreateATFTransaction()` and must not be changed for ATF submissions:

| Field   | Value        | Description                          |
|---------|-------------|--------------------------------------|
| 1.002 VER | `0200`    | ANSI/NIST-ITL 1-2000 (not 2011)    |
| 1.004 TOT | `FAUF`    | Federal Applicant User Fee          |
| 1.007 DAI | `WVIAFIS0Z` | Destination agency identifier     |
| 1.008 ORI | `WVATF0800` | ATF originating agency            |
| 1.011 NSR | `19.69`   | 500 ppi in pixels/mm               |
| 1.012 NTR | `19.69`   | Same as NSR                        |
| 2.037 RFP | `Firearms` | Reason fingerprinted              |
| 2.073 CRI | `WVATF0800` | Controlling agency (same as ORI) |
| Compression | WSQ 0.75 | FBI standard for 500 ppi          |
| Max size | 12 MB       | ATF upload limit                   |

## Version: 0200 vs 0502

ATF eForms uses ANSI/NIST-ITL 1-2000 (VER `0200`), not the newer 2011 standard (VER `0502`). The generic `CreateTransaction()` defaults to `0502` — the ATF path overrides this.

## Type-2 Demographic Fields

Field mapping in `buildATFType2Fields()`:

| Field | Tag | Content | Required |
|-------|-----|---------|----------|
| 2.005 | RET | `N` (new submission) | Yes |
| 2.016 | SOC | SSN (9 digits, no dashes) | No |
| 2.018 | NAM | `Last,First Middle` | Yes |
| 2.020 | POB | State/country code | No |
| 2.021 | CTZ | Citizenship code | No |
| 2.022 | DOB | `YYYYMMDD` | Yes |
| 2.024 | SEX | `M`, `F`, or `X` | No |
| 2.025 | RAC | Race code | No |
| 2.027 | HGT | Height `FII` (e.g., `510`) | No |
| 2.029 | WGT | Weight in pounds | No |
| 2.031 | EYE | 3-letter eye color | No |
| 2.032 | HAI | 3-letter hair color | No |
| 2.037 | RFP | `Firearms` (hardcoded) | Yes |
| 2.038 | DPR | Date printed `YYYYMMDD` | Yes |
| 2.041 | ADR | Address | No |
| 2.073 | CRI | `WVATF0800` (hardcoded) | Yes |

**Name format**: `Last,First Middle` — comma between last and first, space before middle. No comma before middle name.

## Transaction Structure

An ATF submission contains:
1. **Type-1** — Transaction header (1 record)
2. **Type-2** — Demographics (1 record, IDC 0)
3. **Type-4** — Rolled prints (up to 10 records, IDC 1-10)
4. **Type-14** — Flat/slap prints (up to 3 records, IDC 11-13)

## Community Sources

ATF publishes no official EFT specification. These constants were sourced from:
- [EFTSuite](https://www.eftsuite.com/) — Commercial EFT tool, documentation references
- [OpenEFT](https://github.com/search?q=openeft) — Open-source EFT implementations
- ATF eForms validation error messages (trial and error)
- NFA community forums and guides
