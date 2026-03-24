package eft

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"time"
)

// ATF-specific constants per eForms requirements.
const (
	ATFTransactionType   = "FAUF" // Federal Applicant User Fee
	ATFDestinationAgency = "WVIAFIS0Z"
	ATFOriginatingAgency = "WVATF0800"
	ATFReasonPrinted     = "Firearms"
	ATFVersion       = "0200" // ANSI/NIST-ITL 1-2000 (FBI EFTS 7.1)
	ATFDomainVersion = "8.1"

	// ATFMaxFileSize is the maximum EFT file size accepted by ATF eForms.
	ATFMaxFileSize = 12 * 1024 * 1024 // 12 MB
)

// ATFPersonInfo holds the demographic data for an ATF eForms submission.
// These fields populate the Type-2 record. Name and DOB are validated
// by ATF eForms upon upload.
type ATFPersonInfo struct {
	// LastName is required.
	LastName string
	// FirstName is required.
	FirstName string
	// MiddleName is optional.
	MiddleName string
	// DOB is date of birth (required, displayed by ATF for verification).
	DOB time.Time
	// Sex: "M", "F", or "X".
	Sex string
	// Race: "A" (Asian), "B" (Black), "W" (White), "I" (American Indian), "U" (Unknown).
	Race string
	// PlaceOfBirth is a 2-letter state code or country code.
	PlaceOfBirth string
	// Citizenship, e.g., "US".
	Citizenship string
	// Height in format "FII" (e.g., "510" for 5'10"). "000" if unknown.
	Height string
	// Weight in pounds as string (e.g., "180"). "000" if unknown.
	Weight string
	// EyeColor: 3-letter code (e.g., "BRO", "BLU", "GRN", "HAZ").
	EyeColor string
	// HairColor: 3-letter code (e.g., "BLK", "BRO", "BLN", "RED").
	HairColor string
	// SSN is social security number (9 digits, no dashes). Optional.
	SSN string
	// Address is optional. Note: ATF validator rejects spaces in field 2.041,
	// so this field is not included in the EFT output.
	Address string
}

// ATFSubmissionOptions configures an ATF eForms EFT submission.
type ATFSubmissionOptions struct {
	// Person holds the required demographic data.
	Person ATFPersonInfo
	// ControlNumber is a unique transaction identifier (10-40 bytes).
	// If empty, one will be generated from the current timestamp.
	ControlNumber string
	// Date overrides the transaction/print date. Defaults to now.
	Date time.Time
	// Compressor overrides the image compression. Default: WSQ at 0.75 bitrate.
	Compressor Compressor
}

// CreateATFTransaction builds an EFT file suitable for ATF eForms submission
// (Form 1, Form 4, etc.) from a scanned FD-258 fingerprint card image.
//
// The card image should be a scan of the FD-258 fingerprint area at 500+ DPI.
// The function crops the card into individual prints, compresses them with WSQ,
// and builds the complete ANSI/NIST-ITL transaction.
//
// Returns the encoded EFT bytes ready to write to a .eft file.
func CreateATFTransaction(cardReader io.Reader, opts ATFSubmissionOptions) ([]byte, error) {
	if opts.Person.LastName == "" || opts.Person.FirstName == "" {
		return nil, fmt.Errorf("eft: LastName and FirstName are required")
	}
	if opts.Person.DOB.IsZero() {
		return nil, fmt.Errorf("eft: DOB is required")
	}

	// Decode the card image.
	cardImg, _, err := image.Decode(cardReader)
	if err != nil {
		return nil, fmt.Errorf("eft: decoding card image: %w", err)
	}

	// Crop the FD-258 card into individual prints.
	layout := DefaultFD258Layout()
	images, err := CropFD258(cardImg, layout)
	if err != nil {
		return nil, err
	}

	return createATFFromImages(images, opts)
}

// CreateATFTransactionFromImages builds an EFT file from pre-cropped FD-258 images.
// Use this when you have already extracted individual prints from the card.
func CreateATFTransactionFromImages(images *FD258Images, opts ATFSubmissionOptions) ([]byte, error) {
	if opts.Person.LastName == "" || opts.Person.FirstName == "" {
		return nil, fmt.Errorf("eft: LastName and FirstName are required")
	}
	if opts.Person.DOB.IsZero() {
		return nil, fmt.Errorf("eft: DOB is required")
	}
	return createATFFromImages(images, opts)
}

func createATFFromImages(images *FD258Images, opts ATFSubmissionOptions) ([]byte, error) {
	comp := opts.Compressor
	if comp == nil {
		comp = DefaultCompressor()
	}

	date := opts.Date
	if date.IsZero() {
		date = time.Now()
	}

	tcn := opts.ControlNumber
	if tcn == "" {
		tcn = fmt.Sprintf("ATFEFT%s", date.Format("20060102150405"))
	}

	dateStr := date.Format("20060102")

	// Build Type-1 header.
	// NSR/NTR = "19.69" (500 ppi) since we have Type-4 binary records.
	type1, err := NewType1Record(Type1Options{
		TransactionType:               ATFTransactionType,
		DestinationAgency:             ATFDestinationAgency,
		OriginatingAgency:             ATFOriginatingAgency,
		ControlNumber:                 tcn,
		Version:                       ATFVersion,
		NativeScanningResolution:      "19.69",
		NominalTransmittingResolution: "19.69",
		Date:                          date,
		DomainName:                    "NORAM",
		DomainVersion:                 ATFDomainVersion,
	})
	if err != nil {
		return nil, err
	}

	// Build Type-2 demographic record.
	type2Fields := buildATFType2Fields(opts.Person, dateStr)
	type2 := NewType2Record(Type2Options{
		IDC:    0,
		Fields: type2Fields,
	})

	txn := &Transaction{}
	txn.AddRecord(type1)
	txn.AddRecord(type2)

	// Add Type-4 records for 10 rolled prints.
	// ATF eForms expects rolled prints as Type-4 binary records (max 3 Type-14 allowed).
	for i := 0; i < 10; i++ {
		if images.Rolled[i] == nil {
			continue
		}

		fp := FingerPosition(i + 1)
		rec, _, err := NewType4Record(Type4Options{
			IDC:            i + 1,
			ImpressionType: ImpressionNonLiveRolled,
			FingerPosition: fp,
			Image:          images.Rolled[i],
			Compressor:     comp,
		})
		if err != nil {
			return nil, fmt.Errorf("eft: rolled finger %d: %w", i+1, err)
		}

		txn.AddRecord(rec)
	}

	// Note: ATF eForms FAUF transactions accept only Type-4 rolled prints.
	// Slap/flat prints (Type-14) are not included — mixing Type-4 and Type-14
	// causes "mutually exclusive records" validation errors.

	data, err := txn.Encode()
	if err != nil {
		return nil, err
	}

	if len(data) > ATFMaxFileSize {
		return nil, fmt.Errorf("eft: encoded transaction is %d bytes, exceeds ATF limit of %d bytes (12 MB)", len(data), ATFMaxFileSize)
	}

	return data, nil
}

func buildATFType2Fields(p ATFPersonInfo, dateStr string) map[int][]byte {
	fields := make(map[int][]byte)

	// 2.005 RET — return code
	fields[5] = []byte("N")

	// 2.016 SOC — SSN
	if p.SSN != "" {
		fields[16] = []byte(p.SSN)
	}

	// 2.018 NAM — name: "Last,First Middle" (comma-separated per EFTS)
	name := p.LastName + "," + p.FirstName
	if p.MiddleName != "" {
		name += " " + p.MiddleName
	}
	fields[18] = []byte(name)

	// 2.020 POB — place of birth (mandatory for FAUF; "XX" = unknown)
	pob := p.PlaceOfBirth
	if pob == "" {
		pob = "XX"
	}
	fields[20] = []byte(pob)

	// 2.021 CTZ — citizenship
	if p.Citizenship != "" {
		fields[21] = []byte(p.Citizenship)
	}

	// 2.022 DOB
	fields[22] = []byte(p.DOB.Format("20060102"))

	// 2.024 SEX (mandatory for FAUF; "U" = unknown)
	sex := p.Sex
	if sex == "" {
		sex = "U"
	}
	fields[24] = []byte(sex)

	// 2.025 RAC — race (mandatory for FAUF; "U" = unknown)
	race := p.Race
	if race == "" {
		race = "U"
	}
	fields[25] = []byte(race)

	// 2.027 HGT — height (mandatory for FAUF; "000" = unknown)
	hgt := p.Height
	if hgt == "" {
		hgt = "000"
	}
	fields[27] = []byte(hgt)

	// 2.029 WGT — weight (mandatory for FAUF; "000" = unknown)
	wgt := p.Weight
	if wgt == "" {
		wgt = "000"
	}
	fields[29] = []byte(wgt)

	// 2.031 EYE — eye color (mandatory for FAUF; "XXX" = unknown)
	eye := p.EyeColor
	if eye == "" {
		eye = "XXX"
	}
	fields[31] = []byte(eye)

	// 2.032 HAI — hair color (mandatory for FAUF; "XXX" = unknown)
	hai := p.HairColor
	if hai == "" {
		hai = "XXX"
	}
	fields[32] = []byte(hai)

	// 2.037 RFP — reason fingerprinted
	fields[37] = []byte(ATFReasonPrinted)

	// 2.038 DPR — date printed
	fields[38] = []byte(dateStr)

	// Note: 2.041 RES (address) is omitted — ATF validator rejects spaces
	// in this field. Address is optional for FAUF.

	// 2.073 CRI — controlling agency
	fields[73] = []byte(ATFOriginatingAgency)

	return fields
}
