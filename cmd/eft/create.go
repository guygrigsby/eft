package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/guygrigsby/eft/pkg/eft"
	"github.com/spf13/cobra"
)

var createFlags struct {
	transactionType   string
	dai               string
	ori               string
	tcn               string
	domainName        string
	domainVersion     string
	version           string
	date              string
	output            string
	compression       string
	ppi               int
	impressionType    int
	fingerImages      []string // format: "position:file" e.g. "1:thumb.png"
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an ANSI/NIST-ITL transaction from fingerprint images",
	Long: `Create a generic ANSI/NIST-ITL (EFT) transaction file from one or more
fingerprint images. Each image is specified with --finger as "position:file"
where position is the finger number (1-15).

Finger positions:
  1=R.Thumb  2=R.Index  3=R.Middle  4=R.Ring   5=R.Little
  6=L.Thumb  7=L.Index  8=L.Middle  9=L.Ring  10=L.Little
  13=R.Four  14=L.Four  15=Both Thumbs

Examples:
  eft create -t CAR --dai WVFBI0000 --ori WV1234567 --tcn TCN001 \
    --finger 1:thumb.png --finger 2:index.png -o output.eft

  eft create -t FAUF --dai WVIAFIS0Z --ori WVATF0800 --tcn TCN002 \
    --compression wsq --finger 1:thumb.png -o output.eft`,
	RunE: runCreate,
}

func init() {
	f := createCmd.Flags()
	f.StringVarP(&createFlags.transactionType, "type", "t", "", "transaction type code (e.g. CAR, CNA, FAUF) (required)")
	f.StringVar(&createFlags.dai, "dai", "", "destination agency identifier (required)")
	f.StringVar(&createFlags.ori, "ori", "", "originating agency identifier (required)")
	f.StringVar(&createFlags.tcn, "tcn", "", "transaction control number (required)")
	f.StringVar(&createFlags.domainName, "domain", "", "domain name (e.g. NORAM)")
	f.StringVar(&createFlags.domainVersion, "domain-version", "", "domain version (e.g. 11.1)")
	f.StringVar(&createFlags.version, "version", "", "ANSI/NIST-ITL version (default 0502)")
	f.StringVar(&createFlags.date, "date", "", "transaction date YYYY-MM-DD (default today)")
	f.StringVarP(&createFlags.output, "output", "o", "", "output EFT file path (required)")
	f.StringVarP(&createFlags.compression, "compression", "c", "wsq", "compression: wsq or none")
	f.IntVar(&createFlags.ppi, "ppi", 500, "pixels per inch")
	f.IntVar(&createFlags.impressionType, "impression", 0, "impression type (0=livescan plain, 1=livescan rolled, 2=nonlive plain, 3=nonlive rolled)")
	f.StringArrayVar(&createFlags.fingerImages, "finger", nil, "finger image as position:file (repeatable, required)")

	createCmd.MarkFlagRequired("type")
	createCmd.MarkFlagRequired("dai")
	createCmd.MarkFlagRequired("ori")
	createCmd.MarkFlagRequired("tcn")
	createCmd.MarkFlagRequired("output")
	createCmd.MarkFlagRequired("finger")
}

func runCreate(cmd *cobra.Command, args []string) error {
	date, err := parseDate(createFlags.date)
	if err != nil {
		return err
	}

	comp, err := parseCompressor(createFlags.compression)
	if err != nil {
		return err
	}

	var images []eft.FingerprintImage
	for _, spec := range createFlags.fingerImages {
		parts := strings.SplitN(spec, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid --finger format %q, expected position:file", spec)
		}

		pos, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("invalid finger position %q: %w", parts[0], err)
		}

		f, err := os.Open(parts[1])
		if err != nil {
			return fmt.Errorf("opening image %s: %w", parts[1], err)
		}
		defer f.Close()

		images = append(images, eft.FingerprintImage{
			FingerPosition: eft.FingerPosition(pos),
			ImpressionType: eft.ImpressionType(createFlags.impressionType),
			Reader:         f,
			Compressor:     comp,
			PixelsPerInch:  createFlags.ppi,
		})
	}

	data, err := eft.CreateTransaction(
		eft.TransactionOptions{
			TransactionType:   createFlags.transactionType,
			DestinationAgency: createFlags.dai,
			OriginatingAgency: createFlags.ori,
			ControlNumber:     createFlags.tcn,
			DomainName:        createFlags.domainName,
			DomainVersion:     createFlags.domainVersion,
			Version:           createFlags.version,
			Date:              date,
		},
		images,
	)
	if err != nil {
		return err
	}

	if err := os.WriteFile(createFlags.output, data, 0644); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d bytes)\n", createFlags.output, len(data))
	return nil
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Now(), nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q, expected YYYY-MM-DD: %w", s, err)
	}
	return t, nil
}

func parseCompressor(s string) (eft.Compressor, error) {
	switch strings.ToLower(s) {
	case "wsq":
		return eft.DefaultCompressor(), nil
	case "none":
		return &eft.NoneCompressor{}, nil
	default:
		return nil, fmt.Errorf("unknown compression %q, use wsq or none", s)
	}
}
