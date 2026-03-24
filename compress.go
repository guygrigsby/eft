package eft

import (
	"bytes"
	"fmt"
	"image"

	wsq "github.com/jtejido/go-wsq"
)

// Compressor compresses a grayscale fingerprint image.
// Implementations must accept *image.Gray and return compressed bytes.
type Compressor interface {
	// Compress encodes the grayscale image and returns compressed bytes.
	Compress(img *image.Gray) ([]byte, error)
	// Algorithm returns the ANSI/NIST-ITL compression algorithm identifier.
	Algorithm() CompressionAlgorithm
}

// WSQCompressor compresses fingerprint images using the FBI's Wavelet Scalar
// Quantization algorithm. The default bitrate is 0.75 (15:1 ratio) per EBTS.
type WSQCompressor struct {
	// Bitrate controls compression quality. Default 0.75 per FBI spec.
	// Lower values = smaller files, lower quality.
	Bitrate float32
}

// Compress encodes the image as WSQ.
func (c *WSQCompressor) Compress(img *image.Gray) ([]byte, error) {
	bitrate := c.Bitrate
	if bitrate <= 0 {
		bitrate = 0.75
	}

	var buf bytes.Buffer
	err := wsq.Encode(&buf, img, &wsq.Options{
		Bitrate: bitrate,
	})
	if err != nil {
		return nil, fmt.Errorf("eft: wsq compression failed: %w", err)
	}
	return buf.Bytes(), nil
}

// Algorithm returns CompressionWSQ.
func (c *WSQCompressor) Algorithm() CompressionAlgorithm {
	return CompressionWSQ
}

// NoneCompressor stores images uncompressed (raw 8-bit grayscale pixels).
type NoneCompressor struct{}

// Compress returns the raw pixel data from the Gray image.
func (c *NoneCompressor) Compress(img *image.Gray) ([]byte, error) {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// image.Gray.Pix may have a stride != width, so copy row by row.
	pixels := make([]byte, w*h)
	for y := 0; y < h; y++ {
		srcOff := (y+bounds.Min.Y-img.Rect.Min.Y)*img.Stride + (bounds.Min.X - img.Rect.Min.X)
		copy(pixels[y*w:(y+1)*w], img.Pix[srcOff:srcOff+w])
	}
	return pixels, nil
}

// Algorithm returns CompressionNone.
func (c *NoneCompressor) Algorithm() CompressionAlgorithm {
	return CompressionNone
}

// DefaultCompressor returns a WSQCompressor with the standard 0.75 bitrate.
func DefaultCompressor() Compressor {
	return &WSQCompressor{Bitrate: 0.75}
}
