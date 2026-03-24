package eft

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// FD258Layout defines the crop regions on an FD-258 fingerprint card.
// All coordinates are expressed as fractions of the full card's width/height
// (0.0–1.0), making the layout resolution-independent.
//
// The standard FBI FD-258 card is 8" x 8". The demographic header occupies
// roughly the top 28% of the card; fingerprint boxes occupy the remainder.
// The default layout accounts for the full card — no pre-cropping required.
type FD258Layout struct {
	// RolledPrints maps finger positions 1-10 to their crop rectangles.
	RolledPrints [10]FractionalRect
	// FlatRight is the right four-finger slap area (positions 2-5).
	FlatRight FractionalRect
	// FlatLeft is the left four-finger slap area (positions 7-10).
	FlatLeft FractionalRect
	// FlatThumbs is the both-thumbs simultaneous area (positions 1,6).
	FlatThumbs FractionalRect
}

// FractionalRect defines a rectangle as fractions of the parent image dimensions.
type FractionalRect struct {
	X1, Y1, X2, Y2 float64
}

// toRect converts fractional coordinates to pixel coordinates.
func (fr FractionalRect) toRect(w, h int) image.Rectangle {
	return image.Rect(
		int(math.Round(fr.X1*float64(w))),
		int(math.Round(fr.Y1*float64(h))),
		int(math.Round(fr.X2*float64(w))),
		int(math.Round(fr.Y2*float64(h))),
	)
}

// DefaultFD258Layout returns the standard FD-258 card layout.
//
// The full FD-258 card layout (looking at the card):
//
// Top ~28%:    [Demographic header — name, DOB, etc.]
// Row 1 (rolled): [1.R.Thumb] [2.R.Index] [3.R.Middle] [4.R.Ring] [5.R.Little]
// Row 2 (rolled): [6.L.Thumb] [7.L.Index] [8.L.Middle] [9.L.Ring] [10.L.Little]
// Row 3 (flat):   [L.Four Fingers] [L.Thumb] [R.Thumb] [R.Four Fingers]
//
// These coordinates are relative to the full card image including the
// demographic header. The header area (~28%) is simply not cropped.
func DefaultFD258Layout() FD258Layout {
	// The FD-258 card is 8" x 8". The demographic header occupies roughly
	// the top 2.25" (~28%). All Y coordinates below are relative to the
	// full card height, with the fingerprint area spanning ~0.30 to ~0.98.
	//
	// Rolled prints: 2 rows of 5, each box ~20% of card width.
	// Flat prints: bottom row of the card.
	const (
		// Rolled rows (Y coordinates relative to full card)
		row1Top    = 0.30
		row1Bottom = 0.53
		row2Top    = 0.55
		row2Bottom = 0.76

		// Column positions for 5 boxes per row (unchanged — full width)
		col1Left  = 0.02
		col1Right = 0.20
		col2Left  = 0.20
		col2Right = 0.40
		col3Left  = 0.40
		col3Right = 0.60
		col4Left  = 0.60
		col4Right = 0.80
		col5Left  = 0.80
		col5Right = 0.98

		// Flat print row (Y coordinates relative to full card)
		flatTop    = 0.78
		flatBottom = 0.98

		// Flat columns: [left 4 fingers][left thumb][right thumb][right 4 fingers]
		flatLeftStart       = 0.02
		flatLeftEnd         = 0.37
		flatLeftThumbStart  = 0.37
		flatLeftThumbEnd    = 0.50
		flatRightThumbStart = 0.50
		flatRightThumbEnd   = 0.63
		flatRightStart      = 0.63
		flatRightEnd        = 0.98
	)

	return FD258Layout{
		RolledPrints: [10]FractionalRect{
			// 1: Right Thumb
			{col1Left, row1Top, col1Right, row1Bottom},
			// 2: Right Index
			{col2Left, row1Top, col2Right, row1Bottom},
			// 3: Right Middle
			{col3Left, row1Top, col3Right, row1Bottom},
			// 4: Right Ring
			{col4Left, row1Top, col4Right, row1Bottom},
			// 5: Right Little
			{col5Left, row1Top, col5Right, row1Bottom},
			// 6: Left Thumb
			{col1Left, row2Top, col1Right, row2Bottom},
			// 7: Left Index
			{col2Left, row2Top, col2Right, row2Bottom},
			// 8: Left Middle
			{col3Left, row2Top, col3Right, row2Bottom},
			// 9: Left Ring
			{col4Left, row2Top, col4Right, row2Bottom},
			// 10: Left Little
			{col5Left, row2Top, col5Right, row2Bottom},
		},
		FlatLeft: FractionalRect{
			flatLeftStart, flatTop, flatLeftEnd, flatBottom,
		},
		FlatThumbs: FractionalRect{
			flatLeftThumbStart, flatTop, flatRightThumbEnd, flatBottom,
		},
		FlatRight: FractionalRect{
			flatRightStart, flatTop, flatRightEnd, flatBottom,
		},
	}
}

// CropFD258 extracts individual fingerprint images from a scanned FD-258 card.
// The input image should be a scan of the entire FD-258 card (including the
// demographic header). The layout coordinates define where fingerprint boxes
// are located on the full card; the header area is simply ignored.
// Returns a FD258Images struct containing the cropped regions.
func CropFD258(img image.Image, layout FD258Layout) (*FD258Images, error) {
	gray := imageToGray(img)
	bounds := gray.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if w < 100 || h < 100 {
		return nil, fmt.Errorf("eft: image too small for FD-258 extraction (%dx%d)", w, h)
	}

	result := &FD258Images{}

	// Crop rolled prints.
	for i := 0; i < 10; i++ {
		rect := layout.RolledPrints[i].toRect(w, h)
		result.Rolled[i] = cropGray(gray, rect)
	}

	// Crop flat prints.
	result.FlatRight = cropGray(gray, layout.FlatRight.toRect(w, h))
	result.FlatLeft = cropGray(gray, layout.FlatLeft.toRect(w, h))
	result.FlatThumbs = cropGray(gray, layout.FlatThumbs.toRect(w, h))

	return result, nil
}

// FD258Images holds the cropped fingerprint images from an FD-258 card.
type FD258Images struct {
	// Rolled holds the 10 individual rolled fingerprint images (index 0 = finger position 1).
	Rolled [10]*image.Gray
	// FlatRight is the right four-finger slap.
	FlatRight *image.Gray
	// FlatLeft is the left four-finger slap.
	FlatLeft *image.Gray
	// FlatThumbs is the simultaneous both-thumbs impression.
	FlatThumbs *image.Gray
}

// imageToGray converts any image to *image.Gray.
func imageToGray(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.SetGray(x, y, color.GrayModel.Convert(img.At(x, y)).(color.Gray))
		}
	}
	return gray
}

// cropGray extracts a sub-image from a Gray image and returns a new Gray image
// with its own backing array (not a sub-slice of the original).
func cropGray(img *image.Gray, rect image.Rectangle) *image.Gray {
	rect = rect.Intersect(img.Bounds())
	w := rect.Dx()
	h := rect.Dy()
	dst := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		srcOff := (rect.Min.Y+y-img.Rect.Min.Y)*img.Stride + (rect.Min.X - img.Rect.Min.X)
		copy(dst.Pix[y*dst.Stride:y*dst.Stride+w], img.Pix[srcOff:srcOff+w])
	}
	return dst
}
