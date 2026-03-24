package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"time"

	"github.com/guygrigsby/eft/pkg/eft"
	"github.com/spf13/cobra"
)

var atfImagesFlags struct {
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
	rolled      [10]string
	flatRight   string
	flatLeft    string
	flatThumbs  string
}

var atfImagesCmd = &cobra.Command{
	Use:   "atf-images",
	Short: "Create an ATF eForms EFT from pre-cropped fingerprint images",
	Long: `Create an EFT file suitable for ATF eForms submission from individual
pre-cropped fingerprint images (PNG or JPEG).

Provide rolled prints with --rolled-N (N=1-10) and flat/slap prints with
--flat-right, --flat-left, --flat-thumbs. At least one image is required.

Finger positions for rolled prints:
  1=R.Thumb  2=R.Index  3=R.Middle  4=R.Ring   5=R.Little
  6=L.Thumb  7=L.Index  8=L.Middle  9=L.Ring  10=L.Little

Examples:
  eft atf-images --last-name Doe --first-name John --dob 1990-01-01 \
    --sex M --race W \
    --rolled-1 thumb_r.png --rolled-2 index_r.png \
    --rolled-6 thumb_l.png --rolled-7 index_l.png \
    --flat-right right4.png --flat-left left4.png \
    --flat-thumbs thumbs.png -o submission.eft`,
	RunE: runATFImages,
}

func init() {
	f := atfImagesCmd.Flags()
	f.StringVar(&atfImagesFlags.lastName, "last-name", "", "last name (required)")
	f.StringVar(&atfImagesFlags.firstName, "first-name", "", "first name (required)")
	f.StringVar(&atfImagesFlags.middleName, "middle-name", "", "middle name")
	f.StringVar(&atfImagesFlags.dob, "dob", "", "date of birth YYYY-MM-DD (required)")
	f.StringVar(&atfImagesFlags.sex, "sex", "", "sex: M, F, or X")
	f.StringVar(&atfImagesFlags.race, "race", "", "race: A, B, W, I, or U")
	f.StringVar(&atfImagesFlags.pob, "pob", "", "place of birth (2-letter state/country code)")
	f.StringVar(&atfImagesFlags.citizenship, "citizenship", "", "citizenship (e.g. US)")
	f.StringVar(&atfImagesFlags.height, "height", "", "height as FII (e.g. 510 for 5'10\")")
	f.StringVar(&atfImagesFlags.weight, "weight", "", "weight in pounds (e.g. 180)")
	f.StringVar(&atfImagesFlags.eyeColor, "eye-color", "", "eye color: BRO, BLU, GRN, HAZ, etc.")
	f.StringVar(&atfImagesFlags.hairColor, "hair-color", "", "hair color: BLK, BRO, BLN, RED, etc.")
	f.StringVar(&atfImagesFlags.ssn, "ssn", "", "social security number (9 digits, no dashes)")
	f.StringVar(&atfImagesFlags.address, "address", "", "address")
	f.StringVar(&atfImagesFlags.tcn, "tcn", "", "transaction control number (auto-generated if empty)")
	f.StringVar(&atfImagesFlags.date, "date", "", "transaction date YYYY-MM-DD (default today)")
	f.StringVarP(&atfImagesFlags.output, "output", "o", "", "output EFT file path (required)")
	f.StringVarP(&atfImagesFlags.compression, "compression", "c", "wsq", "compression: wsq or none")

	for i := 1; i <= 10; i++ {
		f.StringVar(&atfImagesFlags.rolled[i-1], fmt.Sprintf("rolled-%d", i), "", fmt.Sprintf("rolled print %d image file", i))
	}
	f.StringVar(&atfImagesFlags.flatRight, "flat-right", "", "right four-finger slap image file")
	f.StringVar(&atfImagesFlags.flatLeft, "flat-left", "", "left four-finger slap image file")
	f.StringVar(&atfImagesFlags.flatThumbs, "flat-thumbs", "", "both-thumbs simultaneous image file")

	atfImagesCmd.MarkFlagRequired("last-name")
	atfImagesCmd.MarkFlagRequired("first-name")
	atfImagesCmd.MarkFlagRequired("dob")
	atfImagesCmd.MarkFlagRequired("output")
}

func runATFImages(cmd *cobra.Command, args []string) error {
	dob, err := time.Parse("2006-01-02", atfImagesFlags.dob)
	if err != nil {
		return fmt.Errorf("invalid --dob %q, expected YYYY-MM-DD: %w", atfImagesFlags.dob, err)
	}

	date, err := parseDate(atfImagesFlags.date)
	if err != nil {
		return err
	}

	comp, err := parseCompressor(atfImagesFlags.compression)
	if err != nil {
		return err
	}

	images := &eft.FD258Images{}
	hasAny := false

	for i := range 10 {
		if atfImagesFlags.rolled[i] == "" {
			continue
		}
		gray, err := loadGrayImage(atfImagesFlags.rolled[i])
		if err != nil {
			return fmt.Errorf("loading --rolled-%d: %w", i+1, err)
		}
		images.Rolled[i] = gray
		hasAny = true
	}

	if atfImagesFlags.flatRight != "" {
		gray, err := loadGrayImage(atfImagesFlags.flatRight)
		if err != nil {
			return fmt.Errorf("loading --flat-right: %w", err)
		}
		images.FlatRight = gray
		hasAny = true
	}

	if atfImagesFlags.flatLeft != "" {
		gray, err := loadGrayImage(atfImagesFlags.flatLeft)
		if err != nil {
			return fmt.Errorf("loading --flat-left: %w", err)
		}
		images.FlatLeft = gray
		hasAny = true
	}

	if atfImagesFlags.flatThumbs != "" {
		gray, err := loadGrayImage(atfImagesFlags.flatThumbs)
		if err != nil {
			return fmt.Errorf("loading --flat-thumbs: %w", err)
		}
		images.FlatThumbs = gray
		hasAny = true
	}

	if !hasAny {
		return fmt.Errorf("at least one fingerprint image is required (use --rolled-N or --flat-*)")
	}

	data, err := eft.CreateATFTransactionFromImages(images, eft.ATFSubmissionOptions{
		Person: eft.ATFPersonInfo{
			LastName:     atfImagesFlags.lastName,
			FirstName:    atfImagesFlags.firstName,
			MiddleName:   atfImagesFlags.middleName,
			DOB:          dob,
			Sex:          atfImagesFlags.sex,
			Race:         atfImagesFlags.race,
			PlaceOfBirth: atfImagesFlags.pob,
			Citizenship:  atfImagesFlags.citizenship,
			Height:       atfImagesFlags.height,
			Weight:       atfImagesFlags.weight,
			EyeColor:     atfImagesFlags.eyeColor,
			HairColor:    atfImagesFlags.hairColor,
			SSN:          atfImagesFlags.ssn,
			Address:      atfImagesFlags.address,
		},
		ControlNumber: atfImagesFlags.tcn,
		Date:          date,
		Compressor:    comp,
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(atfImagesFlags.output, data, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes, %.1f MB)\n",
		atfImagesFlags.output, len(data), float64(len(data))/(1024*1024))
	if len(data) > eft.ATFMaxFileSize {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: file exceeds ATF 12 MB limit\n")
	}
	return nil
}

func loadGrayImage(path string) (*image.Gray, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decoding %s: %w", path, err)
	}

	if g, ok := img.(*image.Gray); ok {
		return g, nil
	}

	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}
	return gray, nil
}
