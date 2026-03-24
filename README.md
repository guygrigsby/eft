# eft

Go library for creating ANSI/NIST-ITL biometric transaction files (EFT). Primary use case: generating ATF eForms-compatible `.eft` files from scanned FD-258 fingerprint cards for Form 1, Form 4, and other NFA submissions.

Built-in WSQ compression, FD-258 card cropping, and ATF-specific defaults.

## Install

```bash
go get github.com/guygrigsby/eft
```

## ATF eForms — Quick Start

Scan your entire FD-258 fingerprint card at 500+ DPI, save as PNG or JPEG, then:

```go
package main

import (
	"log"
	"os"
	"time"

	"github.com/guygrigsby/eft"
)

func main() {
	card, err := os.Open("fd258-scan.png")
	if err != nil {
		log.Fatal(err)
	}
	defer card.Close()

	data, err := eft.CreateATFTransaction(card, eft.ATFSubmissionOptions{
		Person: eft.ATFPersonInfo{
			LastName:     "Doe",
			FirstName:    "John",
			MiddleName:   "Q",
			DOB:          time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			Sex:          "M",
			Race:         "W",
			PlaceOfBirth: "VA",
			Citizenship:  "US",
			Height:       "510",  // 5'10"
			Weight:       "180",
			EyeColor:     "BRO",
			HairColor:    "BLK",
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("fingerprints.eft", data, 0644); err != nil {
		log.Fatal(err)
	}
}
```

Upload `fingerprints.eft` to ATF eForms. The library automatically:
- Crops the FD-258 card into 10 rolled prints + 3 flat/slap impressions
- Compresses all images with WSQ at 0.75 bitrate (FBI standard)
- Builds Type-1 (header), Type-2 (demographic), Type-4 (rolled), and Type-14 (slaps) records
- Sets ATF-specific values (TOT=FAUF, DAI=WVIAFIS0Z, ORI=WVATF0800, RFP=Firearms)
- Validates the output is under ATF's 12 MB limit

## How It Works

The FD-258 card has a standard layout:

```
┌─────────────────────────────────────────────┐
│  Row 1 (rolled): [R.Thumb][R.Idx][R.Mid]   │
│                  [R.Ring][R.Little]          │
│  Row 2 (rolled): [L.Thumb][L.Idx][L.Mid]   │
│                  [L.Ring][L.Little]          │
│  Row 3 (flat):   [L.4 Fingers][Thumbs]     │
│                  [R.4 Fingers]              │
└─────────────────────────────────────────────┘
```

The library uses fixed fractional crop regions matching the full card layout. The demographic header (~28% of the card) is automatically skipped. Scan the entire card at 500 DPI.

## Pre-Cropped Images

If you've already extracted individual finger images, skip the card cropping:

```go
images := &eft.FD258Images{}

// Load individual rolled prints (positions 1-10)
for i, path := range rolledImagePaths {
    f, _ := os.Open(path)
    img, _, _ := image.Decode(f)
    images.Rolled[i] = imageToGray(img) // must be *image.Gray
    f.Close()
}

// Load flat prints
// images.FlatRight = ...  (right four-finger slap)
// images.FlatLeft  = ...  (left four-finger slap)
// images.FlatThumbs = ... (both thumbs)

data, err := eft.CreateATFTransactionFromImages(images, opts)
```

## Generic Transactions

For non-ATF use cases, use the generic API with full control:

```go
f, _ := os.Open("fingerprint.png")
defer f.Close()

data, err := eft.CreateTransaction(
    eft.TransactionOptions{
        TransactionType:   "CAR",
        DestinationAgency: "WVFBI0000",
        OriginatingAgency: "WV1234567",
        ControlNumber:     "TCN20240115001",
        Version:           "0502",
    },
    []eft.FingerprintImage{
        {
            FingerPosition: eft.FingerRightIndex,
            ImpressionType: eft.ImpressionLiveScanPlain,
            Reader:         f,
            Compressor:     &eft.WSQCompressor{Bitrate: 0.75},
        },
    },
)
```

## Low-Level Record API

Build records manually for full control:

```go
type1, _ := eft.NewType1Record(eft.Type1Options{...})
type2 := eft.NewType2Record(eft.Type2Options{IDC: 0, Fields: map[int][]byte{...}})

// Type-4 binary record (rolled prints)
type4, _, _ := eft.NewType4Record(eft.Type4Options{
    IDC:            1,
    ImpressionType: eft.ImpressionNonLiveRolled,
    FingerPosition: eft.FingerRightThumb,
    Image:          grayImg,
    Compressor:     &eft.WSQCompressor{Bitrate: 0.75},
})

// Type-14 tagged record (slaps)
type14, _ := eft.NewType14Record(eft.Type14Options{
    IDC:                  11,
    ImpressionType:       eft.ImpressionNonLivePlain,
    SourceAgency:         "WV1234567",
    CaptureDate:          "20240115",
    HorizontalLineLength: 1500,
    VerticalLineLength:   1000,
    Compression:          eft.CompressionWSQ,
    FingerPosition:       eft.FingerRightFourFingers,
    ImageData:            wsqBytes,
})

txn := &eft.Transaction{}
txn.AddRecord(type1)
txn.AddRecord(type2)
txn.AddRecord(type4)
txn.AddRecord(type14)
data, _ := txn.Encode()
```

## Compression

The `Compressor` interface allows swapping compression implementations:

```go
// WSQ (default) — FBI standard for 500 ppi fingerprints
comp := &eft.WSQCompressor{Bitrate: 0.75}

// Uncompressed raw pixels
comp := &eft.NoneCompressor{}

// Custom compressor (implement eft.Compressor interface)
type MyJP2Compressor struct{}
func (c *MyJP2Compressor) Compress(img *image.Gray) ([]byte, error) { ... }
func (c *MyJP2Compressor) Algorithm() eft.CompressionAlgorithm { return eft.CompressionJP2 }
```

WSQ is provided by [jtejido/go-wsq](https://github.com/jtejido/go-wsq), a pure Go port of the NBIS WSQ codec.

## Finger Positions

| Constant | Value | Description |
|---|---|---|
| `FingerRightThumb` | 1 | Right thumb |
| `FingerRightIndex` | 2 | Right index |
| `FingerRightMiddle` | 3 | Right middle |
| `FingerRightRing` | 4 | Right ring |
| `FingerRightLittle` | 5 | Right little |
| `FingerLeftThumb` | 6 | Left thumb |
| `FingerLeftIndex` | 7 | Left index |
| `FingerLeftMiddle` | 8 | Left middle |
| `FingerLeftRing` | 9 | Left ring |
| `FingerLeftLittle` | 10 | Left little |
| `FingerRightFourFingers` | 13 | Right four-finger slap |
| `FingerLeftFourFingers` | 14 | Left four-finger slap |
| `FingerBothThumbs` | 15 | Both thumbs |

## ATF-Specific Values

These are hardcoded in `CreateATFTransaction`:

| Field | Value |
|---|---|
| 1.002 VER | `0200` |
| 1.004 TOT | `FAUF` (Federal Applicant User Fee) |
| 1.007 DAI | `WVIAFIS0Z` |
| 1.008 ORI | `WVATF0800` |
| 1.011 NSR | `19.69` (500 ppi) |
| 2.037 RFP | `Firearms` |
| 2.073 CRI | `WVATF0800` |
| Compression | WSQ at 0.75 bitrate |
| Max file size | 12 MB |

## Example / Test Files

Public-domain ANSI/NIST-ITL sample files for testing:

- **NBIS test data** — AN2K test files and WSQ images from [NIST NIGOS](https://www.nist.gov/image-group/nigos). Public domain.
- **NIST Standard Reference Files** — Traditional encoding reference transactions from [ANSI/NIST-ITL Standard References](https://www.nist.gov/itl/iad/image-group/ansinist-itl-standard-references). Public domain.
- **bentedesco/eft-fingerprint-viewer** — `.eft` files for the FD-258 format at [github.com/bentedesco/eft-fingerprint-viewer](https://github.com/bentedesco/eft-fingerprint-viewer). MIT license.

## Specification References

- [EBTS v11.1](https://www.fbibiospecs.cjis.gov/EBTS/Approved) — FBI Electronic Biometric Transmission Specification (included in this repo as PDF)
- [NIST SP 500-290](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.500-290e3.pdf) — ANSI/NIST-ITL 1-2011 Update 2015
- [WSQ Specification v3.1](http://www.fbibiospecs.cjis.gov/Document/Get?fileName=WSQ_Gray-scale_Specification_Version_3_1_Final.pdf) — Wavelet Scalar Quantization compression
- [NIST NBIS Software](https://www.nist.gov/services-resources/software/nist-biometric-image-software-nbis) — Reference C implementation

## Limitations

- **FD-258 crop regions are approximate.** The fixed layout assumes a clean, straight scan of the entire card. Cards with skew, poor alignment, or non-standard printing may need manual crop region adjustment via a custom `FD258Layout`.
- **No JPEG 2000 encoder.** JP2 compression for 1000 ppi images requires a custom `Compressor` implementation.
- **Traditional encoding only.** NIEM-XML encoding is not supported.
- **No parser/decoder.** This library creates EFT files; it does not read them.
- **WSQ library has no stated license.** The underlying [go-wsq](https://github.com/jtejido/go-wsq) is a port of public-domain NBIS code, but the port itself lacks an explicit license. The `Compressor` interface allows swapping in a different WSQ implementation if needed.
