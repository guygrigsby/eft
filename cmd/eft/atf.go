package main

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"time"

	"github.com/guygrigsby/eft/pkg/eft"
	"github.com/spf13/cobra"
)

var atfFlags struct {
	lastName    string
	firstName   string
	middleName  string
	dob         string
	sex         string
	race        string
	pob         string
	citizenship string
	height      string
	weight      string
	eyeColor    string
	hairColor   string
	ssn         string
	address     string
	tcn         string
	date        string
	output      string
	compression string
	ocr         bool
}

var atfCmd = &cobra.Command{
	Use:   "atf <card-image>",
	Short: "Create an ATF eForms EFT from a scanned FD-258 card",
	Long: `Create an EFT file suitable for ATF eForms submission (Form 1, Form 4)
from a scanned FD-258 fingerprint card image (PNG or JPEG).

The card image should be a scan of the full FD-258 card at 500+ DPI.
Individual fingerprints are automatically cropped from the card.

ATF constants (TOT=FAUF, DAI=WVIAFIS0Z, ORI=WVATF0800, VER=0200) are
set automatically.

Use --ocr to automatically read demographic data (name, DOB, sex, race,
height, weight, etc.) from the card header using tesseract OCR. Any
flags you provide will override the OCR results. With --ocr, the
--last-name, --first-name, and --dob flags become optional.

Examples:
  # Manual entry (all demographics via flags):
  eft atf --last-name Doe --first-name John --dob 1990-01-01 \
    --sex M --race W -o submission.eft card_scan.png

  # OCR mode (demographics read from card, no flags needed):
  eft atf --ocr -o submission.eft card_scan.png

  # OCR with corrections (OCR reads card, flags override mistakes):
  eft atf --ocr --first-name Jon -o submission.eft card_scan.png`,
	Args: cobra.ExactArgs(1),
	RunE: runATF,
}

func init() {
	f := atfCmd.Flags()
	f.StringVar(&atfFlags.lastName, "last-name", "", "last name (required without --ocr)")
	f.StringVar(&atfFlags.firstName, "first-name", "", "first name (required without --ocr)")
	f.StringVar(&atfFlags.middleName, "middle-name", "", "middle name")
	f.StringVar(&atfFlags.dob, "dob", "", "date of birth YYYY-MM-DD (required without --ocr)")
	f.StringVar(&atfFlags.sex, "sex", "", "sex: M, F, or X")
	f.StringVar(&atfFlags.race, "race", "", "race: A, B, W, I, or U")
	f.StringVar(&atfFlags.pob, "pob", "", "place of birth (2-letter state/country code)")
	f.StringVar(&atfFlags.citizenship, "citizenship", "", "citizenship (e.g. US)")
	f.StringVar(&atfFlags.height, "height", "", "height as FII (e.g. 510 for 5'10\")")
	f.StringVar(&atfFlags.weight, "weight", "", "weight in pounds (e.g. 180)")
	f.StringVar(&atfFlags.eyeColor, "eye-color", "", "eye color: BRO, BLU, GRN, HAZ, etc.")
	f.StringVar(&atfFlags.hairColor, "hair-color", "", "hair color: BLK, BRO, BLN, RED, etc.")
	f.StringVar(&atfFlags.ssn, "ssn", "", "social security number (9 digits, no dashes)")
	f.StringVar(&atfFlags.address, "address", "", "address")
	f.StringVar(&atfFlags.tcn, "tcn", "", "transaction control number (auto-generated if empty)")
	f.StringVar(&atfFlags.date, "date", "", "transaction date YYYY-MM-DD (default today)")
	f.StringVarP(&atfFlags.output, "output", "o", "", "output EFT file path (required)")
	f.StringVarP(&atfFlags.compression, "compression", "c", "wsq", "compression: wsq or none")
	f.BoolVar(&atfFlags.ocr, "ocr", false, "extract demographics from card header using tesseract OCR")

	atfCmd.MarkFlagRequired("output")
}

func runATF(cmd *cobra.Command, args []string) error {
	comp, err := parseCompressor(atfFlags.compression)
	if err != nil {
		return err
	}

	date, err := parseDate(atfFlags.date)
	if err != nil {
		return err
	}

	cardFile, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("opening card image: %w", err)
	}
	defer cardFile.Close()

	var person eft.ATFPersonInfo

	if atfFlags.ocr {
		// OCR mode: extract demographics from card header.
		if err := checkTesseract(); err != nil {
			return err
		}

		// Decode the image for OCR (we'll need to re-open for transaction creation).
		cardImg, _, err := image.Decode(cardFile)
		if err != nil {
			return fmt.Errorf("decoding card image: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "extracting demographics from card header...")

		ocr := &TesseractOCR{}
		result, err := eft.ExtractDemographics(
			context.Background(), cardImg, ocr, eft.DefaultFD258HeaderFields(),
		)
		if err != nil {
			return fmt.Errorf("OCR extraction: %w", err)
		}

		person = result.Person

		// Print what OCR found.
		fmt.Fprintln(cmd.OutOrStdout(), "OCR results:")
		printOCRField(cmd, "  name", person.LastName+", "+person.FirstName+" "+person.MiddleName)
		if !person.DOB.IsZero() {
			printOCRField(cmd, "  dob", person.DOB.Format("2006-01-02"))
		} else {
			printOCRField(cmd, "  dob", "(not detected)")
		}
		printOCRField(cmd, "  sex", person.Sex)
		printOCRField(cmd, "  race", person.Race)
		printOCRField(cmd, "  height", person.Height)
		printOCRField(cmd, "  weight", person.Weight)
		printOCRField(cmd, "  eye color", person.EyeColor)
		printOCRField(cmd, "  hair color", person.HairColor)
		printOCRField(cmd, "  pob", person.PlaceOfBirth)
		printOCRField(cmd, "  citizenship", person.Citizenship)
		if person.SSN != "" {
			printOCRField(cmd, "  ssn", "***-**-"+person.SSN[5:])
		}

		if len(result.Warnings) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "warnings:")
			for _, w := range result.Warnings {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", w)
			}
		}

		// Merge CLI flag overrides on top of OCR results.
		cliPerson := buildPersonFromFlags()
		person = eft.MergeDemographics(person, cliPerson)

		// Re-open the card file for transaction creation.
		cardFile.Close()
		cardFile, err = os.Open(args[0])
		if err != nil {
			return fmt.Errorf("re-opening card image: %w", err)
		}
		defer cardFile.Close()
	} else {
		// Manual mode: require name and DOB flags.
		if atfFlags.lastName == "" || atfFlags.firstName == "" || atfFlags.dob == "" {
			return fmt.Errorf("--last-name, --first-name, and --dob are required (use --ocr to read from card)")
		}
		person = buildPersonFromFlags()
	}

	// Parse DOB from flag if provided (overrides OCR).
	if atfFlags.dob != "" {
		dob, err := time.Parse("2006-01-02", atfFlags.dob)
		if err != nil {
			return fmt.Errorf("invalid --dob %q, expected YYYY-MM-DD: %w", atfFlags.dob, err)
		}
		person.DOB = dob
	}

	// Final validation.
	if person.LastName == "" || person.FirstName == "" {
		return fmt.Errorf("could not determine name (OCR may have failed — provide --last-name and --first-name)")
	}
	if person.DOB.IsZero() {
		return fmt.Errorf("could not determine DOB (OCR may have failed — provide --dob)")
	}

	data, err := eft.CreateATFTransaction(cardFile, eft.ATFSubmissionOptions{
		Person:        person,
		ControlNumber: atfFlags.tcn,
		Date:          date,
		Compressor:    comp,
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(atfFlags.output, data, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes, %.1f MB)\n",
		atfFlags.output, len(data), float64(len(data))/(1024*1024))
	if len(data) > eft.ATFMaxFileSize {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: file exceeds ATF 12 MB limit\n")
	}
	return nil
}

// buildPersonFromFlags constructs an ATFPersonInfo from CLI flags.
// Empty flags produce empty fields (zero values).
func buildPersonFromFlags() eft.ATFPersonInfo {
	return eft.ATFPersonInfo{
		LastName:     atfFlags.lastName,
		FirstName:    atfFlags.firstName,
		MiddleName:   atfFlags.middleName,
		Sex:          atfFlags.sex,
		Race:         atfFlags.race,
		PlaceOfBirth: atfFlags.pob,
		Citizenship:  atfFlags.citizenship,
		Height:       atfFlags.height,
		Weight:       atfFlags.weight,
		EyeColor:     atfFlags.eyeColor,
		HairColor:    atfFlags.hairColor,
		SSN:          atfFlags.ssn,
		Address:      atfFlags.address,
	}
}

func printOCRField(cmd *cobra.Command, label, value string) {
	if value == "" {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", label, value)
}
