package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"strings"
)

// TesseractOCR implements eft.OCRProvider using the tesseract CLI.
// Tesseract must be installed and available in PATH.
type TesseractOCR struct{}

// RecognizeText writes the image to a temp file, runs tesseract on it,
// and returns the recognized text.
func (t *TesseractOCR) RecognizeText(ctx context.Context, img image.Image) (string, error) {
	// Write image to a temp PNG file.
	tmpFile, err := os.CreateTemp("", "eft-ocr-*.png")
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := png.Encode(tmpFile, img); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("encoding temp image: %w", err)
	}
	tmpFile.Close()

	// Run tesseract: tesseract <input> stdout --psm 7
	// PSM 7 = "Treat the image as a single text line" — best for
	// individual field crops from a form.
	cmd := exec.CommandContext(ctx, "tesseract", tmpPath, "stdout", "--psm", "7")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tesseract: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	return strings.TrimSpace(stdout.String()), nil
}

// checkTesseract verifies that tesseract is installed and available.
func checkTesseract() error {
	cmd := exec.Command("tesseract", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tesseract not found in PATH: %w\n\nInstall tesseract:\n  macOS:  brew install tesseract\n  Ubuntu: sudo apt install tesseract-ocr\n  Fedora: sudo dnf install tesseract", err)
	}
	return nil
}
