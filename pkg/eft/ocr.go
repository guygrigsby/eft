package eft

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"regexp"
	"strings"
	"time"
)

// OCRProvider recognizes text in an image. Implementations may use
// cloud services (Google Cloud Vision, AWS Textract), local engines
// (Tesseract), or any other text recognition backend.
//
// RecognizeText should return the recognized text stripped of leading/trailing
// whitespace. Return an empty string (not an error) if no text is detected.
type OCRProvider interface {
	RecognizeText(ctx context.Context, img image.Image) (string, error)
}

// FD258HeaderFields defines crop regions for demographic fields in the
// FD-258 card header. All coordinates are fractional (0.0–1.0) relative
// to the full card dimensions (including fingerprint area).
type FD258HeaderFields struct {
	Name         FractionalRect
	Address      FractionalRect
	DOB          FractionalRect
	Sex          FractionalRect
	Race         FractionalRect
	Height       FractionalRect
	Weight       FractionalRect
	EyeColor     FractionalRect
	HairColor    FractionalRect
	PlaceOfBirth FractionalRect
	Citizenship  FractionalRect
	SSN          FractionalRect
}

// DefaultFD258HeaderFields returns standard FD-258 header field positions.
// These coordinates are calibrated for the standard FBI FD-258 card
// (8" × 8"). Override individual fields for non-standard cards or cards
// with alignment issues.
//
// The header occupies the top ~28% of the card. Within that space, fields
// are arranged in labeled rows:
//
//	Row 1 (Y ~0.025–0.07):  NAME (Last, First, Middle)
//	Row 2 (Y ~0.095–0.13):  RESIDENCE (address)
//	Row 3 (Y ~0.165–0.205): DOB | SEX | RACE | HGT | WGT | EYES | HAIR | POB
//	Row 4 (Y ~0.205–0.235): CITIZENSHIP
//	Row 5 (Y ~0.255–0.28):  SSN
func DefaultFD258HeaderFields() FD258HeaderFields {
	return FD258HeaderFields{
		Name:         FractionalRect{0.15, 0.025, 0.98, 0.07},
		Address:      FractionalRect{0.02, 0.095, 0.98, 0.13},
		DOB:          FractionalRect{0.02, 0.165, 0.13, 0.205},
		Sex:          FractionalRect{0.13, 0.165, 0.19, 0.205},
		Race:         FractionalRect{0.19, 0.165, 0.26, 0.205},
		Height:       FractionalRect{0.26, 0.165, 0.34, 0.205},
		Weight:       FractionalRect{0.34, 0.165, 0.42, 0.205},
		EyeColor:     FractionalRect{0.42, 0.165, 0.50, 0.205},
		HairColor:    FractionalRect{0.50, 0.165, 0.58, 0.205},
		PlaceOfBirth: FractionalRect{0.58, 0.165, 0.98, 0.205},
		Citizenship:  FractionalRect{0.02, 0.205, 0.20, 0.235},
		SSN:          FractionalRect{0.55, 0.255, 0.75, 0.28},
	}
}

// DemographicResult holds the extracted demographic data and any
// warnings about fields that couldn't be read or normalized.
type DemographicResult struct {
	// Person contains the best-effort parsed demographic data.
	Person ATFPersonInfo
	// Warnings lists fields that couldn't be read or normalized.
	// Required fields (name, DOB) that fail appear here as warnings
	// rather than causing an error, so the caller can review partial
	// results and supply missing values via CLI flags or other means.
	Warnings []string
	// RawFields contains the raw OCR text for each field before
	// normalization. Useful for debugging OCR quality.
	RawFields map[string]string
}

// ExtractDemographics crops each demographic field from the FD-258 card
// header image, OCRs it, and normalizes the results into an ATFPersonInfo.
//
// Fields that fail OCR or normalization are left empty and noted in Warnings.
// Required fields (name, DOB) produce warnings but do not cause the function
// to return an error — the caller decides how to handle partial results
// (e.g., prompt the user or fill from CLI flags).
func ExtractDemographics(ctx context.Context, img image.Image, ocr OCRProvider, fields FD258HeaderFields) (*DemographicResult, error) {
	if ocr == nil {
		return nil, fmt.Errorf("eft: OCR provider is required")
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w < 100 || h < 100 {
		return nil, fmt.Errorf("eft: image too small for header extraction (%dx%d)", w, h)
	}

	result := &DemographicResult{
		RawFields: make(map[string]string),
	}

	// ocrField crops a region and runs OCR on it.
	ocrField := func(name string, rect FractionalRect) string {
		crop := cropSubImage(img, rect.toRect(w, h))
		text, err := ocr.RecognizeText(ctx, crop)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: OCR failed: %v", name, err))
			return ""
		}
		text = strings.TrimSpace(text)
		result.RawFields[name] = text
		return text
	}

	// Name
	nameRaw := ocrField("name", fields.Name)
	if nameRaw != "" {
		last, first, middle := parseName(nameRaw)
		result.Person.LastName = last
		result.Person.FirstName = first
		result.Person.MiddleName = middle
	}
	if result.Person.LastName == "" {
		result.Warnings = append(result.Warnings, "name: could not extract last name (required)")
	}
	if result.Person.FirstName == "" {
		result.Warnings = append(result.Warnings, "name: could not extract first name (required)")
	}

	// DOB
	dobRaw := ocrField("dob", fields.DOB)
	if dobRaw != "" {
		dob, err := parseDOB(dobRaw)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("dob: could not parse %q: %v", dobRaw, err))
		} else {
			result.Person.DOB = dob
		}
	} else {
		result.Warnings = append(result.Warnings, "dob: could not extract date of birth (required)")
	}

	// Sex
	if raw := ocrField("sex", fields.Sex); raw != "" {
		result.Person.Sex = normalizeSex(raw)
	}

	// Race
	if raw := ocrField("race", fields.Race); raw != "" {
		result.Person.Race = normalizeRace(raw)
	}

	// Height
	if raw := ocrField("height", fields.Height); raw != "" {
		result.Person.Height = normalizeHeight(raw)
	}

	// Weight
	if raw := ocrField("weight", fields.Weight); raw != "" {
		result.Person.Weight = normalizeWeight(raw)
	}

	// Eye color
	if raw := ocrField("eye_color", fields.EyeColor); raw != "" {
		result.Person.EyeColor = normalizeEyeColor(raw)
	}

	// Hair color
	if raw := ocrField("hair_color", fields.HairColor); raw != "" {
		result.Person.HairColor = normalizeHairColor(raw)
	}

	// Place of birth
	if raw := ocrField("place_of_birth", fields.PlaceOfBirth); raw != "" {
		result.Person.PlaceOfBirth = strings.ToUpper(strings.TrimSpace(raw))
	}

	// Citizenship
	if raw := ocrField("citizenship", fields.Citizenship); raw != "" {
		result.Person.Citizenship = normalizeCitizenship(raw)
	}

	// SSN
	if raw := ocrField("ssn", fields.SSN); raw != "" {
		result.Person.SSN = normalizeSSN(raw)
	}

	// Address
	if raw := ocrField("address", fields.Address); raw != "" {
		result.Person.Address = raw
	}

	return result, nil
}

// CropHeader extracts the demographic header region from an FD-258 card
// image. Returns the top ~36% of the card (everything above the fingerprint
// boxes). Useful for document-level OCR where the entire header is read
// at once and then parsed.
func CropHeader(img image.Image) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	headerRect := FractionalRect{0.0, 0.0, 1.0, 0.36}.toRect(w, h)
	return cropSubImage(img, headerRect)
}

// cropSubImage extracts a sub-image from any image type and returns a
// new image with its own backing array.
func cropSubImage(img image.Image, rect image.Rectangle) image.Image {
	if g, ok := img.(*image.Gray); ok {
		return cropGray(g, rect)
	}
	rect = rect.Intersect(img.Bounds())
	w := rect.Dx()
	h := rect.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(x, y, img.At(rect.Min.X+x, rect.Min.Y+y))
		}
	}
	return dst
}

// CropHeaderField extracts a single demographic field from the card image
// for inspection or custom OCR. Returns a grayscale image of the field.
func CropHeaderField(img image.Image, field FractionalRect) *image.Gray {
	bounds := img.Bounds()
	rect := field.toRect(bounds.Dx(), bounds.Dy())
	sub := cropSubImage(img, rect)
	if g, ok := sub.(*image.Gray); ok {
		return g
	}
	// Convert to gray
	b := sub.Bounds()
	gray := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gray.SetGray(x-b.Min.X, y-b.Min.Y,
				color.GrayModel.Convert(sub.At(x, y)).(color.Gray))
		}
	}
	return gray
}

// --- Name parsing ---

// parseName extracts last, first, and middle names from OCR text.
// Handles comma-separated ("Last, First Middle") and space-separated
// ("Last First Middle") formats.
func parseName(raw string) (last, first, middle string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}

	// Try comma-separated: "Last, First Middle"
	if idx := strings.Index(raw, ","); idx > 0 {
		last = strings.TrimSpace(raw[:idx])
		rest := strings.TrimSpace(raw[idx+1:])
		parts := strings.Fields(rest)
		if len(parts) >= 1 {
			first = parts[0]
		}
		if len(parts) >= 2 {
			middle = strings.Join(parts[1:], " ")
		}
		return
	}

	// Space-separated: assume "Last First Middle"
	parts := strings.Fields(raw)
	switch len(parts) {
	case 1:
		last = parts[0]
	case 2:
		last = parts[0]
		first = parts[1]
	default:
		last = parts[0]
		first = parts[1]
		middle = strings.Join(parts[2:], " ")
	}
	return
}

// --- DOB parsing ---

var dobFormats = []string{
	"01/02/2006",
	"01-02-2006",
	"2006-01-02",
	"01/02/06",
	"01-02-06",
	"January 2, 2006",
	"Jan 2, 2006",
}

// parseDOB attempts to parse a date of birth string in various formats.
func parseDOB(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)

	// Try standard formats first (before digit substitution).
	for _, layout := range dobFormats {
		if t, err := time.Parse(layout, raw); err == nil {
			if t.Year() >= 1900 && t.Before(time.Now().AddDate(0, 0, 1)) {
				return t, nil
			}
		}
	}

	// Try all-digit formats: MMDDYYYY or YYYYMMDD.
	digits := extractDigits(raw)
	if len(digits) == 8 {
		for _, layout := range []string{"01022006", "20060102"} {
			if t, err := time.Parse(layout, digits); err == nil {
				if t.Year() >= 1900 && t.Before(time.Now().AddDate(0, 0, 1)) {
					return t, nil
				}
			}
		}
	}

	return time.Time{}, fmt.Errorf("unrecognized date format")
}

// --- Sex normalization ---

func normalizeSex(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case raw == "M" || strings.HasPrefix(raw, "MAL"):
		return "M"
	case raw == "F" || strings.HasPrefix(raw, "FEM"):
		return "F"
	case raw == "X":
		return "X"
	default:
		return raw
	}
}

// --- Race normalization ---

func normalizeRace(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case raw == "W" || strings.HasPrefix(raw, "WHI") || strings.HasPrefix(raw, "CAU"):
		return "W"
	case raw == "B" || strings.HasPrefix(raw, "BLA") || strings.HasPrefix(raw, "AFR"):
		return "B"
	case raw == "A" || strings.HasPrefix(raw, "ASI"):
		return "A"
	case raw == "I" || strings.Contains(raw, "INDIAN") || strings.Contains(raw, "NATIVE"):
		return "I"
	case raw == "U" || strings.HasPrefix(raw, "UNK"):
		return "U"
	default:
		return raw
	}
}

// --- Height normalization ---

var heightPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^(\d)\s*['']\s*(\d{1,2})\s*[""]?$`), // 5'10" or 5' 10
	regexp.MustCompile(`^(\d)\s*-\s*(\d{1,2})$`),             // 5-10
	regexp.MustCompile(`^(\d)\s+(\d{1,2})$`),                 // 5 10
	regexp.MustCompile(`^(\d)(\d{2})$`),                      // 510
}

func normalizeHeight(raw string) string {
	raw = strings.TrimSpace(raw)
	for _, pat := range heightPatterns {
		if m := pat.FindStringSubmatch(raw); m != nil {
			feet := m[1]
			inches := m[2]
			if len(inches) == 1 {
				inches = "0" + inches
			}
			return feet + inches
		}
	}
	// Already a 3-digit code?
	if len(raw) == 3 && raw[0] >= '3' && raw[0] <= '7' {
		allDigits := true
		for _, c := range raw {
			if c < '0' || c > '9' {
				allDigits = false
				break
			}
		}
		if allDigits {
			return raw
		}
	}
	return raw
}

// --- Weight normalization ---

func normalizeWeight(raw string) string {
	raw = strings.TrimSpace(raw)
	digits := extractDigits(raw)
	if digits == "" {
		return raw
	}
	return digits
}

// --- Eye color normalization ---

var eyeColorMap = map[string]string{
	"BROWN": "BRO", "BRO": "BRO", "BRN": "BRO",
	"BLUE": "BLU", "BLU": "BLU",
	"GREEN": "GRN", "GRN": "GRN",
	"HAZEL": "HAZ", "HAZ": "HAZ",
	"GRAY": "GRY", "GREY": "GRY", "GRY": "GRY",
	"BLACK": "BLK", "BLK": "BLK",
	"MAROON": "MAR", "MAR": "MAR",
	"MULTICOLOR": "MUL", "MUL": "MUL",
	"PINK": "PNK", "PNK": "PNK",
	"UNKNOWN": "XXX", "XXX": "XXX",
}

func normalizeEyeColor(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if code, ok := eyeColorMap[raw]; ok {
		return code
	}
	for word, code := range eyeColorMap {
		if len(word) > 3 && strings.HasPrefix(word, raw) {
			return code
		}
	}
	return raw
}

// --- Hair color normalization ---

var hairColorMap = map[string]string{
	"BLACK": "BLK", "BLK": "BLK",
	"BROWN": "BRO", "BRO": "BRO", "BRN": "BRO",
	"BLOND": "BLN", "BLONDE": "BLN", "BLN": "BLN",
	"RED": "RED",
	"GRAY": "GRY", "GREY": "GRY", "GRY": "GRY",
	"WHITE": "WHI", "WHI": "WHI",
	"SANDY": "SDY", "SDY": "SDY",
	"BALD": "BAL", "BAL": "BAL",
	"UNKNOWN": "XXX", "XXX": "XXX",
}

func normalizeHairColor(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if code, ok := hairColorMap[raw]; ok {
		return code
	}
	for word, code := range hairColorMap {
		if len(word) > 3 && strings.HasPrefix(word, raw) {
			return code
		}
	}
	return raw
}

// --- SSN normalization ---

func normalizeSSN(raw string) string {
	raw = strings.TrimSpace(raw)
	digits := extractDigits(raw)
	if len(digits) == 9 {
		return digits
	}
	return raw
}

// --- Citizenship normalization ---

func normalizeCitizenship(raw string) string {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	switch {
	case raw == "US" || raw == "USA" || raw == "U.S." || raw == "U.S.A." ||
		strings.Contains(raw, "UNITED STATES") || strings.Contains(raw, "AMERICAN"):
		return "US"
	default:
		return raw
	}
}

// --- Helpers ---

// extractDigits returns only the digit characters from a string.
func extractDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// RawDemographicFields holds raw OCR text for each demographic field.
// Used by NormalizeRawDemographics to apply normalization outside of the
// full ExtractDemographics pipeline (e.g., from browser-side OCR via WASM).
type RawDemographicFields struct {
	Name         string `json:"name"`
	DOB          string `json:"dob"`
	Sex          string `json:"sex"`
	Race         string `json:"race"`
	Height       string `json:"height"`
	Weight       string `json:"weight"`
	EyeColor     string `json:"eye_color"`
	HairColor    string `json:"hair_color"`
	PlaceOfBirth string `json:"place_of_birth"`
	Citizenship  string `json:"citizenship"`
	SSN          string `json:"ssn"`
	Address      string `json:"address"`
}

// NormalizeRawDemographics applies the normalization pipeline to raw OCR
// text for each field and returns a populated ATFPersonInfo. This is the
// exported entry point for normalizing OCR results produced outside of Go
// (e.g., tesseract.js in the browser).
func NormalizeRawDemographics(raw RawDemographicFields) ATFPersonInfo {
	var p ATFPersonInfo
	if raw.Name != "" {
		p.LastName, p.FirstName, p.MiddleName = parseName(raw.Name)
	}
	if raw.DOB != "" {
		if dob, err := parseDOB(raw.DOB); err == nil {
			p.DOB = dob
		}
	}
	if raw.Sex != "" {
		p.Sex = normalizeSex(raw.Sex)
	}
	if raw.Race != "" {
		p.Race = normalizeRace(raw.Race)
	}
	if raw.Height != "" {
		p.Height = normalizeHeight(raw.Height)
	}
	if raw.Weight != "" {
		p.Weight = normalizeWeight(raw.Weight)
	}
	if raw.EyeColor != "" {
		p.EyeColor = normalizeEyeColor(raw.EyeColor)
	}
	if raw.HairColor != "" {
		p.HairColor = normalizeHairColor(raw.HairColor)
	}
	if raw.PlaceOfBirth != "" {
		p.PlaceOfBirth = strings.ToUpper(strings.TrimSpace(raw.PlaceOfBirth))
	}
	if raw.Citizenship != "" {
		p.Citizenship = normalizeCitizenship(raw.Citizenship)
	}
	if raw.SSN != "" {
		p.SSN = normalizeSSN(raw.SSN)
	}
	if raw.Address != "" {
		p.Address = strings.TrimSpace(raw.Address)
	}
	return p
}

// MergeDemographics copies non-empty fields from override into base.
// This allows CLI flags to selectively override OCR results.
func MergeDemographics(base, override ATFPersonInfo) ATFPersonInfo {
	if override.LastName != "" {
		base.LastName = override.LastName
	}
	if override.FirstName != "" {
		base.FirstName = override.FirstName
	}
	if override.MiddleName != "" {
		base.MiddleName = override.MiddleName
	}
	if !override.DOB.IsZero() {
		base.DOB = override.DOB
	}
	if override.Sex != "" {
		base.Sex = override.Sex
	}
	if override.Race != "" {
		base.Race = override.Race
	}
	if override.PlaceOfBirth != "" {
		base.PlaceOfBirth = override.PlaceOfBirth
	}
	if override.Citizenship != "" {
		base.Citizenship = override.Citizenship
	}
	if override.Height != "" {
		base.Height = override.Height
	}
	if override.Weight != "" {
		base.Weight = override.Weight
	}
	if override.EyeColor != "" {
		base.EyeColor = override.EyeColor
	}
	if override.HairColor != "" {
		base.HairColor = override.HairColor
	}
	if override.SSN != "" {
		base.SSN = override.SSN
	}
	if override.Address != "" {
		base.Address = override.Address
	}
	return base
}
