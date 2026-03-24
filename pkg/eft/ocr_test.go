package eft

import (
	"context"
	"fmt"
	"image"
	"testing"
	"time"
)

// sequentialOCR returns responses in order of calls.
type sequentialOCR struct {
	responses []string
	index     int
}

func (s *sequentialOCR) RecognizeText(_ context.Context, _ image.Image) (string, error) {
	if s.index >= len(s.responses) {
		return "", nil
	}
	text := s.responses[s.index]
	s.index++
	return text, nil
}

// fixedOCR always returns the same text.
type fixedOCR struct {
	text string
	err  error
}

func (f *fixedOCR) RecognizeText(_ context.Context, _ image.Image) (string, error) {
	return f.text, f.err
}

func TestParseName_CommaSeparated(t *testing.T) {
	tests := []struct {
		input                      string
		wantLast, wantFirst, wantM string
	}{
		{"Doe, John", "Doe", "John", ""},
		{"Doe, John Michael", "Doe", "John", "Michael"},
		{"SMITH, JANE Q", "SMITH", "JANE", "Q"},
		{"O'Brien, Patrick James Jr", "O'Brien", "Patrick", "James Jr"},
	}
	for _, tt := range tests {
		last, first, middle := parseName(tt.input)
		if last != tt.wantLast || first != tt.wantFirst || middle != tt.wantM {
			t.Errorf("parseName(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.input, last, first, middle, tt.wantLast, tt.wantFirst, tt.wantM)
		}
	}
}

func TestParseName_SpaceSeparated(t *testing.T) {
	tests := []struct {
		input                      string
		wantLast, wantFirst, wantM string
	}{
		{"DOE JOHN", "DOE", "JOHN", ""},
		{"DOE JOHN MICHAEL", "DOE", "JOHN", "MICHAEL"},
		{"SMITH", "SMITH", "", ""},
		{"", "", "", ""},
	}
	for _, tt := range tests {
		last, first, middle := parseName(tt.input)
		if last != tt.wantLast || first != tt.wantFirst || middle != tt.wantM {
			t.Errorf("parseName(%q) = (%q, %q, %q), want (%q, %q, %q)",
				tt.input, last, first, middle, tt.wantLast, tt.wantFirst, tt.wantM)
		}
	}
}

func TestParseDOB(t *testing.T) {
	tests := []struct {
		input string
		want  string // YYYYMMDD
	}{
		{"01/15/1990", "19900115"},
		{"01-15-1990", "19900115"},
		{"1990-01-15", "19900115"},
		{"01152000", "20000115"}, // MMDDYYYY → Jan 15, 2000
		{"20000115", "20000115"}, // YYYYMMDD → Jan 15, 2000
		{"01/15/90", "19900115"},
	}
	for _, tt := range tests {
		got, err := parseDOB(tt.input)
		if err != nil {
			t.Errorf("parseDOB(%q) error: %v", tt.input, err)
			continue
		}
		gotStr := got.Format("20060102")
		if gotStr != tt.want {
			t.Errorf("parseDOB(%q) = %s, want %s", tt.input, gotStr, tt.want)
		}
	}
}

func TestParseDOB_Invalid(t *testing.T) {
	invalids := []string{"", "not a date", "13/45/2000", "hello"}
	for _, s := range invalids {
		_, err := parseDOB(s)
		if err == nil {
			t.Errorf("parseDOB(%q) should fail", s)
		}
	}
}

func TestNormalizeSex(t *testing.T) {
	tests := []struct{ input, want string }{
		{"M", "M"},
		{"m", "M"},
		{"MALE", "M"},
		{"Male", "M"},
		{"F", "F"},
		{"FEMALE", "F"},
		{"X", "X"},
	}
	for _, tt := range tests {
		if got := normalizeSex(tt.input); got != tt.want {
			t.Errorf("normalizeSex(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeRace(t *testing.T) {
	tests := []struct{ input, want string }{
		{"W", "W"},
		{"WHITE", "W"},
		{"CAUCASIAN", "W"},
		{"B", "B"},
		{"BLACK", "B"},
		{"A", "A"},
		{"ASIAN", "A"},
		{"I", "I"},
		{"AMERICAN INDIAN", "I"},
		{"U", "U"},
		{"UNKNOWN", "U"},
	}
	for _, tt := range tests {
		if got := normalizeRace(tt.input); got != tt.want {
			t.Errorf("normalizeRace(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeHeight(t *testing.T) {
	tests := []struct{ input, want string }{
		{"5'10\"", "510"},
		{"5'10", "510"},
		{"5-10", "510"},
		{"5 10", "510"},
		{"510", "510"},
		{"6'2\"", "602"},
		{"6'2", "602"},
		{"5'8", "508"},
		{"508", "508"},
	}
	for _, tt := range tests {
		if got := normalizeHeight(tt.input); got != tt.want {
			t.Errorf("normalizeHeight(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeWeight(t *testing.T) {
	tests := []struct{ input, want string }{
		{"180", "180"},
		{"180 lbs", "180"},
		{"200lbs", "200"},
		{"95 kg", "95"},
	}
	for _, tt := range tests {
		if got := normalizeWeight(tt.input); got != tt.want {
			t.Errorf("normalizeWeight(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeEyeColor(t *testing.T) {
	tests := []struct{ input, want string }{
		{"BROWN", "BRO"},
		{"BRO", "BRO"},
		{"BRN", "BRO"},
		{"BLUE", "BLU"},
		{"GREEN", "GRN"},
		{"HAZEL", "HAZ"},
		{"BLACK", "BLK"},
	}
	for _, tt := range tests {
		if got := normalizeEyeColor(tt.input); got != tt.want {
			t.Errorf("normalizeEyeColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeHairColor(t *testing.T) {
	tests := []struct{ input, want string }{
		{"BLACK", "BLK"},
		{"BLK", "BLK"},
		{"BROWN", "BRO"},
		{"BLOND", "BLN"},
		{"BLONDE", "BLN"},
		{"RED", "RED"},
		{"GRAY", "GRY"},
		{"GREY", "GRY"},
		{"BALD", "BAL"},
	}
	for _, tt := range tests {
		if got := normalizeHairColor(tt.input); got != tt.want {
			t.Errorf("normalizeHairColor(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeSSN(t *testing.T) {
	tests := []struct{ input, want string }{
		{"123456789", "123456789"},
		{"123-45-6789", "123456789"},
		{"123 45 6789", "123456789"},
	}
	for _, tt := range tests {
		if got := normalizeSSN(tt.input); got != tt.want {
			t.Errorf("normalizeSSN(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNormalizeCitizenship(t *testing.T) {
	tests := []struct{ input, want string }{
		{"US", "US"},
		{"USA", "US"},
		{"U.S.", "US"},
		{"UNITED STATES", "US"},
		{"AMERICAN", "US"},
		{"CA", "CA"},
	}
	for _, tt := range tests {
		if got := normalizeCitizenship(tt.input); got != tt.want {
			t.Errorf("normalizeCitizenship(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestExtractDemographics_NilOCR(t *testing.T) {
	img := createTestGrayImage(400, 400)
	_, err := ExtractDemographics(context.Background(), img, nil, DefaultFD258HeaderFields())
	if err == nil {
		t.Error("expected error for nil OCR provider")
	}
}

func TestExtractDemographics_TooSmall(t *testing.T) {
	img := createTestGrayImage(10, 10)
	ocr := &fixedOCR{text: "test"}
	_, err := ExtractDemographics(context.Background(), img, ocr, DefaultFD258HeaderFields())
	if err == nil {
		t.Error("expected error for small image")
	}
}

func TestExtractDemographics_OCRError(t *testing.T) {
	img := createTestGrayImage(4000, 4000)
	ocr := &fixedOCR{text: "", err: fmt.Errorf("service unavailable")}

	result, err := ExtractDemographics(context.Background(), img, ocr, DefaultFD258HeaderFields())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All fields should have warnings about OCR failure.
	if len(result.Warnings) == 0 {
		t.Error("expected warnings for OCR failures")
	}
}

func TestExtractDemographics_AllFields(t *testing.T) {
	img := createTestGrayImage(4000, 4000)

	// Fields are OCR'd in this order:
	// name, dob, sex, race, height, weight, eye_color, hair_color,
	// place_of_birth, citizenship, ssn, address
	ocr := &sequentialOCR{responses: []string{
		"Doe, John Michael",              // name
		"01/15/1990",                      // dob
		"M",                               // sex
		"W",                               // race
		"5'10\"",                          // height
		"180 lbs",                         // weight
		"BROWN",                           // eye_color
		"BLACK",                           // hair_color
		"VA",                              // place_of_birth
		"US",                              // citizenship
		"123-45-6789",                     // ssn
		"123 Main St, Anytown, VA 12345", // address
	}}

	result, err := ExtractDemographics(context.Background(), img, ocr, DefaultFD258HeaderFields())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	p := result.Person
	if p.LastName != "Doe" {
		t.Errorf("LastName = %q, want Doe", p.LastName)
	}
	if p.FirstName != "John" {
		t.Errorf("FirstName = %q, want John", p.FirstName)
	}
	if p.MiddleName != "Michael" {
		t.Errorf("MiddleName = %q, want Michael", p.MiddleName)
	}
	wantDOB := time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)
	if !p.DOB.Equal(wantDOB) {
		t.Errorf("DOB = %v, want %v", p.DOB, wantDOB)
	}
	if p.Sex != "M" {
		t.Errorf("Sex = %q, want M", p.Sex)
	}
	if p.Race != "W" {
		t.Errorf("Race = %q, want W", p.Race)
	}
	if p.Height != "510" {
		t.Errorf("Height = %q, want 510", p.Height)
	}
	if p.Weight != "180" {
		t.Errorf("Weight = %q, want 180", p.Weight)
	}
	if p.EyeColor != "BRO" {
		t.Errorf("EyeColor = %q, want BRO", p.EyeColor)
	}
	if p.HairColor != "BLK" {
		t.Errorf("HairColor = %q, want BLK", p.HairColor)
	}
	if p.PlaceOfBirth != "VA" {
		t.Errorf("PlaceOfBirth = %q, want VA", p.PlaceOfBirth)
	}
	if p.Citizenship != "US" {
		t.Errorf("Citizenship = %q, want US", p.Citizenship)
	}
	if p.SSN != "123456789" {
		t.Errorf("SSN = %q, want 123456789", p.SSN)
	}
	if p.Address != "123 Main St, Anytown, VA 12345" {
		t.Errorf("Address = %q", p.Address)
	}

	// Should have no warnings since all fields were extracted.
	if len(result.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	// All raw fields should be populated.
	if len(result.RawFields) != 12 {
		t.Errorf("RawFields has %d entries, want 12", len(result.RawFields))
	}
}

func TestMergeDemographics(t *testing.T) {
	base := ATFPersonInfo{
		LastName:  "Doe",
		FirstName: "John",
		Sex:       "M",
		Height:    "510",
	}
	override := ATFPersonInfo{
		FirstName: "Jonathan", // override
		Race:      "W",       // new field
	}

	merged := MergeDemographics(base, override)

	if merged.LastName != "Doe" {
		t.Errorf("LastName = %q, want Doe (kept from base)", merged.LastName)
	}
	if merged.FirstName != "Jonathan" {
		t.Errorf("FirstName = %q, want Jonathan (from override)", merged.FirstName)
	}
	if merged.Sex != "M" {
		t.Errorf("Sex = %q, want M (kept from base)", merged.Sex)
	}
	if merged.Race != "W" {
		t.Errorf("Race = %q, want W (from override)", merged.Race)
	}
	if merged.Height != "510" {
		t.Errorf("Height = %q, want 510 (kept from base)", merged.Height)
	}
}

func TestCropHeader(t *testing.T) {
	img := createTestGrayImage(4000, 4000)
	header := CropHeader(img)
	bounds := header.Bounds()

	// Header should be full width, ~36% height.
	if bounds.Dx() != 4000 {
		t.Errorf("header width = %d, want 4000", bounds.Dx())
	}
	expectedH := int(0.36 * 4000) // ~1440
	if bounds.Dy() < expectedH-10 || bounds.Dy() > expectedH+10 {
		t.Errorf("header height = %d, want ~%d", bounds.Dy(), expectedH)
	}
}

func TestCropHeaderField(t *testing.T) {
	img := createTestGrayImage(4000, 4000)
	fields := DefaultFD258HeaderFields()

	nameImg := CropHeaderField(img, fields.Name)
	if nameImg == nil {
		t.Fatal("CropHeaderField returned nil")
	}
	bounds := nameImg.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		t.Error("cropped field has zero dimension")
	}
}

func TestExtractDigits(t *testing.T) {
	tests := []struct{ input, want string }{
		{"123-45-6789", "123456789"},
		{"abc", ""},
		{"1a2b3c", "123"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := extractDigits(tt.input); got != tt.want {
			t.Errorf("extractDigits(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
