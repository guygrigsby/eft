# WSQ Compression

## Overview

Wavelet Scalar Quantization (WSQ) is the FBI standard compression algorithm for 500 ppi fingerprint images. It achieves ~15:1 compression at 0.75 bitrate while preserving fingerprint minutiae.

## Compressor Interface

```go
type Compressor interface {
    Compress(img *image.Gray) ([]byte, error)
    Algorithm() CompressionAlgorithm
}
```

Two built-in implementations:
- **`WSQCompressor`** — Wraps `jtejido/go-wsq`. Default bitrate: 0.75.
- **`NoneCompressor`** — Returns raw grayscale pixels (no compression).

`DefaultCompressor()` returns `&WSQCompressor{Bitrate: 0.75}`.

## go-wsq Library

- Package: `github.com/jtejido/go-wsq` v0.0.3-beta
- Pure Go port of the NIST NBIS WSQ codec
- Input: `*image.Gray` (8-bit grayscale)
- **License note**: The port lacks an explicit license. The original NBIS code is public domain. The `Compressor` interface allows swapping implementations if this is a concern.

## Known Limitations

**WSQ fails on uniform or smooth images.** The quantization step produces zero-sized blocks when image content has insufficient variation. This affects:
- Solid color images
- Gradient patterns
- Very smooth synthetic images

Real fingerprint scans (which have ridge detail) work correctly. The test suite uses random noise images (seeded with 42) to avoid this issue.

## Implementing a Custom Compressor

For JPEG 2000 (required at 1000 ppi) or alternative WSQ implementations:

```go
type MyJP2Compressor struct{}

func (c *MyJP2Compressor) Compress(img *image.Gray) ([]byte, error) {
    // Your JP2 encoding logic
}

func (c *MyJP2Compressor) Algorithm() eft.CompressionAlgorithm {
    return eft.CompressionJP2
}
```

Pass it via `Type4Options.Compressor`, `Type14Options` image data, or `ATFSubmissionOptions.Compressor`.

## Compression Algorithm Codes

| Constant | Type-4 GCA byte | Description |
|----------|-----------------|-------------|
| `CompressionNone` | 0 | Uncompressed |
| `CompressionWSQ` | 1 | WSQ |
| `CompressionJPEGB` | 2 | JPEG baseline |
| `CompressionJPEGL` | 3 | JPEG lossless |
| `CompressionJP2` | 4 | JPEG 2000 |
| `CompressionJP2L` | 5 | JPEG 2000 lossless |

## References

- [WSQ Specification v3.1](http://www.fbibiospecs.cjis.gov/Document/Get?fileName=WSQ_Gray-scale_Specification_Version_3_1_Final.pdf)
- [NIST NBIS Software](https://www.nist.gov/services-resources/software/nist-biometric-image-software-nbis)
- [jtejido/go-wsq](https://github.com/jtejido/go-wsq)
