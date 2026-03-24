package main

import (
	"bytes"
	"image"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func testPNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	rng := rand.New(rand.NewSource(42))
	for i := range img.Pix {
		img.Pix[i] = uint8(rng.Intn(256))
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.png")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSmoke_Create(t *testing.T) {
	imgPath := testPNG(t, 100, 100)
	outPath := filepath.Join(t.TempDir(), "out.eft")

	rootCmd.SetArgs([]string{
		"create",
		"-t", "CAR",
		"--dai", "WVFBI0000",
		"--ori", "WV1234567",
		"--tcn", "TCN001",
		"--finger", "2:" + imgPath,
		"--compression", "none",
		"-o", outPath,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

func TestSmoke_ATF(t *testing.T) {
	cardPath := testPNG(t, 1000, 1000)
	outPath := filepath.Join(t.TempDir(), "atf.eft")

	rootCmd.SetArgs([]string{
		"atf",
		"--last-name", "Doe",
		"--first-name", "John",
		"--dob", "1990-01-01",
		"--sex", "M",
		"--race", "W",
		"--compression", "none",
		"-o", outPath,
		cardPath,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}

func TestSmoke_Crop(t *testing.T) {
	cardPath := testPNG(t, 1000, 1000)
	outDir := t.TempDir()

	rootCmd.SetArgs([]string{
		"crop",
		"-o", outDir,
		cardPath,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// Should produce 13 files.
	entries, err := os.ReadDir(outDir)
	if err != nil {
		t.Fatal(err)
	}

	pngCount := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".png" {
			pngCount++
		}
	}

	if pngCount != 13 {
		t.Errorf("expected 13 PNG files, got %d", pngCount)
	}
}

func TestSmoke_ATFImages(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 10 rolled + 3 flat images.
	rolledPaths := make([]string, 10)
	for i := range 10 {
		img := image.NewGray(image.Rect(0, 0, 100, 100))
		rng := rand.New(rand.NewSource(int64(i)))
		for j := range img.Pix {
			img.Pix[j] = uint8(rng.Intn(256))
		}
		var buf bytes.Buffer
		png.Encode(&buf, img)
		path := filepath.Join(tmpDir, "rolled.png")
		// Use unique names.
		path = filepath.Join(tmpDir, "rolled_"+string(rune('0'+i))+".png")
		os.WriteFile(path, buf.Bytes(), 0644)
		rolledPaths[i] = path
	}

	flatRight := testPNG(t, 200, 100)
	flatLeft := testPNG(t, 200, 100)
	flatThumbs := testPNG(t, 130, 100)

	outPath := filepath.Join(tmpDir, "atf_images.eft")

	rootCmd.SetArgs([]string{
		"atf-images",
		"--last-name", "Smith",
		"--first-name", "Jane",
		"--dob", "1985-06-15",
		"--sex", "F",
		"--race", "W",
		"--compression", "none",
		"--rolled-1", rolledPaths[0],
		"--rolled-2", rolledPaths[1],
		"--rolled-6", rolledPaths[5],
		"--flat-right", flatRight,
		"--flat-left", flatLeft,
		"--flat-thumbs", flatThumbs,
		"-o", outPath,
	})

	if err := rootCmd.Execute(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("output file is empty")
	}
}
