package eft

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"strings"
	"testing"
	"time"
)

func createTestGrayImage(w, h int) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	// Use random noise — WSQ requires non-trivial image content for
	// its quantization step. Uniform or smooth gradients cause quantization
	// block sizes of zero which the encoder rejects.
	rng := rand.New(rand.NewSource(42))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.Intn(256))
	}
	return img
}



func createTestPNG(w, h int) *bytes.Buffer {
	img := createTestGrayImage(w, h)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return &buf
}

func TestField_Tag(t *testing.T) {
	f := Field{RecordType: 14, FieldNumber: 13}
	if got := f.Tag(); got != "14.013" {
		t.Errorf("Tag() = %q, want %q", got, "14.013")
	}
	f = Field{RecordType: 1, FieldNumber: 1}
	if got := f.Tag(); got != "1.001" {
		t.Errorf("Tag() = %q, want %q", got, "1.001")
	}
}

func TestRecord_SetGetField(t *testing.T) {
	r := &Record{Type: 1}
	r.SetField(4, []byte("CAR"))

	got := r.GetField(4)
	if string(got) != "CAR" {
		t.Errorf("GetField(4) = %q, want %q", got, "CAR")
	}

	// Overwrite
	r.SetField(4, []byte("CNA"))
	got = r.GetField(4)
	if string(got) != "CNA" {
		t.Errorf("GetField(4) after overwrite = %q, want %q", got, "CNA")
	}

	if r.GetField(99) != nil {
		t.Error("GetField(99) should return nil")
	}
}

func TestRecord_Encode(t *testing.T) {
	r := &Record{Type: 1}
	r.SetField(2, []byte("0502"))
	r.SetField(4, []byte("CAR"))

	data, err := r.encode()
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	if !strings.Contains(s, "1.001:") {
		t.Error("encoded record missing LEN tag")
	}
	if !strings.Contains(s, "1.002:0502") {
		t.Error("encoded record missing VER field")
	}
	if !strings.Contains(s, "1.004:CAR") {
		t.Error("encoded record missing TOT field")
	}
	if data[len(data)-1] != FS {
		t.Errorf("record does not end with FS, got 0x%02x", data[len(data)-1])
	}
}

func TestRecord_Encode_LEN_SelfConsistent(t *testing.T) {
	r := &Record{Type: 1}
	r.SetField(2, []byte("0502"))
	r.SetField(4, []byte("CAR"))

	data, err := r.encode()
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	lenEnd := strings.Index(s, string([]byte{GS}))
	if lenEnd == -1 {
		lenEnd = strings.Index(s, string([]byte{FS}))
	}
	lenField := s[:lenEnd]
	parts := strings.SplitN(lenField, ":", 2)
	if len(parts) != 2 {
		t.Fatalf("cannot parse LEN field: %q", lenField)
	}

	var lenVal int
	for _, ch := range parts[1] {
		lenVal = lenVal*10 + int(ch-'0')
	}

	if lenVal != len(data) {
		t.Errorf("LEN field says %d but record is %d bytes", lenVal, len(data))
	}
}

func TestNewType1Record(t *testing.T) {
	date := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	r, err := NewType1Record(Type1Options{
		TransactionType:   "CAR",
		DestinationAgency: "WVFBI0000",
		OriginatingAgency: "WV1234567",
		ControlNumber:     "TCN20240115001",
		DomainName:        "NORAM",
		DomainVersion:     "11.1",
		Date:              date,
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(r.GetField(4)) != "CAR" {
		t.Error("TOT not set")
	}
	if string(r.GetField(5)) != "20240115" {
		t.Errorf("DAT = %q, want 20240115", r.GetField(5))
	}
	if string(r.GetField(7)) != "WVFBI0000" {
		t.Error("DAI not set")
	}
	if string(r.GetField(9)) != "TCN20240115001" {
		t.Error("TCN not set")
	}
}

func TestNewType1Record_Validation(t *testing.T) {
	_, err := NewType1Record(Type1Options{})
	if err == nil {
		t.Error("expected error for empty options")
	}
}

func TestNewType14Record(t *testing.T) {
	imgData := make([]byte, 100*100)
	r, err := NewType14Record(Type14Options{
		IDC:                  1,
		ImpressionType:       ImpressionLiveScanRolled,
		SourceAgency:         "WV1234567",
		CaptureDate:          "20240115",
		HorizontalLineLength: 100,
		VerticalLineLength:   100,
		HorizontalPixelScale: 500,
		VerticalPixelScale:   500,
		Compression:          CompressionNone,
		BitsPerPixel:         8,
		FingerPosition:       FingerRightIndex,
		ImageData:            imgData,
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(r.GetField(3)) != "1" {
		t.Errorf("IMP = %q, want 1", r.GetField(3))
	}
	if string(r.GetField(13)) != "2" {
		t.Errorf("FGP = %q, want 2", r.GetField(13))
	}
	if !bytes.Equal(r.GetField(999), imgData) {
		t.Error("image data mismatch")
	}
}

func TestNewType14Record_Validation(t *testing.T) {
	_, err := NewType14Record(Type14Options{})
	if err == nil {
		t.Error("expected error for empty options")
	}
}

func TestNewType4Record(t *testing.T) {
	img := createTestGrayImage(100, 100)

	rec, rawBytes, err := NewType4Record(Type4Options{
		IDC:            1,
		ImpressionType: ImpressionNonLiveRolled,
		FingerPosition: FingerRightThumb,
		Image:          img,
		Compressor:     &NoneCompressor{},
	})
	if err != nil {
		t.Fatal(err)
	}

	if rec.Type != 4 {
		t.Errorf("record type = %d, want 4", rec.Type)
	}
	if rawBytes == nil {
		t.Fatal("rawBytes is nil")
	}

	// Verify binary header: first 4 bytes = total length
	totalLen := int(rawBytes[0])<<24 | int(rawBytes[1])<<16 | int(rawBytes[2])<<8 | int(rawBytes[3])
	if totalLen != len(rawBytes) {
		t.Errorf("LEN = %d, actual = %d", totalLen, len(rawBytes))
	}

	// Verify IDC
	if rawBytes[4] != 1 {
		t.Errorf("IDC = %d, want 1", rawBytes[4])
	}

	// Verify finger position
	if rawBytes[6] != 1 {
		t.Errorf("FGP = %d, want 1", rawBytes[6])
	}

	// Image data should be at offset 18
	expectedImgSize := 100 * 100
	if len(rawBytes)-18 != expectedImgSize {
		t.Errorf("image data size = %d, want %d", len(rawBytes)-18, expectedImgSize)
	}
}

func TestWSQCompressor(t *testing.T) {
	// WSQ requires images large enough for its wavelet decomposition.
	// Minimum practical size is around 500x500.
	// WSQ requires images with dimensions compatible with its wavelet
	// decomposition. Typical fingerprint images are 800x750 at 500 ppi.
	img := createTestGrayImage(800, 750)
	comp := &WSQCompressor{Bitrate: 0.75}

	data, err := comp.Compress(img)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("WSQ compressed data is empty")
	}

	rawSize := 800 * 750
	if len(data) >= rawSize {
		t.Errorf("WSQ data (%d bytes) not smaller than raw (%d bytes)", len(data), rawSize)
	}

	if comp.Algorithm() != CompressionWSQ {
		t.Errorf("Algorithm() = %q, want %q", comp.Algorithm(), CompressionWSQ)
	}
}

func TestNoneCompressor(t *testing.T) {
	img := createTestGrayImage(50, 50)
	comp := &NoneCompressor{}

	data, err := comp.Compress(img)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 50*50 {
		t.Errorf("raw data size = %d, want %d", len(data), 50*50)
	}

	if comp.Algorithm() != CompressionNone {
		t.Errorf("Algorithm() = %q, want %q", comp.Algorithm(), CompressionNone)
	}
}

func TestTransaction_Encode(t *testing.T) {
	type1, _ := NewType1Record(Type1Options{
		TransactionType:   "CAR",
		DestinationAgency: "WVFBI0000",
		OriginatingAgency: "WV1234567",
		ControlNumber:     "TCN20240115001",
	})
	type2 := NewType2Record(Type2Options{IDC: 0})

	imgData := make([]byte, 50*50)
	type14, _ := NewType14Record(Type14Options{
		IDC:                  1,
		ImpressionType:       ImpressionLiveScanRolled,
		SourceAgency:         "WV1234567",
		CaptureDate:          "20240115",
		HorizontalLineLength: 50,
		VerticalLineLength:   50,
		Compression:          CompressionNone,
		FingerPosition:       FingerRightThumb,
		ImageData:            imgData,
	})

	txn := &Transaction{}
	txn.AddRecord(type1)
	txn.AddRecord(type2)
	txn.AddRecord(type14)

	data, err := txn.Encode()
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("encoded transaction is empty")
	}

	s := string(data)
	if !strings.Contains(s, "1.003:") {
		t.Error("encoded transaction missing CNT field")
	}
}

func TestTransaction_WithType4(t *testing.T) {
	type1, _ := NewType1Record(Type1Options{
		TransactionType:   "FAUF",
		DestinationAgency: ATFDestinationAgency,
		OriginatingAgency: ATFOriginatingAgency,
		ControlNumber:     "TCN20240115001",
	})
	type2 := NewType2Record(Type2Options{IDC: 0})

	img := createTestGrayImage(100, 100)
	type4, _, err := NewType4Record(Type4Options{
		IDC:            1,
		ImpressionType: ImpressionNonLiveRolled,
		FingerPosition: FingerRightThumb,
		Image:          img,
		Compressor:     &NoneCompressor{},
	})
	if err != nil {
		t.Fatal(err)
	}

	txn := &Transaction{}
	txn.AddRecord(type1)
	txn.AddRecord(type2)
	txn.AddRecord(type4)

	data, err := txn.Encode()
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("encoded transaction is empty")
	}
}

func TestCreateTransaction(t *testing.T) {
	imgBuf := createTestPNG(100, 100)

	data, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "CAR",
			DestinationAgency: "WVFBI0000",
			OriginatingAgency: "WV1234567",
			ControlNumber:     "TCN20240115001",
			Date:              time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		[]FingerprintImage{
			{
				FingerPosition: FingerRightIndex,
				ImpressionType: ImpressionLiveScanPlain,
				Reader:         imgBuf,
				Compressor:     &NoneCompressor{},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("encoded transaction is empty")
	}

	if len(data) < 100*100 {
		t.Errorf("encoded data too small: %d bytes", len(data))
	}
}

func TestCreateTransaction_WSQ(t *testing.T) {
	imgBuf := createTestPNG(800, 750)

	data, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "FAUF",
			DestinationAgency: ATFDestinationAgency,
			OriginatingAgency: ATFOriginatingAgency,
			ControlNumber:     "TCN20240115001",
			Date:              time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		},
		[]FingerprintImage{
			{
				FingerPosition: FingerRightIndex,
				ImpressionType: ImpressionLiveScanPlain,
				Reader:         imgBuf,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("encoded transaction is empty")
	}

	// WSQ-compressed should be smaller than raw
	if len(data) >= 800*750 {
		t.Errorf("WSQ transaction (%d bytes) should be smaller than raw image", len(data))
	}
}

func TestCreateTransaction_NoImages(t *testing.T) {
	_, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "CAR",
			DestinationAgency: "WVFBI0000",
			OriginatingAgency: "WV1234567",
			ControlNumber:     "TCN001",
		},
		nil,
	)
	if err == nil {
		t.Error("expected error for no images")
	}
}

func TestCropFD258(t *testing.T) {
	// Create a large test image simulating a card scan.
	img := createTestGrayImage(4000, 4000)
	layout := DefaultFD258Layout()

	images, err := CropFD258(img, layout)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all 10 rolled prints were extracted.
	for i := range 10 {
		if images.Rolled[i] == nil {
			t.Errorf("rolled print %d is nil", i+1)
			continue
		}
		bounds := images.Rolled[i].Bounds()
		if bounds.Dx() == 0 || bounds.Dy() == 0 {
			t.Errorf("rolled print %d has zero dimension", i+1)
		}
	}

	// Verify flat prints.
	if images.FlatRight == nil {
		t.Error("flat right is nil")
	}
	if images.FlatLeft == nil {
		t.Error("flat left is nil")
	}
	if images.FlatThumbs == nil {
		t.Error("flat thumbs is nil")
	}
}

func TestCropFD258_TooSmall(t *testing.T) {
	img := createTestGrayImage(10, 10)
	_, err := CropFD258(img, DefaultFD258Layout())
	if err == nil {
		t.Error("expected error for small image")
	}
}

func TestCreateATFTransaction(t *testing.T) {
	// Create a simulated card scan (small for fast tests).
	cardImg := createTestGrayImage(1000, 1000)
	var cardBuf bytes.Buffer
	if err := png.Encode(&cardBuf, cardImg); err != nil {
		t.Fatal(err)
	}

	data, err := CreateATFTransaction(&cardBuf, ATFSubmissionOptions{
		Person: ATFPersonInfo{
			LastName:  "Doe",
			FirstName: "John",
			DOB:       time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			Sex:       "M",
			Race:      "W",
		},
		Date:       time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Compressor: &NoneCompressor{}, // use none for fast tests
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("ATF transaction is empty")
	}

	// Verify ATF-specific fields are in the output.
	s := string(data)
	if !strings.Contains(s, "1.004:FAUF") {
		t.Error("missing TOT FAUF")
	}
	if !strings.Contains(s, "1.002:0200") {
		t.Error("missing VER 0200")
	}
	if !strings.Contains(s, ATFDestinationAgency) {
		t.Error("missing DAI")
	}
	if !strings.Contains(s, "Doe,John") {
		t.Error("missing name in Type-2")
	}
	if !strings.Contains(s, "Firearms") {
		t.Error("missing RFP Firearms")
	}
}

func TestCreateATFTransaction_Validation(t *testing.T) {
	cardBuf := createTestPNG(100, 100)

	_, err := CreateATFTransaction(cardBuf, ATFSubmissionOptions{
		Person: ATFPersonInfo{},
	})
	if err == nil {
		t.Error("expected error for empty person info")
	}
}

func TestATFType2Fields(t *testing.T) {
	fields := buildATFType2Fields(ATFPersonInfo{
		LastName:     "Smith",
		FirstName:    "Jane",
		MiddleName:   "Q",
		DOB:          time.Date(1985, 6, 15, 0, 0, 0, 0, time.UTC),
		Sex:          "F",
		Race:         "W",
		PlaceOfBirth: "VA",
		Citizenship:  "US",
		Height:       "507",
		Weight:       "130",
		EyeColor:     "BRO",
		HairColor:    "BLK",
		SSN:          "123456789",
	}, "20240115")

	if string(fields[18]) != "Smith,Jane Q" {
		t.Errorf("NAM = %q, want %q", fields[18], "Smith,Jane Q")
	}
	if string(fields[22]) != "19850615" {
		t.Errorf("DOB = %q, want 19850615", fields[22])
	}
	if string(fields[37]) != "Firearms" {
		t.Errorf("RFP = %q, want Firearms", fields[37])
	}
	if string(fields[73]) != ATFOriginatingAgency {
		t.Errorf("CRI = %q, want %q", fields[73], ATFOriginatingAgency)
	}
	if string(fields[16]) != "123456789" {
		t.Errorf("SOC = %q, want 123456789", fields[16])
	}
}

func TestImageToGray(t *testing.T) {
	// Test with RGBA image.
	rgba := image.NewRGBA(image.Rect(0, 0, 2, 2))
	rgba.Set(0, 0, color.White)
	rgba.Set(1, 0, color.Black)

	gray := imageToGray(rgba)
	if gray.GrayAt(0, 0).Y != 255 {
		t.Errorf("white pixel = %d, want 255", gray.GrayAt(0, 0).Y)
	}
	if gray.GrayAt(1, 0).Y != 0 {
		t.Errorf("black pixel = %d, want 0", gray.GrayAt(1, 0).Y)
	}

	// Test passthrough with Gray image.
	orig := image.NewGray(image.Rect(0, 0, 5, 5))
	result := imageToGray(orig)
	if result != orig {
		t.Error("imageToGray should return same pointer for *image.Gray input")
	}
}
