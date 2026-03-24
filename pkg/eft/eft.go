package eft

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"time"
)

// FingerprintImage describes a single fingerprint image to include
// in an EFT transaction.
type FingerprintImage struct {
	// FingerPosition identifies which finger.
	FingerPosition FingerPosition
	// ImpressionType indicates how the print was captured.
	ImpressionType ImpressionType
	// Reader provides the image data. Supported formats: PNG, JPEG.
	// The image is converted to 8-bit grayscale for the EFT record.
	Reader io.Reader
	// Compressor performs image compression. If nil, DefaultCompressor() (WSQ
	// at 0.75 bitrate) is used. Set to &NoneCompressor{} for uncompressed.
	Compressor Compressor
	// PixelsPerInch overrides the resolution. Default 500.
	PixelsPerInch int
}

// TransactionOptions configures the EFT transaction header.
type TransactionOptions struct {
	// TransactionType is the TOT code (e.g., "CAR", "CNA", "FAUF").
	TransactionType string
	// DestinationAgency is the 9-byte ORI of the receiving agency.
	DestinationAgency string
	// OriginatingAgency is the 9-byte ORI of the sending agency.
	OriginatingAgency string
	// ControlNumber is a unique transaction identifier (10-40 bytes).
	ControlNumber string
	// DomainName (e.g., "NORAM").
	DomainName string
	// DomainVersion (e.g., "11.1").
	DomainVersion string
	// Version overrides the ANSI/NIST-ITL version (1.002 VER).
	// Default "0502". Set to "0200" for older EFTS compatibility.
	Version string
	// Date overrides the transaction date. Defaults to now.
	Date time.Time
	// DemographicFields sets Type-2 fields. See Type2Options.Fields.
	DemographicFields map[int][]byte
}

// CreateTransaction builds an ANSI/NIST-ITL transaction from one or more
// fingerprint images and returns the encoded bytes ready to write to a file.
// Images are compressed using each FingerprintImage's Compressor (default WSQ).
func CreateTransaction(opts TransactionOptions, images []FingerprintImage) ([]byte, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("eft: at least one fingerprint image is required")
	}

	// Build Type-1 record.
	type1, err := NewType1Record(Type1Options{
		TransactionType:   opts.TransactionType,
		DestinationAgency: opts.DestinationAgency,
		OriginatingAgency: opts.OriginatingAgency,
		ControlNumber:     opts.ControlNumber,
		DomainName:        opts.DomainName,
		DomainVersion:     opts.DomainVersion,
		Date:              opts.Date,
	})
	if err != nil {
		return nil, err
	}

	// Override version if specified.
	if opts.Version != "" {
		type1.SetField(2, []byte(opts.Version))
	}

	// Build Type-2 record.
	type2 := NewType2Record(Type2Options{
		IDC:    0,
		Fields: opts.DemographicFields,
	})

	txn := &Transaction{}
	txn.AddRecord(type1)
	txn.AddRecord(type2)

	// Build Type-14 records for each fingerprint image.
	for i, img := range images {
		idc := i + 1

		if img.Reader == nil {
			return nil, fmt.Errorf("eft: FingerprintImage[%d]: Reader is required", i)
		}

		// Decode the image.
		decoded, _, err := image.Decode(img.Reader)
		if err != nil {
			return nil, fmt.Errorf("eft: FingerprintImage[%d]: decoding image: %w", i, err)
		}

		gray := imageToGray(decoded)
		bounds := gray.Bounds()

		comp := img.Compressor
		if comp == nil {
			comp = DefaultCompressor()
		}

		imgData, err := comp.Compress(gray)
		if err != nil {
			return nil, fmt.Errorf("eft: FingerprintImage[%d]: compressing: %w", i, err)
		}

		ppi := img.PixelsPerInch
		if ppi == 0 {
			ppi = 500
		}

		captureDate := time.Now().Format("20060102")
		if !opts.Date.IsZero() {
			captureDate = opts.Date.Format("20060102")
		}

		rec, err := NewType14Record(Type14Options{
			IDC:                  idc,
			ImpressionType:       img.ImpressionType,
			SourceAgency:         opts.OriginatingAgency,
			CaptureDate:          captureDate,
			HorizontalLineLength: bounds.Dx(),
			VerticalLineLength:   bounds.Dy(),
			ScaleUnits:           ScaleUnitsPPI,
			HorizontalPixelScale: ppi,
			VerticalPixelScale:   ppi,
			Compression:          comp.Algorithm(),
			BitsPerPixel:         8,
			FingerPosition:       img.FingerPosition,
			ImageData:            imgData,
		})
		if err != nil {
			return nil, fmt.Errorf("eft: FingerprintImage[%d]: %w", i, err)
		}

		txn.AddRecord(rec)
	}

	return txn.Encode()
}
