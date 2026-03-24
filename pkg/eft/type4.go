package eft

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
)

// Type4Options holds parameters for creating a Type-4 grayscale fingerprint
// image record. Type-4 is a fully binary record (fixed 500 ppi) used for
// rolled and plain impressions in older EFTS transactions.
type Type4Options struct {
	// IDC is the Information Designation Character (1-99).
	IDC int
	// ImpressionType indicates how the print was captured.
	ImpressionType ImpressionType
	// FingerPosition identifies which finger (1-15).
	FingerPosition FingerPosition
	// Image is the grayscale fingerprint image. Will be compressed using
	// the provided Compressor.
	Image *image.Gray
	// Compressor performs image compression. Default: WSQCompressor(0.75).
	Compressor Compressor
}

// NewType4Record creates a Type-4 binary fingerprint image record.
// Type-4 records use a fixed binary header format (not tagged fields).
//
// Binary header layout (18 bytes):
//
//	Bytes 0-3:   LEN (record length, 4 bytes big-endian)
//	Byte  4:     IDC
//	Byte  5:     IMP (impression type)
//	Bytes 6-11:  FGP (6 finger position bytes, first is position, rest 0xFF)
//	Byte  12:    ISR (scanning resolution, 0 = 500 ppi)
//	Bytes 13-14: HLL (horizontal line length, 2 bytes big-endian)
//	Bytes 15-16: VLL (vertical line length, 2 bytes big-endian)
//	Byte  17:    GCA (compression algorithm: 0=none, 1=WSQ)
//	Bytes 18+:   Image data
func NewType4Record(opts Type4Options) (*Record, []byte, error) {
	if opts.Image == nil {
		return nil, nil, fmt.Errorf("eft: Type4Options.Image is required")
	}

	comp := opts.Compressor
	if comp == nil {
		comp = DefaultCompressor()
	}

	imgData, err := comp.Compress(opts.Image)
	if err != nil {
		return nil, nil, fmt.Errorf("eft: compressing Type-4 image: %w", err)
	}

	bounds := opts.Image.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Map compression algorithm to binary value.
	var gcaByte byte
	switch comp.Algorithm() {
	case CompressionNone:
		gcaByte = 0
	case CompressionWSQ:
		gcaByte = 1
	case CompressionJPEGB:
		gcaByte = 2
	case CompressionJPEGL:
		gcaByte = 3
	case CompressionJP2:
		gcaByte = 4
	case CompressionJP2L:
		gcaByte = 5
	default:
		return nil, nil, fmt.Errorf("eft: unsupported Type-4 compression: %s", comp.Algorithm())
	}

	headerLen := 18
	totalLen := headerLen + len(imgData)

	var buf bytes.Buffer
	buf.Grow(totalLen)

	// LEN (4 bytes, big-endian)
	if err := binary.Write(&buf, binary.BigEndian, uint32(totalLen)); err != nil {
		return nil, nil, err
	}
	// IDC
	buf.WriteByte(byte(opts.IDC))
	// IMP
	buf.WriteByte(byte(opts.ImpressionType))
	// FGP (6 bytes: position + 5x 0xFF padding)
	buf.WriteByte(byte(opts.FingerPosition))
	for i := 0; i < 5; i++ {
		buf.WriteByte(0xFF)
	}
	// ISR (0 = 500 ppi native)
	buf.WriteByte(0)
	// HLL (2 bytes, big-endian)
	if err := binary.Write(&buf, binary.BigEndian, uint16(width)); err != nil {
		return nil, nil, err
	}
	// VLL (2 bytes, big-endian)
	if err := binary.Write(&buf, binary.BigEndian, uint16(height)); err != nil {
		return nil, nil, err
	}
	// GCA
	buf.WriteByte(gcaByte)
	// Image data
	buf.Write(imgData)

	rawBytes := buf.Bytes()

	// Also create a Record for use with Transaction.
	// Type-4 is binary, so we store the entire binary blob as a single "field".
	r := &Record{Type: 4}
	r.rawBinary = rawBytes

	return r, rawBytes, nil
}
