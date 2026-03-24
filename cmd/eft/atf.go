package main

import (
	"fmt"
	"os"
	"time"

	"github.com/guygrigsby/eft/pkg/eft"
	"github.com/spf13/cobra"
)

var atfFlags struct {
	lastName     string
	firstName    string
	middleName   string
	dob          string
	sex          string
	race         string
	pob          string
	citizenship  string
	height       string
	weight       string
	eyeColor     string
	hairColor    string
	ssn          string
	address      string
	tcn          string
	date         string
	output       string
	compression  string
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

Examples:
  eft atf --last-name Doe --first-name John --dob 1990-01-01 \
    --sex M --race W -o submission.eft card_scan.png

  eft atf --last-name Smith --first-name Jane --middle-name Q \
    --dob 1985-06-15 --sex F --race W --ssn 123456789 \
    --pob VA --citizenship US --height 507 --weight 130 \
    --eye-color BRO --hair-color BLK --compression none \
    -o submission.eft fd258_scan.png`,
	Args: cobra.ExactArgs(1),
	RunE: runATF,
}

func init() {
	f := atfCmd.Flags()
	f.StringVar(&atfFlags.lastName, "last-name", "", "last name (required)")
	f.StringVar(&atfFlags.firstName, "first-name", "", "first name (required)")
	f.StringVar(&atfFlags.middleName, "middle-name", "", "middle name")
	f.StringVar(&atfFlags.dob, "dob", "", "date of birth YYYY-MM-DD (required)")
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

	atfCmd.MarkFlagRequired("last-name")
	atfCmd.MarkFlagRequired("first-name")
	atfCmd.MarkFlagRequired("dob")
	atfCmd.MarkFlagRequired("output")
}

func runATF(cmd *cobra.Command, args []string) error {
	dob, err := time.Parse("2006-01-02", atfFlags.dob)
	if err != nil {
		return fmt.Errorf("invalid --dob %q, expected YYYY-MM-DD: %w", atfFlags.dob, err)
	}

	date, err := parseDate(atfFlags.date)
	if err != nil {
		return err
	}

	comp, err := parseCompressor(atfFlags.compression)
	if err != nil {
		return err
	}

	cardFile, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("opening card image: %w", err)
	}
	defer cardFile.Close()

	data, err := eft.CreateATFTransaction(cardFile, eft.ATFSubmissionOptions{
		Person: eft.ATFPersonInfo{
			LastName:     atfFlags.lastName,
			FirstName:    atfFlags.firstName,
			MiddleName:   atfFlags.middleName,
			DOB:          dob,
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
		},
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
