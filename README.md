# eft

Go library and CLI for creating ANSI/NIST-ITL biometric transaction files (EFT). Primary use case: generating ATF eForms-compatible `.eft` files from scanned FD-258 fingerprint cards for Form 1, Form 4, and other NFA submissions.

Built-in WSQ compression, FD-258 card cropping, OCR demographic extraction, and ATF-specific defaults.

> **Disclaimer:** This software generates EFT files based on the ANSI/NIST-ITL specification and community-documented ATF constants. Generated files have **not** been independently verified with ATF eForms. The ATF does not publish an official EFT upload specification — the constants used here (TOT, DAI, ORI, etc.) come from community reverse-engineering. Always review your submission carefully before uploading. Use at your own risk.

## Web App

**No installation required.** Use the web app at [guygrigsby.github.io/eft](https://guygrigsby.github.io/eft) — everything runs in your browser via WebAssembly. No data is sent to any server.

1. Upload your scanned FD-258 card (PNG or JPEG, 500+ DPI recommended)
2. Verify the fingerprint crops look correct
3. Fill in demographics manually or click "Try OCR" to auto-fill from the card header
4. Click Generate to download your `.eft` file

The WASM binary is ~4.5 MB (compressed ~2 MB on download). WSQ compression runs in-browser.

## Install

**Library:**
```bash
go get github.com/guygrigsby/eft/pkg/eft
```

**CLI:**
```bash
go install github.com/guygrigsby/eft/cmd/eft@latest

# Or build locally:
make build    # produces ./eft binary
```

**OCR support** (optional — needed for `--ocr` flag):
```bash
brew install tesseract        # macOS
sudo apt install tesseract-ocr  # Ubuntu/Debian
```

## Quick Start

Scan your FD-258 fingerprint card at 500+ DPI, save as PNG or JPEG, then:

```bash
# Easiest: OCR reads demographics from the card automatically
eft atf --ocr -o submission.eft card_scan.png

# Or provide demographics manually
eft atf --last-name Doe --first-name John --dob 1990-01-01 \
  --sex M --race W -o submission.eft card_scan.png
```

Upload `submission.eft` to ATF eForms.

## CLI

### `eft atf` — ATF from card scan

Creates an ATF eForms EFT from a scanned FD-258 card. Automatically crops the card into individual prints and sets all ATF constants.

**With OCR** — reads name, DOB, sex, race, height, weight, eye/hair color, and more directly from the card header using [tesseract](https://github.com/tesseract-ocr/tesseract):

```bash
# Demographics extracted automatically:
eft atf --ocr -o submission.eft card_scan.png

# OCR with corrections (flags override OCR results):
eft atf --ocr --first-name Jon -o submission.eft card_scan.png
```

**Manual entry** — provide all demographics via flags:

```bash
eft atf --last-name Smith --first-name Jane --middle-name Q \
  --dob 1985-06-15 --sex F --race W --ssn 123456789 \
  --pob VA --citizenship US --height 507 --weight 130 \
  --eye-color BRO --hair-color BLK --compression none \
  -o submission.eft fd258_scan.png
```

### `eft atf-images` — ATF from pre-cropped images

Creates an ATF EFT from individual fingerprint images already extracted from the card.

```bash
eft atf-images --last-name Doe --first-name John --dob 1990-01-01 \
  --sex M --race W \
  --rolled-1 thumb_r.png --rolled-2 index_r.png \
  --rolled-6 thumb_l.png --rolled-7 index_l.png \
  --flat-right right4.png --flat-left left4.png \
  --flat-thumbs thumbs.png -o submission.eft
```

### `eft create` — Generic transaction

Creates a generic ANSI/NIST-ITL transaction with full control over record types and fields.

```bash
eft create -t CAR --dai WVFBI0000 --ori WV1234567 --tcn TCN001 \
  --finger 1:thumb.png --finger 2:index.png -o output.eft
```

The `--finger` flag format is `position:file` where position is the finger number (1-15). Use `--impression` to set the impression type (0=livescan plain, 1=livescan rolled, 2=nonlive plain, 3=nonlive rolled).

### `eft crop` — Extract fingerprints from card

Extracts individual fingerprint images from a scanned card without building an EFT.

```bash
eft crop -o ./prints card_scan.png
```

Outputs 13 PNG files: `rolled_01_right_thumb.png` through `rolled_10_left_little.png`, plus `flat_right_four.png`, `flat_left_four.png`, and `flat_both_thumbs.png`.

### Common flags

| Flag | Commands | Description |
|------|----------|-------------|
| `-o, --output` | create, atf, atf-images | Output EFT file path |
| `-o, --output-dir` | crop | Output directory for cropped PNGs |
| `-c, --compression` | create, atf, atf-images | `wsq` (default) or `none` |
| `--date` | create, atf, atf-images | Transaction date `YYYY-MM-DD` (default today) |
| `--tcn` | create, atf, atf-images | Transaction control number |
| `--ocr` | atf | Extract demographics from card header via tesseract |

## Library

Import path: `github.com/guygrigsby/eft/pkg/eft`

### ATF eForms

Scan your entire FD-258 fingerprint card at 500+ DPI, save as PNG or JPEG, then:

```go
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

os.WriteFile("fingerprints.eft", data, 0644)
```

The library automatically:
- Crops the FD-258 card into 10 rolled prints
- Compresses all images with WSQ at 0.75 bitrate (FBI standard)
- Builds Type-1 (header), Type-2 (demographic), and Type-4 (rolled) records
- Sets ATF-specific values (TOT=FAUF, DAI=WVIAFIS0Z, ORI=WVATF0800, VER=0200, RFP=Firearms)
- Validates the output is under ATF's 12 MB limit

> **Note:** ATF eForms FAUF transactions accept only Type-4 rolled prints. Slap/flat prints (Type-14) are not included — mixing Type-4 and Type-14 causes "mutually exclusive records" validation errors.

### OCR Demographic Extraction

Extract demographic data directly from the FD-258 card header instead of typing it manually. Implement the `OCRProvider` interface with any backend — the library handles cropping each field region, running OCR, and normalizing the results.

```go
type OCRProvider interface {
    RecognizeText(ctx context.Context, img image.Image) (string, error)
}
```

```go
card, _ := os.Open("fd258-scan.png")
cardImg, _, _ := image.Decode(card)

result, err := eft.ExtractDemographics(
    ctx, cardImg, myOCRProvider, eft.DefaultFD258HeaderFields(),
)
// result.Person   — parsed ATFPersonInfo
// result.Warnings — fields that couldn't be read
// result.RawFields — raw OCR text per field (for debugging)

// Merge manual corrections on top of OCR results:
person := eft.MergeDemographics(result.Person, manualOverrides)
```

The normalization pipeline handles common OCR variations:

| Field | Input examples | Output |
|-------|---------------|--------|
| Name | `"Doe, John Michael"`, `"DOE JOHN MICHAEL"` | Structured last/first/middle |
| DOB | `"01/15/1990"`, `"1990-01-15"`, `"01152000"` | `time.Time` |
| Sex | `"M"`, `"MALE"` | `"M"` |
| Race | `"W"`, `"CAUCASIAN"` | `"W"` |
| Height | `"5'10\""`, `"5-10"`, `"5 10"` | `"510"` |
| Weight | `"180 lbs"` | `"180"` |
| Eye/Hair | `"BROWN"`, `"BRN"` | `"BRO"` |
| SSN | `"123-45-6789"` | `"123456789"` |
| Citizenship | `"USA"`, `"United States"` | `"US"` |

Header field crop regions (`FD258HeaderFields`) use the same fractional coordinate system as fingerprint regions and can be overridden for non-standard cards.

### Pre-Cropped Images

If you've already extracted individual finger images, skip the card cropping:

```go
images := &eft.FD258Images{}

for i, path := range rolledImagePaths {
    f, _ := os.Open(path)
    img, _, _ := image.Decode(f)
    images.Rolled[i] = imageToGray(img) // must be *image.Gray
    f.Close()
}
// images.FlatRight, images.FlatLeft, images.FlatThumbs = ...

data, err := eft.CreateATFTransactionFromImages(images, opts)
```

### Generic Transactions

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

### Low-Level Record API

Build records manually for full control:

```go
type1, _ := eft.NewType1Record(eft.Type1Options{...})
type2 := eft.NewType2Record(eft.Type2Options{IDC: 0, Fields: map[int][]byte{...}})

type4, _, _ := eft.NewType4Record(eft.Type4Options{
    IDC:            1,
    ImpressionType: eft.ImpressionNonLiveRolled,
    FingerPosition: eft.FingerRightThumb,
    Image:          grayImg,
    Compressor:     &eft.WSQCompressor{Bitrate: 0.75},
})

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

## How It Works

The FD-258 card is 8" x 8" with a standard layout:

```
┌─────────────────────────────────────────────┐
│  Header (~28%): [Name, DOB, Sex, Race, ...]│  ← OCR extracts demographics
├─────────────────────────────────────────────┤
│  Row 1 (rolled): [R.Thumb][R.Idx][R.Mid]  │
│                   [R.Ring][R.Little]         │
│  Row 2 (rolled): [L.Thumb][L.Idx][L.Mid]  │
│                   [L.Ring][L.Little]         │
│  Row 3 (flat):   [L.4 Fingers][Thumbs]    │
│                   [R.4 Fingers]             │
└─────────────────────────────────────────────┘
```

The library uses fractional crop regions (0.0–1.0) to locate each area on the card, making the layout resolution-independent. Scan the entire card at 500 DPI — the library handles the rest.

## Reference

### Compression

The `Compressor` interface allows swapping compression implementations:

```go
comp := &eft.WSQCompressor{Bitrate: 0.75}  // WSQ (default) — FBI standard for 500 ppi
comp := &eft.NoneCompressor{}               // Uncompressed raw pixels

// Custom compressor (implement eft.Compressor interface)
type MyJP2Compressor struct{}
func (c *MyJP2Compressor) Compress(img *image.Gray) ([]byte, error) { ... }
func (c *MyJP2Compressor) Algorithm() eft.CompressionAlgorithm { return eft.CompressionJP2 }
```

WSQ is provided by [jtejido/go-wsq](https://github.com/jtejido/go-wsq), a pure Go port of the NBIS WSQ codec.

### Finger Positions

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

### ATF-Specific Values

These are hardcoded in `CreateATFTransaction`:

| Field | Value |
|---|---|
| 1.002 VER | `0200` (ANSI/NIST-ITL 1-2000 / EFTS 7.1) |
| 1.004 TOT | `FAUF` (Federal Applicant User Fee) |
| 1.007 DAI | `WVIAFIS0Z` |
| 1.008 ORI | `WVATF0800` |
| 1.011 NSR | `19.69` (500 ppi) |
| 1.012 NTR | `19.69` (500 ppi) |
| 1.013 DOM | `NORAM` / `8.1` |
| 2.037 RFP | `Firearms` |
| 2.073 CRI | `WVATF0800` |
| Records | Type-4 only (10 rolled prints, no Type-14 slaps) |
| Compression | WSQ at 0.75 bitrate |
| Max file size | 12 MB |

## Example / Test Files

Public-domain ANSI/NIST-ITL sample files for testing:

- **NBIS test data** — AN2K test files and WSQ images from [NIST NIGOS](https://www.nist.gov/image-group/nigos). Public domain.
- **NIST Standard Reference Files** — Traditional encoding reference transactions from [ANSI/NIST-ITL Standard References](https://www.nist.gov/itl/iad/image-group/ansinist-itl-standard-references). Public domain.
- **bentedesco/eft-fingerprint-viewer** — `.eft` files for the FD-258 format at [github.com/bentedesco/eft-fingerprint-viewer](https://github.com/bentedesco/eft-fingerprint-viewer). MIT license.

## Specification References

- [EBTS v11.3](https://fbibiospecs.fbi.gov/file-repository/ebts) — FBI Electronic Biometric Transmission Specification (included in this repo as PDF). New versions are published at this URL.
- [NIST SP 500-290](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.500-290e3.pdf) — ANSI/NIST-ITL 1-2011 Update 2015
- [WSQ Specification v3.1](http://www.fbibiospecs.cjis.gov/Document/Get?fileName=WSQ_Gray-scale_Specification_Version_3_1_Final.pdf) — Wavelet Scalar Quantization compression
- [NIST NBIS Software](https://www.nist.gov/services-resources/software/nist-biometric-image-software-nbis) — Reference C implementation

## Limitations

- **FD-258 crop regions are approximate.** The fixed layout assumes a clean, straight scan of the entire card. Cards with skew, poor alignment, or non-standard printing may need manual crop region adjustment via a custom `FD258Layout` or `FD258HeaderFields`.
- **OCR accuracy depends on scan quality.** The `DefaultFD258HeaderFields()` coordinates are calibrated for the standard FBI FD-258 card but may need tuning for specific printings or scan alignments. Handwritten fields are harder to OCR than machine-printed ones — always review OCR results and use flag overrides to correct mistakes.
- **No JPEG 2000 encoder.** JP2 compression for 1000 ppi images requires a custom `Compressor` implementation.
- **Traditional encoding only.** NIEM-XML encoding is not supported.
- **No parser/decoder.** This library creates EFT files; it does not read them.
- **WSQ library has no stated license.** The underlying [go-wsq](https://github.com/jtejido/go-wsq) is a port of public-domain NBIS code, but the port itself lacks an explicit license. The `Compressor` interface allows swapping in a different WSQ implementation if needed.
