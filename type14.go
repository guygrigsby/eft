package eft

import (
	"fmt"
	"strconv"
)

// FingerPosition identifies a finger per ANSI/NIST-ITL.
type FingerPosition int

const (
	FingerRightThumb  FingerPosition = 1
	FingerRightIndex  FingerPosition = 2
	FingerRightMiddle FingerPosition = 3
	FingerRightRing   FingerPosition = 4
	FingerRightLittle FingerPosition = 5
	FingerLeftThumb   FingerPosition = 6
	FingerLeftIndex   FingerPosition = 7
	FingerLeftMiddle  FingerPosition = 8
	FingerLeftRing    FingerPosition = 9
	FingerLeftLittle  FingerPosition = 10
	// Slap positions
	FingerRightFourFingers FingerPosition = 13
	FingerLeftFourFingers  FingerPosition = 14
	FingerBothThumbs       FingerPosition = 15
)

// ImpressionType identifies how the fingerprint was captured.
type ImpressionType int

const (
	ImpressionLiveScanPlain  ImpressionType = 0
	ImpressionLiveScanRolled ImpressionType = 1
	ImpressionNonLivePlain   ImpressionType = 2
	ImpressionNonLiveRolled  ImpressionType = 3
	ImpressionLatent         ImpressionType = 4
	ImpressionSwipe          ImpressionType = 8
)

// ScaleUnits identifies the unit system for pixel density.
type ScaleUnits int

const (
	ScaleUnitsNone ScaleUnits = 0 // no scale — rejected by NGI
	ScaleUnitsPPI  ScaleUnits = 1 // pixels per inch
	ScaleUnitsPPCM ScaleUnits = 2 // pixels per centimeter
)

// Type14Options holds parameters for creating a Type-14 fingerprint image record.
type Type14Options struct {
	// IDC is the Information Designation Character (1-99).
	IDC int
	// ImpressionType indicates how the print was captured.
	ImpressionType ImpressionType
	// SourceAgency is the ORI of the agency that captured the image (14.004 SRC).
	SourceAgency string
	// CaptureDate is YYYYMMDD (14.005 FCD).
	CaptureDate string
	// HorizontalLineLength is the image width in pixels (14.006 HLL).
	HorizontalLineLength int
	// VerticalLineLength is the image height in pixels (14.007 VLL).
	VerticalLineLength int
	// ScaleUnits specifies the pixel density unit (14.008 SLC). Default PPI.
	ScaleUnits ScaleUnits
	// HorizontalPixelScale is the horizontal resolution (14.009 HPS), e.g. 500.
	HorizontalPixelScale int
	// VerticalPixelScale is the vertical resolution (14.010 VPS), e.g. 500.
	VerticalPixelScale int
	// Compression is the algorithm used on ImageData (14.011 CGA).
	Compression CompressionAlgorithm
	// BitsPerPixel is typically 8 for grayscale (14.012 BPX).
	BitsPerPixel int
	// FingerPosition identifies which finger (14.013 FGP).
	FingerPosition FingerPosition
	// ImageData is the raw or compressed fingerprint image bytes (14.999 DATA).
	ImageData []byte
}

// NewType14Record creates a Type-14 Variable Resolution Fingerprint Image record.
func NewType14Record(opts Type14Options) (*Record, error) {
	if opts.ImageData == nil {
		return nil, fmt.Errorf("eft: Type14Options.ImageData is required")
	}
	if opts.HorizontalLineLength == 0 || opts.VerticalLineLength == 0 {
		return nil, fmt.Errorf("eft: image dimensions are required")
	}
	if opts.SourceAgency == "" {
		return nil, fmt.Errorf("eft: Type14Options.SourceAgency is required")
	}
	if opts.CaptureDate == "" {
		return nil, fmt.Errorf("eft: Type14Options.CaptureDate is required")
	}

	r := &Record{Type: 14}

	// 14.001 LEN — computed during encode
	// 14.002 IDC
	r.SetField(2, []byte(formatIDC(opts.IDC)))

	// 14.003 IMP — impression type
	r.SetField(3, []byte(strconv.Itoa(int(opts.ImpressionType))))

	// 14.004 SRC
	r.SetField(4, []byte(opts.SourceAgency))

	// 14.005 FCD — capture date
	r.SetField(5, []byte(opts.CaptureDate))

	// 14.006 HLL
	r.SetField(6, []byte(strconv.Itoa(opts.HorizontalLineLength)))

	// 14.007 VLL
	r.SetField(7, []byte(strconv.Itoa(opts.VerticalLineLength)))

	// 14.008 SLC — scale units
	su := opts.ScaleUnits
	if su == 0 {
		su = ScaleUnitsPPI
	}
	r.SetField(8, []byte(strconv.Itoa(int(su))))

	// 14.009 HPS
	hps := opts.HorizontalPixelScale
	if hps == 0 {
		hps = 500
	}
	r.SetField(9, []byte(strconv.Itoa(hps)))

	// 14.010 VPS
	vps := opts.VerticalPixelScale
	if vps == 0 {
		vps = 500
	}
	r.SetField(10, []byte(strconv.Itoa(vps)))

	// 14.011 CGA
	cga := opts.Compression
	if cga == "" {
		cga = CompressionNone
	}
	r.SetField(11, []byte(string(cga)))

	// 14.012 BPX
	bpx := opts.BitsPerPixel
	if bpx == 0 {
		bpx = 8
	}
	r.SetField(12, []byte(strconv.Itoa(bpx)))

	// 14.013 FGP
	r.SetField(13, []byte(strconv.Itoa(int(opts.FingerPosition))))

	// 14.999 DATA — image data (must be last field, handled by encoding)
	r.SetField(999, opts.ImageData)

	return r, nil
}
