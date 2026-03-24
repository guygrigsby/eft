package eft

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "update golden files in testdata/")

// goldenPath returns the path to a golden file in testdata/.
func goldenPath(name string) string {
	return filepath.Join("testdata", name)
}

// checkGolden compares data against a golden file, or updates it if -update is set.
func checkGolden(t *testing.T, name string, data []byte) {
	t.Helper()
	path := goldenPath(name)

	if *update {
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("updating golden file %s: %v", path, err)
		}
		t.Logf("updated golden file: %s (%d bytes)", path, len(data))
		return
	}

	golden, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s: %v (run with -update to generate)", path, err)
	}

	if !bytes.Equal(data, golden) {
		t.Errorf("output does not match golden file %s", path)
		t.Errorf("  got:  %d bytes", len(data))
		t.Errorf("  want: %d bytes", len(golden))

		// Find first divergence point for debugging.
		minLen := len(data)
		if len(golden) < minLen {
			minLen = len(golden)
		}
		for i := 0; i < minLen; i++ {
			if data[i] != golden[i] {
				// Show context around the divergence.
				start := i - 16
				if start < 0 {
					start = 0
				}
				end := i + 16
				if end > minLen {
					end = minLen
				}
				t.Errorf("  first diff at byte %d: got 0x%02x, want 0x%02x", i, data[i], golden[i])
				t.Errorf("  got  context: %q", data[start:end])
				t.Errorf("  want context: %q", golden[start:end])
				break
			}
		}
	}
}

// ---- Deterministic test data generators ----

// deterministicGray creates a deterministic grayscale image using a fixed seed.
func deterministicGray(w, h int, seed int64) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(seed))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.Intn(256))
	}
	return img
}

// deterministicPNG encodes a deterministic grayscale image as PNG.
func deterministicPNG(w, h int, seed int64) *bytes.Buffer {
	img := deterministicGray(w, h, seed)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return &buf
}

// fixedDate is the deterministic date used across golden tests.
var fixedDate = time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)

// ---- Golden file tests: full EFT transaction outputs ----

func TestGolden_SimpleType14Transaction(t *testing.T) {
	imgBuf := deterministicPNG(100, 100, 100)

	data, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "CAR",
			DestinationAgency: "WVFBI0000",
			OriginatingAgency: "WV1234567",
			ControlNumber:     "TCN20240615001",
			Date:              fixedDate,
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

	checkGolden(t, "simple_type14.eft", data)
}

func TestGolden_MultiFingerType14(t *testing.T) {
	fingers := []struct {
		pos  FingerPosition
		imp  ImpressionType
		seed int64
	}{
		{FingerRightThumb, ImpressionLiveScanRolled, 201},
		{FingerRightIndex, ImpressionLiveScanRolled, 202},
		{FingerRightMiddle, ImpressionLiveScanRolled, 203},
		{FingerLeftThumb, ImpressionLiveScanRolled, 204},
		{FingerLeftIndex, ImpressionLiveScanRolled, 205},
	}

	var images []FingerprintImage
	for _, f := range fingers {
		images = append(images, FingerprintImage{
			FingerPosition: f.pos,
			ImpressionType: f.imp,
			Reader:         deterministicPNG(100, 100, f.seed),
			Compressor:     &NoneCompressor{},
		})
	}

	data, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "CNA",
			DestinationAgency: "WVFBI0000",
			OriginatingAgency: "WV1234567",
			ControlNumber:     "TCN20240615002",
			DomainName:        "NORAM",
			DomainVersion:     "11.1",
			Date:              fixedDate,
		},
		images,
	)
	if err != nil {
		t.Fatal(err)
	}

	checkGolden(t, "multi_finger_type14.eft", data)
}

func TestGolden_Type4Transaction(t *testing.T) {
	type1, err := NewType1Record(Type1Options{
		TransactionType:   "FAUF",
		DestinationAgency: ATFDestinationAgency,
		OriginatingAgency: ATFOriginatingAgency,
		ControlNumber:     "TCN20240615003",
		Date:              fixedDate,
	})
	if err != nil {
		t.Fatal(err)
	}

	type2 := NewType2Record(Type2Options{
		IDC: 0,
		Fields: map[int][]byte{
			18: []byte("Doe,John"),
			22: []byte("19900101"),
			24: []byte("M"),
		},
	})

	img := deterministicGray(100, 100, 300)
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

	checkGolden(t, "type4_transaction.eft", data)
}

func TestGolden_MixedType4Type14(t *testing.T) {
	type1, err := NewType1Record(Type1Options{
		TransactionType:   "FAUF",
		DestinationAgency: ATFDestinationAgency,
		OriginatingAgency: ATFOriginatingAgency,
		ControlNumber:     "TCN20240615004",
		Date:              fixedDate,
	})
	if err != nil {
		t.Fatal(err)
	}
	type1.SetField(2, []byte(ATFVersion))

	type2 := NewType2Record(Type2Options{IDC: 0})

	// Type-4 rolled print
	rolledImg := deterministicGray(100, 100, 400)
	type4, _, err := NewType4Record(Type4Options{
		IDC:            1,
		ImpressionType: ImpressionNonLiveRolled,
		FingerPosition: FingerRightThumb,
		Image:          rolledImg,
		Compressor:     &NoneCompressor{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Type-14 flat print
	flatImgData := make([]byte, 80*60)
	rng := rand.New(rand.NewSource(401))
	for i := range flatImgData {
		flatImgData[i] = uint8(rng.Intn(256))
	}
	type14, err := NewType14Record(Type14Options{
		IDC:                  2,
		ImpressionType:       ImpressionNonLivePlain,
		SourceAgency:         ATFOriginatingAgency,
		CaptureDate:          "20240615",
		HorizontalLineLength: 80,
		VerticalLineLength:   60,
		Compression:          CompressionNone,
		FingerPosition:       FingerRightFourFingers,
		ImageData:            flatImgData,
	})
	if err != nil {
		t.Fatal(err)
	}

	txn := &Transaction{}
	txn.AddRecord(type1)
	txn.AddRecord(type2)
	txn.AddRecord(type4)
	txn.AddRecord(type14)

	data, err := txn.Encode()
	if err != nil {
		t.Fatal(err)
	}

	checkGolden(t, "mixed_type4_type14.eft", data)
}

func TestGolden_ATFFromPreCroppedImages(t *testing.T) {
	images := &FD258Images{}

	// Generate deterministic rolled prints (10 fingers).
	for i := 0; i < 10; i++ {
		images.Rolled[i] = deterministicGray(100, 100, int64(500+i))
	}
	// Generate deterministic flat prints.
	images.FlatRight = deterministicGray(200, 100, 510)
	images.FlatLeft = deterministicGray(200, 100, 511)
	images.FlatThumbs = deterministicGray(130, 100, 512)

	data, err := CreateATFTransactionFromImages(images, ATFSubmissionOptions{
		Person: ATFPersonInfo{
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
		},
		ControlNumber: "ATFEFT20240615103000",
		Date:          fixedDate,
		Compressor:    &NoneCompressor{},
	})
	if err != nil {
		t.Fatal(err)
	}

	checkGolden(t, "atf_precropped.eft", data)
}

func TestGolden_ATFFromCardScan(t *testing.T) {
	// Use a small card for speed (NoneCompressor keeps size manageable).
	cardImg := deterministicGray(1000, 1000, 600)
	var cardBuf bytes.Buffer
	if err := png.Encode(&cardBuf, cardImg); err != nil {
		t.Fatal(err)
	}

	data, err := CreateATFTransaction(&cardBuf, ATFSubmissionOptions{
		Person: ATFPersonInfo{
			LastName:  "Doe",
			FirstName: "John",
			DOB:       time.Date(1990, 3, 20, 0, 0, 0, 0, time.UTC),
			Sex:       "M",
			Race:      "W",
		},
		ControlNumber: "ATFEFT20240615103001",
		Date:          fixedDate,
		Compressor:    &NoneCompressor{},
	})
	if err != nil {
		t.Fatal(err)
	}

	checkGolden(t, "atf_card_scan.eft", data)
}

func TestGolden_MinimalTransaction(t *testing.T) {
	// Minimal valid transaction: Type-1 + Type-2 + one tiny Type-14.
	imgData := make([]byte, 10*10)
	for i := range imgData {
		imgData[i] = 0x80 // uniform mid-gray
	}

	data, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "CAR",
			DestinationAgency: "AAAAAAAAA",
			OriginatingAgency: "BBBBBBBBB",
			ControlNumber:     "TCN0000001",
			Date:              fixedDate,
			Version:           "0200",
		},
		[]FingerprintImage{
			{
				FingerPosition: FingerRightThumb,
				ImpressionType: ImpressionLiveScanPlain,
				Reader:         pngFromGrayPixels(10, 10, imgData),
				Compressor:     &NoneCompressor{},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	checkGolden(t, "minimal.eft", data)
}

func TestGolden_AllDemographicFields(t *testing.T) {
	// Transaction with maximum demographic data in Type-2.
	imgData := make([]byte, 20*20)
	rng := rand.New(rand.NewSource(700))
	for i := range imgData {
		imgData[i] = uint8(rng.Intn(256))
	}

	data, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "FAUF",
			DestinationAgency: ATFDestinationAgency,
			OriginatingAgency: ATFOriginatingAgency,
			ControlNumber:     "TCN20240615005",
			Date:              fixedDate,
			DemographicFields: map[int][]byte{
				5:  []byte("N"),
				16: []byte("987654321"),
				18: []byte("Johnson,Robert K"),
				20: []byte("NY"),
				21: []byte("US"),
				22: []byte("19750815"),
				24: []byte("M"),
				25: []byte("W"),
				27: []byte("600"),
				29: []byte("185"),
				31: []byte("BRO"),
				32: []byte("BLN"),
				37: []byte("Firearms"),
				38: []byte("20240615"),
				41: []byte("123 Main St, Springfield, VA 22150"),
				73: []byte(ATFOriginatingAgency),
			},
		},
		[]FingerprintImage{
			{
				FingerPosition: FingerRightIndex,
				ImpressionType: ImpressionLiveScanPlain,
				Reader:         pngFromGrayPixels(20, 20, imgData),
				Compressor:     &NoneCompressor{},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	checkGolden(t, "all_demographics.eft", data)
}

func TestGolden_DomainField(t *testing.T) {
	// Transaction with DOM field set.
	imgData := make([]byte, 20*20)
	data, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "CAR",
			DestinationAgency: "WVFBI0000",
			OriginatingAgency: "WV1234567",
			ControlNumber:     "TCN20240615006",
			DomainName:        "NORAM",
			DomainVersion:     "11.1",
			Date:              fixedDate,
		},
		[]FingerprintImage{
			{
				FingerPosition: FingerRightThumb,
				ImpressionType: ImpressionLiveScanRolled,
				Reader:         pngFromGrayPixels(20, 20, imgData),
				Compressor:     &NoneCompressor{},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	checkGolden(t, "with_domain.eft", data)
}

// ---- Structural validation tests (parse generated EFT and verify structure) ----

func TestStructure_RecordSeparation(t *testing.T) {
	// Verify that tagged records end with FS separator.
	r := &Record{Type: 2}
	r.SetField(2, []byte("00"))
	r.SetField(18, []byte("Doe,John"))

	data, err := r.encode()
	if err != nil {
		t.Fatal(err)
	}

	if data[len(data)-1] != FS {
		t.Errorf("record does not end with FS (0x1C), got 0x%02x", data[len(data)-1])
	}
}

func TestStructure_FieldSeparation(t *testing.T) {
	// Verify GS separates fields within a record.
	r := &Record{Type: 2}
	r.SetField(2, []byte("00"))
	r.SetField(18, []byte("Doe,John"))
	r.SetField(24, []byte("M"))

	data, err := r.encode()
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	// Count GS separators — should be one fewer than the number of fields (including LEN).
	gsCount := strings.Count(s, string([]byte{GS}))
	// 3 fields total (LEN, IDC, NAM, SEX) → 3 GS separators between them.
	if gsCount != 3 {
		t.Errorf("expected 3 GS separators, got %d", gsCount)
	}
}

func TestStructure_FieldTagFormat(t *testing.T) {
	// Verify field tags use "TT.FFF:" format.
	r := &Record{Type: 14}
	r.SetField(2, []byte("01"))
	r.SetField(3, []byte("0"))
	r.SetField(13, []byte("2"))

	data, err := r.encode()
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	if !strings.Contains(s, "14.001:") {
		t.Error("missing LEN tag 14.001:")
	}
	if !strings.Contains(s, "14.002:") {
		t.Error("missing IDC tag 14.002:")
	}
	if !strings.Contains(s, "14.003:") {
		t.Error("missing IMP tag 14.003:")
	}
	if !strings.Contains(s, "14.013:") {
		t.Error("missing FGP tag 14.013:")
	}
}

func TestStructure_FieldOrdering(t *testing.T) {
	// Verify fields are encoded in ascending field number order.
	r := &Record{Type: 2}
	// Insert in reverse order.
	r.SetField(73, []byte("ORI123456"))
	r.SetField(24, []byte("M"))
	r.SetField(18, []byte("Doe,John"))
	r.SetField(5, []byte("N"))
	r.SetField(2, []byte("00"))

	data, err := r.encode()
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	// Extract field tag positions.
	tags := []string{"2.001:", "2.002:", "2.005:", "2.018:", "2.024:", "2.073:"}
	lastPos := -1
	for _, tag := range tags {
		pos := strings.Index(s, tag)
		if pos == -1 {
			t.Errorf("missing tag %s in encoded record", tag)
			continue
		}
		if pos <= lastPos {
			t.Errorf("tag %s at position %d is not after previous tag at %d", tag, pos, lastPos)
		}
		lastPos = pos
	}
}

func TestStructure_LENConsistency(t *testing.T) {
	// Verify LEN field matches actual byte count for various record sizes.
	testCases := []struct {
		name   string
		fields map[int][]byte
		rtype  int
	}{
		{
			name:   "small_type2",
			rtype:  2,
			fields: map[int][]byte{2: []byte("00")},
		},
		{
			name:  "medium_type2",
			rtype: 2,
			fields: map[int][]byte{
				2:  []byte("00"),
				18: []byte("VeryLongLastNameThatIsQuiteExtended,FirstName MiddleName"),
				24: []byte("M"),
				25: []byte("W"),
			},
		},
		{
			name:  "type14_with_data",
			rtype: 14,
			fields: map[int][]byte{
				2:   []byte("01"),
				3:   []byte("0"),
				4:   []byte("WV1234567"),
				5:   []byte("20240615"),
				6:   []byte("50"),
				7:   []byte("50"),
				8:   []byte("1"),
				9:   []byte("500"),
				10:  []byte("500"),
				11:  []byte("NONE"),
				12:  []byte("8"),
				13:  []byte("2"),
				999: make([]byte, 2500),
			},
		},
		{
			name:  "type1_header",
			rtype: 1,
			fields: map[int][]byte{
				2:  []byte("0502"),
				3:  []byte("0"),
				4:  []byte("FAUF"),
				5:  []byte("20240615"),
				6:  []byte("4"),
				7:  []byte("WVIAFIS0Z"),
				8:  []byte("WVATF0800"),
				9:  []byte("TCN20240615001"),
				11: []byte("19.69"),
				12: []byte("19.69"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Record{Type: tc.rtype}
			for num, val := range tc.fields {
				r.SetField(num, val)
			}

			data, err := r.encode()
			if err != nil {
				t.Fatal(err)
			}

			// Parse LEN from the encoded data.
			lenEnd := bytes.IndexByte(data, GS)
			if lenEnd == -1 {
				lenEnd = bytes.IndexByte(data, FS)
			}
			if lenEnd == -1 {
				t.Fatal("cannot find end of LEN field")
			}

			lenField := string(data[:lenEnd])
			parts := strings.SplitN(lenField, ":", 2)
			if len(parts) != 2 {
				t.Fatalf("cannot parse LEN field: %q", lenField)
			}

			lenVal, err := strconv.Atoi(parts[1])
			if err != nil {
				t.Fatalf("LEN value not numeric: %q", parts[1])
			}

			if lenVal != len(data) {
				t.Errorf("LEN says %d but record is %d bytes", lenVal, len(data))
			}
		})
	}
}

func TestStructure_TransactionCNT(t *testing.T) {
	// Verify CNT field format: count{RS}type{US}IDC for each record.
	type1, _ := NewType1Record(Type1Options{
		TransactionType:   "FAUF",
		DestinationAgency: ATFDestinationAgency,
		OriginatingAgency: ATFOriginatingAgency,
		ControlNumber:     "TCN20240615CNT",
		Date:              fixedDate,
	})
	type2 := NewType2Record(Type2Options{IDC: 0})

	imgData := make([]byte, 20*20)
	type14a, _ := NewType14Record(Type14Options{
		IDC:                  1,
		ImpressionType:       ImpressionLiveScanRolled,
		SourceAgency:         ATFOriginatingAgency,
		CaptureDate:          "20240615",
		HorizontalLineLength: 20,
		VerticalLineLength:   20,
		Compression:          CompressionNone,
		FingerPosition:       FingerRightThumb,
		ImageData:            imgData,
	})
	type14b, _ := NewType14Record(Type14Options{
		IDC:                  2,
		ImpressionType:       ImpressionLiveScanRolled,
		SourceAgency:         ATFOriginatingAgency,
		CaptureDate:          "20240615",
		HorizontalLineLength: 20,
		VerticalLineLength:   20,
		Compression:          CompressionNone,
		FingerPosition:       FingerRightIndex,
		ImageData:            imgData,
	})

	txn := &Transaction{}
	txn.AddRecord(type1)
	txn.AddRecord(type2)
	txn.AddRecord(type14a)
	txn.AddRecord(type14b)

	data, err := txn.Encode()
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)
	cntIdx := strings.Index(s, "1.003:")
	if cntIdx == -1 {
		t.Fatal("CNT field not found")
	}

	// Extract CNT value (between "1.003:" and next GS or FS).
	cntStart := cntIdx + len("1.003:")
	cntEnd := cntStart
	for cntEnd < len(s) && s[cntEnd] != GS && s[cntEnd] != FS {
		cntEnd++
	}
	cntVal := s[cntStart:cntEnd]

	// CNT should be "4{RS}1{US}0{RS}2{US}00{RS}14{US}01{RS}14{US}02"
	// (4 records total, then type/IDC pairs)
	subfields := strings.Split(cntVal, string([]byte{RS}))
	if len(subfields) != 5 { // count + 4 type/IDC pairs
		t.Errorf("CNT has %d subfields, want 5; value=%q", len(subfields), cntVal)
	}

	if subfields[0] != "4" {
		t.Errorf("CNT record count = %q, want 4", subfields[0])
	}

	// Verify each type/IDC pair.
	expectedPairs := []struct {
		recType string
		idc     string
	}{
		{"1", "0"},
		{"2", "00"},
		{"14", "01"},
		{"14", "02"},
	}
	for i, exp := range expectedPairs {
		if i+1 >= len(subfields) {
			t.Errorf("missing CNT subfield %d", i+1)
			continue
		}
		pair := strings.Split(subfields[i+1], string([]byte{US}))
		if len(pair) != 2 {
			t.Errorf("CNT pair %d: expected 2 parts, got %d: %q", i, len(pair), subfields[i+1])
			continue
		}
		if pair[0] != exp.recType || pair[1] != exp.idc {
			t.Errorf("CNT pair %d: got %s/%s, want %s/%s", i, pair[0], pair[1], exp.recType, exp.idc)
		}
	}
}

func TestStructure_Type4BinaryHeader(t *testing.T) {
	// Thoroughly verify the 18-byte binary header of a Type-4 record.
	img := deterministicGray(150, 120, 800)
	_, raw, err := NewType4Record(Type4Options{
		IDC:            3,
		ImpressionType: ImpressionNonLiveRolled,
		FingerPosition: FingerLeftIndex,
		Image:          img,
		Compressor:     &NoneCompressor{},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Total length (4 bytes big-endian).
	totalLen := binary.BigEndian.Uint32(raw[0:4])
	if int(totalLen) != len(raw) {
		t.Errorf("LEN: got %d, actual %d", totalLen, len(raw))
	}
	expectedLen := 18 + 150*120
	if int(totalLen) != expectedLen {
		t.Errorf("LEN: got %d, expected %d (18 header + %d pixels)", totalLen, expectedLen, 150*120)
	}

	// IDC.
	if raw[4] != 3 {
		t.Errorf("IDC: got %d, want 3", raw[4])
	}

	// IMP (impression type).
	if raw[5] != byte(ImpressionNonLiveRolled) {
		t.Errorf("IMP: got %d, want %d", raw[5], ImpressionNonLiveRolled)
	}

	// FGP[0] = finger position, FGP[1-5] = 0xFF.
	if raw[6] != byte(FingerLeftIndex) {
		t.Errorf("FGP[0]: got %d, want %d", raw[6], FingerLeftIndex)
	}
	for i := 1; i <= 5; i++ {
		if raw[6+i] != 0xFF {
			t.Errorf("FGP[%d]: got 0x%02x, want 0xFF", i, raw[6+i])
		}
	}

	// ISR (0 = 500 ppi).
	if raw[12] != 0 {
		t.Errorf("ISR: got %d, want 0", raw[12])
	}

	// HLL (2 bytes big-endian).
	hll := binary.BigEndian.Uint16(raw[13:15])
	if hll != 150 {
		t.Errorf("HLL: got %d, want 150", hll)
	}

	// VLL (2 bytes big-endian).
	vll := binary.BigEndian.Uint16(raw[15:17])
	if vll != 120 {
		t.Errorf("VLL: got %d, want 120", vll)
	}

	// GCA (compression algorithm).
	if raw[17] != 0 { // 0 = no compression
		t.Errorf("GCA: got %d, want 0 (none)", raw[17])
	}

	// Image data.
	imgDataLen := len(raw) - 18
	if imgDataLen != 150*120 {
		t.Errorf("image data: got %d bytes, want %d", imgDataLen, 150*120)
	}
}

func TestStructure_Type4WSQCompression(t *testing.T) {
	// Verify Type-4 GCA byte is 1 for WSQ.
	img := deterministicGray(800, 750, 801)
	_, raw, err := NewType4Record(Type4Options{
		IDC:            1,
		ImpressionType: ImpressionNonLiveRolled,
		FingerPosition: FingerRightThumb,
		Image:          img,
		Compressor:     &WSQCompressor{Bitrate: 0.75},
	})
	if err != nil {
		t.Fatal(err)
	}

	if raw[17] != 1 { // 1 = WSQ
		t.Errorf("GCA: got %d, want 1 (WSQ)", raw[17])
	}

	// WSQ data should be smaller than raw.
	imgDataLen := len(raw) - 18
	rawSize := 800 * 750
	if imgDataLen >= rawSize {
		t.Errorf("WSQ data (%d bytes) not smaller than raw (%d bytes)", imgDataLen, rawSize)
	}
}

func TestStructure_Type14Fields(t *testing.T) {
	// Verify all expected fields are present with correct values in a Type-14 record.
	imgData := make([]byte, 60*40)
	rec, err := NewType14Record(Type14Options{
		IDC:                  5,
		ImpressionType:       ImpressionLiveScanPlain,
		SourceAgency:         "TESTAGENCY",
		CaptureDate:          "20240615",
		HorizontalLineLength: 60,
		VerticalLineLength:   40,
		ScaleUnits:           ScaleUnitsPPCM,
		HorizontalPixelScale: 1969,
		VerticalPixelScale:   1969,
		Compression:          CompressionNone,
		BitsPerPixel:         8,
		FingerPosition:       FingerLeftMiddle,
		ImageData:            imgData,
	})
	if err != nil {
		t.Fatal(err)
	}

	checks := map[int]string{
		2:  "05",                    // IDC
		3:  "0",                     // IMP (livescan plain)
		4:  "TESTAGENCY",           // SRC
		5:  "20240615",             // FCD
		6:  "60",                   // HLL
		7:  "40",                   // VLL
		8:  "2",                    // SLC (PPCM)
		9:  "1969",                // HPS
		10: "1969",                // VPS
		11: "NONE",                // CGA
		12: "8",                   // BPX
		13: "8",                   // FGP (LeftMiddle)
	}

	for fieldNum, expected := range checks {
		got := string(rec.GetField(fieldNum))
		if got != expected {
			t.Errorf("field %d.%03d: got %q, want %q", rec.Type, fieldNum, got, expected)
		}
	}

	// Verify image data field.
	if !bytes.Equal(rec.GetField(999), imgData) {
		t.Error("image data mismatch in field 14.999")
	}
}

func TestStructure_ATFConstants(t *testing.T) {
	// Verify ATF-specific fields appear correctly in the output.
	images := &FD258Images{}
	for i := 0; i < 10; i++ {
		images.Rolled[i] = deterministicGray(50, 50, int64(900+i))
	}
	images.FlatRight = deterministicGray(100, 50, 910)
	images.FlatLeft = deterministicGray(100, 50, 911)
	images.FlatThumbs = deterministicGray(70, 50, 912)

	data, err := CreateATFTransactionFromImages(images, ATFSubmissionOptions{
		Person: ATFPersonInfo{
			LastName:  "Test",
			FirstName: "User",
			DOB:       time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			Sex:       "M",
			Race:      "W",
		},
		ControlNumber: "ATFTEST001",
		Date:          fixedDate,
		Compressor:    &NoneCompressor{},
	})
	if err != nil {
		t.Fatal(err)
	}

	s := string(data)

	// ATF-specific values must appear.
	checks := map[string]string{
		"TOT":     "1.004:FAUF",
		"VER":     "1.002:0200",
		"DAI":     ATFDestinationAgency,
		"ORI":     ATFOriginatingAgency,
		"NSR":     "1.011:19.69",
		"NTR":     "1.012:19.69",
		"RFP":     "Firearms",
		"Name":    "Test,User",
		"DOB":     "20000101",
		"Sex":     "2.024:M",
		"Race":    "2.025:W",
		"CRI/ORI": "2.073:" + ATFOriginatingAgency,
	}

	for label, substr := range checks {
		if !strings.Contains(s, substr) {
			t.Errorf("ATF output missing %s (%q)", label, substr)
		}
	}
}

func TestStructure_TransactionRecordOrder(t *testing.T) {
	// In a multi-record transaction, verify records appear in order:
	// Type-1 first, then Type-2, then Type-4/14.
	type1, _ := NewType1Record(Type1Options{
		TransactionType:   "FAUF",
		DestinationAgency: ATFDestinationAgency,
		OriginatingAgency: ATFOriginatingAgency,
		ControlNumber:     "TCN20240615ORD",
		Date:              fixedDate,
	})
	type2 := NewType2Record(Type2Options{IDC: 0})

	imgData := make([]byte, 20*20)
	type14, _ := NewType14Record(Type14Options{
		IDC:                  1,
		ImpressionType:       ImpressionLiveScanPlain,
		SourceAgency:         ATFOriginatingAgency,
		CaptureDate:          "20240615",
		HorizontalLineLength: 20,
		VerticalLineLength:   20,
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

	s := string(data)

	// Type-1 should come first.
	idx1 := strings.Index(s, "1.001:")
	idx2 := strings.Index(s, "2.001:")
	idx14 := strings.Index(s, "14.001:")

	if idx1 == -1 || idx2 == -1 || idx14 == -1 {
		t.Fatal("missing record header in output")
	}

	if idx1 >= idx2 {
		t.Errorf("Type-1 record (%d) should appear before Type-2 (%d)", idx1, idx2)
	}
	if idx2 >= idx14 {
		t.Errorf("Type-2 record (%d) should appear before Type-14 (%d)", idx2, idx14)
	}
}

// ---- Edge case and error tests ----

func TestEdge_EmptyTransaction(t *testing.T) {
	txn := &Transaction{}
	_, err := txn.Encode()
	if err == nil {
		t.Error("expected error for empty transaction")
	}
}

func TestEdge_TransactionMissingType1(t *testing.T) {
	type2 := NewType2Record(Type2Options{IDC: 0})
	txn := &Transaction{}
	txn.AddRecord(type2)
	_, err := txn.Encode()
	if err == nil {
		t.Error("expected error for transaction without Type-1")
	}
}

func TestEdge_Type14MissingImageData(t *testing.T) {
	_, err := NewType14Record(Type14Options{
		IDC:                  1,
		ImpressionType:       ImpressionLiveScanPlain,
		SourceAgency:         "WV1234567",
		CaptureDate:          "20240615",
		HorizontalLineLength: 100,
		VerticalLineLength:   100,
	})
	if err == nil {
		t.Error("expected error for nil image data")
	}
}

func TestEdge_Type14MissingDimensions(t *testing.T) {
	_, err := NewType14Record(Type14Options{
		IDC:          1,
		SourceAgency: "WV1234567",
		CaptureDate:  "20240615",
		ImageData:    make([]byte, 100),
	})
	if err == nil {
		t.Error("expected error for zero dimensions")
	}
}

func TestEdge_Type14MissingSourceAgency(t *testing.T) {
	_, err := NewType14Record(Type14Options{
		IDC:                  1,
		CaptureDate:          "20240615",
		HorizontalLineLength: 10,
		VerticalLineLength:   10,
		ImageData:            make([]byte, 100),
	})
	if err == nil {
		t.Error("expected error for empty source agency")
	}
}

func TestEdge_Type14MissingCaptureDate(t *testing.T) {
	_, err := NewType14Record(Type14Options{
		IDC:                  1,
		SourceAgency:         "WV1234567",
		HorizontalLineLength: 10,
		VerticalLineLength:   10,
		ImageData:            make([]byte, 100),
	})
	if err == nil {
		t.Error("expected error for empty capture date")
	}
}

func TestEdge_Type4NilImage(t *testing.T) {
	_, _, err := NewType4Record(Type4Options{
		IDC:            1,
		FingerPosition: FingerRightThumb,
		Image:          nil,
	})
	if err == nil {
		t.Error("expected error for nil image")
	}
}

func TestEdge_Type1MissingFields(t *testing.T) {
	tests := []struct {
		name string
		opts Type1Options
	}{
		{"missing_tot", Type1Options{DestinationAgency: "A", OriginatingAgency: "B", ControlNumber: "C"}},
		{"missing_dai", Type1Options{TransactionType: "A", OriginatingAgency: "B", ControlNumber: "C"}},
		{"missing_ori", Type1Options{TransactionType: "A", DestinationAgency: "B", ControlNumber: "C"}},
		{"missing_tcn", Type1Options{TransactionType: "A", DestinationAgency: "B", OriginatingAgency: "C"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewType1Record(tc.opts)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestEdge_ATFMissingPersonInfo(t *testing.T) {
	images := &FD258Images{}
	for i := 0; i < 10; i++ {
		images.Rolled[i] = deterministicGray(50, 50, int64(1000+i))
	}
	images.FlatRight = deterministicGray(100, 50, 1010)
	images.FlatLeft = deterministicGray(100, 50, 1011)
	images.FlatThumbs = deterministicGray(70, 50, 1012)

	tests := []struct {
		name   string
		person ATFPersonInfo
	}{
		{"missing_lastname", ATFPersonInfo{FirstName: "John", DOB: fixedDate}},
		{"missing_firstname", ATFPersonInfo{LastName: "Doe", DOB: fixedDate}},
		{"missing_dob", ATFPersonInfo{LastName: "Doe", FirstName: "John"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CreateATFTransactionFromImages(images, ATFSubmissionOptions{
				Person:     tc.person,
				Compressor: &NoneCompressor{},
				Date:       fixedDate,
			})
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

func TestEdge_CreateTransactionNilReader(t *testing.T) {
	_, err := CreateTransaction(
		TransactionOptions{
			TransactionType:   "CAR",
			DestinationAgency: "AAAAAAAAA",
			OriginatingAgency: "BBBBBBBBB",
			ControlNumber:     "TCN001",
			Date:              fixedDate,
		},
		[]FingerprintImage{
			{
				FingerPosition: FingerRightThumb,
				ImpressionType: ImpressionLiveScanPlain,
				Reader:         nil,
			},
		},
	)
	if err == nil {
		t.Error("expected error for nil reader")
	}
}

func TestEdge_CropFD258MinimumSize(t *testing.T) {
	// Just above the minimum size threshold.
	img := deterministicGray(101, 101, 1100)
	layout := DefaultFD258Layout()
	images, err := CropFD258(img, layout)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all crops are non-nil.
	for i := 0; i < 10; i++ {
		if images.Rolled[i] == nil {
			t.Errorf("rolled[%d] is nil for 101x101 image", i)
		}
	}
}

// ---- FD-258 crop dimension tests ----

func TestFD258_CropDimensions(t *testing.T) {
	// Verify crop regions produce images with expected relative dimensions.
	img := deterministicGray(4000, 4000, 1200)
	layout := DefaultFD258Layout()

	images, err := CropFD258(img, layout)
	if err != nil {
		t.Fatal(err)
	}

	// All rolled prints should have similar dimensions (~20% of card width, ~23% of card height).
	for i := 0; i < 10; i++ {
		bounds := images.Rolled[i].Bounds()
		w := bounds.Dx()
		h := bounds.Dy()

		// Width should be ~18% of 4000 = ~720 pixels.
		if w < 600 || w > 900 {
			t.Errorf("rolled[%d] width %d outside expected range [600, 900]", i, w)
		}
		// Height should be ~23% of 4000 = ~920 pixels.
		if h < 750 || h > 1100 {
			t.Errorf("rolled[%d] height %d outside expected range [750, 1100]", i, h)
		}
	}

	// Flat prints should be wider than rolled prints.
	flatRightW := images.FlatRight.Bounds().Dx()
	flatLeftW := images.FlatLeft.Bounds().Dx()

	if flatRightW < images.Rolled[0].Bounds().Dx() {
		t.Error("FlatRight should be wider than a rolled print")
	}
	if flatLeftW < images.Rolled[0].Bounds().Dx() {
		t.Error("FlatLeft should be wider than a rolled print")
	}
}

func TestFD258_LayoutFractions(t *testing.T) {
	layout := DefaultFD258Layout()

	// Verify all fractions are in [0, 1].
	rects := make([]FractionalRect, 0, 13)
	for i := 0; i < 10; i++ {
		rects = append(rects, layout.RolledPrints[i])
	}
	rects = append(rects, layout.FlatRight, layout.FlatLeft, layout.FlatThumbs)

	for i, r := range rects {
		if r.X1 < 0 || r.X1 > 1 || r.Y1 < 0 || r.Y1 > 1 ||
			r.X2 < 0 || r.X2 > 1 || r.Y2 < 0 || r.Y2 > 1 {
			t.Errorf("rect %d has coordinates outside [0,1]: %+v", i, r)
		}
		if r.X2 <= r.X1 {
			t.Errorf("rect %d: X2 (%f) <= X1 (%f)", i, r.X2, r.X1)
		}
		if r.Y2 <= r.Y1 {
			t.Errorf("rect %d: Y2 (%f) <= Y1 (%f)", i, r.Y2, r.Y1)
		}
	}
}

func TestFD258_FractionalRectToRect(t *testing.T) {
	fr := FractionalRect{0.1, 0.2, 0.5, 0.8}
	r := fr.toRect(1000, 500)

	if r.Min.X != 100 {
		t.Errorf("Min.X: got %d, want 100", r.Min.X)
	}
	if r.Min.Y != 100 {
		t.Errorf("Min.Y: got %d, want 100", r.Min.Y)
	}
	if r.Max.X != 500 {
		t.Errorf("Max.X: got %d, want 500", r.Max.X)
	}
	if r.Max.Y != 400 {
		t.Errorf("Max.Y: got %d, want 400", r.Max.Y)
	}
}

// ---- Compressor tests ----

func TestCompressor_NonePreservesPixels(t *testing.T) {
	// Verify NoneCompressor returns exact pixel data.
	img := deterministicGray(30, 20, 1300)
	comp := &NoneCompressor{}

	data, err := comp.Compress(img)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != 30*20 {
		t.Fatalf("compressed size: got %d, want %d", len(data), 30*20)
	}

	// Compare pixel by pixel.
	for y := 0; y < 20; y++ {
		for x := 0; x < 30; x++ {
			expected := img.GrayAt(x, y).Y
			got := data[y*30+x]
			if got != expected {
				t.Errorf("pixel (%d,%d): got %d, want %d", x, y, got, expected)
			}
		}
	}
}

func TestCompressor_WSQRoundTrip(t *testing.T) {
	// Verify WSQ produces non-empty output that's smaller than raw.
	img := deterministicGray(800, 750, 1400)
	comp := &WSQCompressor{Bitrate: 0.75}

	data, err := comp.Compress(img)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("WSQ output is empty")
	}

	rawSize := 800 * 750
	ratio := float64(len(data)) / float64(rawSize)
	if ratio >= 1.0 {
		t.Errorf("WSQ not compressing: ratio %.2f", ratio)
	}
	// At 0.75 bitrate, expect roughly 15:1 compression.
	if ratio > 0.2 {
		t.Errorf("WSQ compression ratio %.2f worse than expected (~0.07)", ratio)
	}
}

func TestCompressor_WSQDefaultBitrate(t *testing.T) {
	// Verify zero bitrate defaults to 0.75.
	comp := &WSQCompressor{Bitrate: 0}
	img := deterministicGray(800, 750, 1401)

	data, err := comp.Compress(img)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) == 0 {
		t.Fatal("WSQ with default bitrate produced empty output")
	}
}

func TestCompressor_DefaultCompressorIsWSQ(t *testing.T) {
	comp := DefaultCompressor()
	if comp.Algorithm() != CompressionWSQ {
		t.Errorf("DefaultCompressor().Algorithm() = %q, want %q", comp.Algorithm(), CompressionWSQ)
	}
}

// ---- Type-2 field encoding tests ----

func TestType2_IDCFormatting(t *testing.T) {
	tests := []struct {
		idc  int
		want string
	}{
		{0, "00"},
		{1, "01"},
		{9, "09"},
		{10, "10"},
		{42, "42"},
		{99, "99"},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("idc_%d", tc.idc), func(t *testing.T) {
			r := NewType2Record(Type2Options{IDC: tc.idc})
			got := string(r.GetField(2))
			if got != tc.want {
				t.Errorf("IDC %d formatted as %q, want %q", tc.idc, got, tc.want)
			}
		})
	}
}

func TestType2_FieldsDoNotOverrideLENOrIDC(t *testing.T) {
	r := NewType2Record(Type2Options{
		IDC: 5,
		Fields: map[int][]byte{
			1:  []byte("should_be_ignored"),
			2:  []byte("should_be_ignored"),
			18: []byte("Doe,John"),
		},
	})

	// IDC should still be "05", not overridden.
	if string(r.GetField(2)) != "05" {
		t.Errorf("IDC was overridden: got %q", r.GetField(2))
	}

	// NAM should be set.
	if string(r.GetField(18)) != "Doe,John" {
		t.Errorf("NAM not set: got %q", r.GetField(18))
	}
}

// ---- ATF Type-2 field builder tests ----

func TestATFType2Fields_AllFields(t *testing.T) {
	fields := buildATFType2Fields(ATFPersonInfo{
		LastName:     "O'Brien",
		FirstName:    "James",
		MiddleName:   "Patrick",
		DOB:          time.Date(1970, 12, 25, 0, 0, 0, 0, time.UTC),
		Sex:          "M",
		Race:         "W",
		PlaceOfBirth: "TX",
		Citizenship:  "US",
		Height:       "511",
		Weight:       "195",
		EyeColor:     "GRN",
		HairColor:    "RED",
		SSN:          "555667788",
		Address:      "456 Oak Ave, Dallas, TX 75201",
	}, "20240615")

	checks := map[int]string{
		5:  "N",
		16: "555667788",
		18: "O'Brien,James Patrick",
		20: "TX",
		21: "US",
		22: "19701225",
		24: "M",
		25: "W",
		27: "511",
		29: "195",
		31: "GRN",
		32: "RED",
		37: "Firearms",
		38: "20240615",
		41: "456 Oak Ave, Dallas, TX 75201",
		73: ATFOriginatingAgency,
	}

	for fieldNum, expected := range checks {
		got := string(fields[fieldNum])
		if got != expected {
			t.Errorf("field 2.%03d: got %q, want %q", fieldNum, got, expected)
		}
	}
}

func TestATFType2Fields_MinimalFields(t *testing.T) {
	// Only required fields, no optional ones.
	fields := buildATFType2Fields(ATFPersonInfo{
		LastName:  "Doe",
		FirstName: "Jane",
		DOB:       time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
	}, "20240615")

	// Required fields must be present.
	if string(fields[18]) != "Doe,Jane" {
		t.Errorf("NAM: got %q", fields[18])
	}
	if string(fields[22]) != "19900101" {
		t.Errorf("DOB: got %q", fields[22])
	}
	if string(fields[37]) != "Firearms" {
		t.Errorf("RFP: got %q", fields[37])
	}

	// Optional fields should be absent.
	optionalFields := []int{16, 20, 21, 24, 25, 27, 29, 31, 32, 41}
	for _, fn := range optionalFields {
		if _, ok := fields[fn]; ok {
			t.Errorf("optional field 2.%03d should not be set for minimal person", fn)
		}
	}
}

func TestATFType2Fields_NameFormat(t *testing.T) {
	tests := []struct {
		name       string
		person     ATFPersonInfo
		wantNAM    string
	}{
		{
			"first_last_only",
			ATFPersonInfo{LastName: "Smith", FirstName: "John", DOB: fixedDate},
			"Smith,John",
		},
		{
			"with_middle",
			ATFPersonInfo{LastName: "Smith", FirstName: "John", MiddleName: "Q", DOB: fixedDate},
			"Smith,John Q",
		},
		{
			"hyphenated_last",
			ATFPersonInfo{LastName: "Garcia-Lopez", FirstName: "Maria", DOB: fixedDate},
			"Garcia-Lopez,Maria",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := buildATFType2Fields(tc.person, "20240615")
			got := string(fields[18])
			if got != tc.wantNAM {
				t.Errorf("NAM: got %q, want %q", got, tc.wantNAM)
			}
		})
	}
}

// ---- Image conversion tests ----

func TestImageToGray_RGBA(t *testing.T) {
	rgba := image.NewRGBA(image.Rect(0, 0, 3, 3))
	rgba.Set(0, 0, color.White)
	rgba.Set(1, 0, color.Black)
	rgba.Set(2, 0, color.RGBA{128, 128, 128, 255})

	gray := imageToGray(rgba)

	if gray.GrayAt(0, 0).Y != 255 {
		t.Errorf("white: got %d", gray.GrayAt(0, 0).Y)
	}
	if gray.GrayAt(1, 0).Y != 0 {
		t.Errorf("black: got %d", gray.GrayAt(1, 0).Y)
	}
	// Mid-gray should be close to 128.
	midGray := gray.GrayAt(2, 0).Y
	if midGray < 120 || midGray > 136 {
		t.Errorf("mid-gray: got %d, expected ~128", midGray)
	}
}

func TestImageToGray_AlreadyGray(t *testing.T) {
	orig := image.NewGray(image.Rect(0, 0, 10, 10))
	result := imageToGray(orig)
	if result != orig {
		t.Error("imageToGray should return same pointer for Gray input")
	}
}

func TestImageToGray_NRGBA(t *testing.T) {
	nrgba := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	nrgba.Set(0, 0, color.White)
	nrgba.Set(1, 1, color.Black)

	gray := imageToGray(nrgba)
	if gray.GrayAt(0, 0).Y != 255 {
		t.Errorf("white: got %d", gray.GrayAt(0, 0).Y)
	}
	if gray.GrayAt(1, 1).Y != 0 {
		t.Errorf("black: got %d", gray.GrayAt(1, 1).Y)
	}
}

// ---- Type-1 field tests ----

func TestType1_DefaultValues(t *testing.T) {
	date := time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC)
	r, err := NewType1Record(Type1Options{
		TransactionType:   "CAR",
		DestinationAgency: "WVFBI0000",
		OriginatingAgency: "WV1234567",
		ControlNumber:     "TCN001",
		Date:              date,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Default version.
	if string(r.GetField(2)) != "0502" {
		t.Errorf("VER: got %q, want 0502", r.GetField(2))
	}

	// Default priority.
	if string(r.GetField(6)) != "4" {
		t.Errorf("PRY: got %q, want 4", r.GetField(6))
	}

	// Default NSR/NTR.
	if string(r.GetField(11)) != "00.00" {
		t.Errorf("NSR: got %q, want 00.00", r.GetField(11))
	}
	if string(r.GetField(12)) != "00.00" {
		t.Errorf("NTR: got %q, want 00.00", r.GetField(12))
	}

	// Date formatting.
	if string(r.GetField(5)) != "20240615" {
		t.Errorf("DAT: got %q, want 20240615", r.GetField(5))
	}
}

func TestType1_ControlReference(t *testing.T) {
	r, err := NewType1Record(Type1Options{
		TransactionType:   "CAR",
		DestinationAgency: "WVFBI0000",
		OriginatingAgency: "WV1234567",
		ControlNumber:     "TCN001",
		ControlReference:  "REF001",
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(r.GetField(10)) != "REF001" {
		t.Errorf("TCR: got %q, want REF001", r.GetField(10))
	}
}

func TestType1_DomainField(t *testing.T) {
	r, err := NewType1Record(Type1Options{
		TransactionType:   "CAR",
		DestinationAgency: "WVFBI0000",
		OriginatingAgency: "WV1234567",
		ControlNumber:     "TCN001",
		DomainName:        "NORAM",
		DomainVersion:     "11.1",
	})
	if err != nil {
		t.Fatal(err)
	}

	dom := r.GetField(13)
	expected := "NORAM" + string(US) + "11.1"
	if string(dom) != expected {
		t.Errorf("DOM: got %q, want %q", dom, expected)
	}
}

func TestType1_CustomResolution(t *testing.T) {
	r, err := NewType1Record(Type1Options{
		TransactionType:               "CAR",
		DestinationAgency:             "WVFBI0000",
		OriginatingAgency:             "WV1234567",
		ControlNumber:                 "TCN001",
		NativeScanningResolution:      "19.69",
		NominalTransmittingResolution: "19.69",
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(r.GetField(11)) != "19.69" {
		t.Errorf("NSR: got %q, want 19.69", r.GetField(11))
	}
	if string(r.GetField(12)) != "19.69" {
		t.Errorf("NTR: got %q, want 19.69", r.GetField(12))
	}
}

// ---- Helper ----

// pngFromGrayPixels creates a PNG from raw grayscale pixel data.
func pngFromGrayPixels(w, h int, pixels []byte) *bytes.Buffer {
	img := image.NewGray(image.Rect(0, 0, w, h))
	copy(img.Pix, pixels)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return &buf
}
