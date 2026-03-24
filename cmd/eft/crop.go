package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"os"
	"path/filepath"

	"github.com/guygrigsby/eft/pkg/eft"
	"github.com/spf13/cobra"
)

var cropFlags struct {
	outputDir string
}

var cropCmd = &cobra.Command{
	Use:   "crop <card-image>",
	Short: "Crop an FD-258 card into individual fingerprint images",
	Long: `Extract individual fingerprint images from a scanned FD-258 card.
Outputs 13 PNG files: 10 rolled prints and 3 flat/slap prints.

Output files:
  rolled_01_right_thumb.png  through  rolled_10_left_little.png
  flat_right_four.png
  flat_left_four.png
  flat_both_thumbs.png

Examples:
  eft crop -o ./prints card_scan.png
  eft crop --output-dir /tmp/fingerprints fd258.jpg`,
	Args: cobra.ExactArgs(1),
	RunE: runCrop,
}

func init() {
	cropCmd.Flags().StringVarP(&cropFlags.outputDir, "output-dir", "o", ".", "output directory for cropped images")
}

var fingerNames = [10]string{
	"right_thumb",
	"right_index",
	"right_middle",
	"right_ring",
	"right_little",
	"left_thumb",
	"left_index",
	"left_middle",
	"left_ring",
	"left_little",
}

func runCrop(cmd *cobra.Command, args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return fmt.Errorf("opening card image: %w", err)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decoding card image: %w", err)
	}

	layout := eft.DefaultFD258Layout()
	images, err := eft.CropFD258(img, layout)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cropFlags.outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	count := 0

	for i := range 10 {
		if images.Rolled[i] == nil {
			continue
		}
		name := fmt.Sprintf("rolled_%02d_%s.png", i+1, fingerNames[i])
		path := filepath.Join(cropFlags.outputDir, name)
		if err := savePNG(path, images.Rolled[i]); err != nil {
			return fmt.Errorf("saving %s: %w", name, err)
		}
		count++
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%dx%d)\n", name,
			images.Rolled[i].Bounds().Dx(), images.Rolled[i].Bounds().Dy())
	}

	flats := []struct {
		img  *image.Gray
		name string
	}{
		{images.FlatRight, "flat_right_four.png"},
		{images.FlatLeft, "flat_left_four.png"},
		{images.FlatThumbs, "flat_both_thumbs.png"},
	}

	for _, flat := range flats {
		if flat.img == nil {
			continue
		}
		path := filepath.Join(cropFlags.outputDir, flat.name)
		if err := savePNG(path, flat.img); err != nil {
			return fmt.Errorf("saving %s: %w", flat.name, err)
		}
		count++
		fmt.Fprintf(cmd.OutOrStdout(), "  %s (%dx%d)\n", flat.name,
			flat.img.Bounds().Dx(), flat.img.Bounds().Dy())
	}

	fmt.Fprintf(cmd.OutOrStdout(), "cropped %d images to %s\n", count, cropFlags.outputDir)
	return nil
}

func savePNG(path string, img *image.Gray) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
