//go:build js && wasm

// Package main is the WASM entry point for the EFT library.
// It exposes Go functions to JavaScript for in-browser EFT generation.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"strings"
	"syscall/js"
	"time"

	"github.com/guygrigsby/eft/pkg/eft"
)

func main() {
	js.Global().Set("eftCropFD258", promiseFunc(cropFD258))
	js.Global().Set("eftGenerateEFT", promiseFunc(generateEFT))
	js.Global().Set("eftNormalizeDemographics", promiseFunc(normalizeDemographics))
	js.Global().Set("eftCropHeaderFields", promiseFunc(cropHeaderFields))

	// Signal that WASM is ready.
	if cb := js.Global().Get("onEftReady"); !cb.IsUndefined() && !cb.IsNull() {
		cb.Invoke()
	}

	// Block forever — keep the WASM instance alive.
	select {}
}

// promiseFunc wraps a Go function as a JS function that returns a Promise.
func promiseFunc(fn func(args []js.Value) (interface{}, error)) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		handler := js.FuncOf(func(this js.Value, promArgs []js.Value) interface{} {
			resolve := promArgs[0]
			reject := promArgs[1]
			go func() {
				result, err := fn(args)
				if err != nil {
					reject.Invoke(js.Global().Get("Error").New(err.Error()))
					return
				}
				resolve.Invoke(result)
			}()
			return nil
		})
		return js.Global().Get("Promise").New(handler)
	})
}

// jsBytes copies a JS Uint8Array to a Go []byte.
func jsBytes(val js.Value) []byte {
	length := val.Get("length").Int()
	buf := make([]byte, length)
	js.CopyBytesToGo(buf, val)
	return buf
}

// grayToPNGBase64 encodes a Gray image as a base64 PNG data URI.
func grayToPNGBase64(img *image.Gray) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// cropFD258 takes a card image (Uint8Array) and returns JSON with 13 cropped
// fingerprint preview images as base64 data URIs.
//
// JS: eftCropFD258(imageBytes) → Promise<{rolled: string[], flatRight, flatLeft, flatThumbs}>
func cropFD258(args []js.Value) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("cropFD258: expected 1 argument (image bytes)")
	}

	imgBytes := jsBytes(args[0])
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	layout := eft.DefaultFD258Layout()
	images, err := eft.CropFD258(img, layout)
	if err != nil {
		return nil, err
	}

	// Encode all cropped images as base64 PNGs.
	result := make(map[string]interface{})

	rolled := make([]interface{}, 10)
	for i := 0; i < 10; i++ {
		if images.Rolled[i] == nil {
			rolled[i] = ""
			continue
		}
		b64, err := grayToPNGBase64(images.Rolled[i])
		if err != nil {
			return nil, fmt.Errorf("encoding rolled %d: %w", i+1, err)
		}
		rolled[i] = b64
	}
	result["rolled"] = rolled

	if images.FlatRight != nil {
		b64, err := grayToPNGBase64(images.FlatRight)
		if err != nil {
			return nil, fmt.Errorf("encoding flat right: %w", err)
		}
		result["flatRight"] = b64
	}
	if images.FlatLeft != nil {
		b64, err := grayToPNGBase64(images.FlatLeft)
		if err != nil {
			return nil, fmt.Errorf("encoding flat left: %w", err)
		}
		result["flatLeft"] = b64
	}
	if images.FlatThumbs != nil {
		b64, err := grayToPNGBase64(images.FlatThumbs)
		if err != nil {
			return nil, fmt.Errorf("encoding flat thumbs: %w", err)
		}
		result["flatThumbs"] = b64
	}

	// Marshal to JSON string for JS.
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

// demographicsInput matches the JSON sent from the browser form.
type demographicsInput struct {
	LastName     string `json:"lastName"`
	FirstName    string `json:"firstName"`
	MiddleName   string `json:"middleName"`
	DOB          string `json:"dob"` // YYYY-MM-DD
	Sex          string `json:"sex"`
	Race         string `json:"race"`
	PlaceOfBirth string `json:"placeOfBirth"`
	Citizenship  string `json:"citizenship"`
	Height       string `json:"height"`
	Weight       string `json:"weight"`
	EyeColor     string `json:"eyeColor"`
	HairColor    string `json:"hairColor"`
	SSN          string `json:"ssn"`
	Address      string `json:"address"`
	Compression  string `json:"compression"` // "wsq" or "none"
}

// generateEFT takes a card image (Uint8Array) and demographics JSON string,
// returns the .eft file bytes as a Uint8Array.
//
// JS: eftGenerateEFT(imageBytes, demographicsJSON) → Promise<Uint8Array>
func generateEFT(args []js.Value) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("generateEFT: expected 2 arguments (image bytes, demographics JSON)")
	}

	imgBytes := jsBytes(args[0])
	demoJSON := args[1].String()

	var demo demographicsInput
	if err := json.Unmarshal([]byte(demoJSON), &demo); err != nil {
		return nil, fmt.Errorf("parsing demographics: %w", err)
	}

	// Parse DOB.
	var dob time.Time
	if demo.DOB != "" {
		var err error
		dob, err = time.Parse("2006-01-02", demo.DOB)
		if err != nil {
			return nil, fmt.Errorf("invalid DOB %q: expected YYYY-MM-DD", demo.DOB)
		}
	}

	// Select compressor.
	var comp eft.Compressor
	if strings.ToLower(demo.Compression) == "none" {
		comp = &eft.NoneCompressor{}
	}
	// nil comp → default WSQ

	data, err := eft.CreateATFTransaction(bytes.NewReader(imgBytes), eft.ATFSubmissionOptions{
		Person: eft.ATFPersonInfo{
			LastName:     demo.LastName,
			FirstName:    demo.FirstName,
			MiddleName:   demo.MiddleName,
			DOB:          dob,
			Sex:          demo.Sex,
			Race:         demo.Race,
			PlaceOfBirth: demo.PlaceOfBirth,
			Citizenship:  demo.Citizenship,
			Height:       demo.Height,
			Weight:       demo.Weight,
			EyeColor:     demo.EyeColor,
			HairColor:    demo.HairColor,
			SSN:          demo.SSN,
			Address:      demo.Address,
		},
		Compressor: comp,
	})
	if err != nil {
		return nil, err
	}

	// Return as Uint8Array.
	jsData := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(jsData, data)
	return jsData, nil
}

// normalizeDemographics takes raw OCR text for each field (JSON string)
// and returns normalized demographics (JSON string).
//
// JS: eftNormalizeDemographics(rawJSON) → Promise<normalizedJSON>
func normalizeDemographics(args []js.Value) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("normalizeDemographics: expected 1 argument (raw fields JSON)")
	}

	var raw eft.RawDemographicFields
	if err := json.Unmarshal([]byte(args[0].String()), &raw); err != nil {
		return nil, fmt.Errorf("parsing raw fields: %w", err)
	}

	person := eft.NormalizeRawDemographics(raw)

	// Convert to output format.
	out := demographicsInput{
		LastName:     person.LastName,
		FirstName:    person.FirstName,
		MiddleName:   person.MiddleName,
		Sex:          person.Sex,
		Race:         person.Race,
		PlaceOfBirth: person.PlaceOfBirth,
		Citizenship:  person.Citizenship,
		Height:       person.Height,
		Weight:       person.Weight,
		EyeColor:     person.EyeColor,
		HairColor:    person.HairColor,
		SSN:          person.SSN,
		Address:      person.Address,
	}
	if !person.DOB.IsZero() {
		out.DOB = person.DOB.Format("2006-01-02")
	}

	jsonBytes, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}

// cropHeaderFields takes a card image and returns cropped header field
// images as base64 PNGs for browser-side OCR.
//
// JS: eftCropHeaderFields(imageBytes) → Promise<{name: dataURI, dob: dataURI, ...}>
func cropHeaderFields(args []js.Value) (interface{}, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("cropHeaderFields: expected 1 argument (image bytes)")
	}

	imgBytes := jsBytes(args[0])
	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("decoding image: %w", err)
	}

	fields := eft.DefaultFD258HeaderFields()
	fieldMap := map[string]eft.FractionalRect{
		"name":           fields.Name,
		"dob":            fields.DOB,
		"sex":            fields.Sex,
		"race":           fields.Race,
		"height":         fields.Height,
		"weight":         fields.Weight,
		"eye_color":      fields.EyeColor,
		"hair_color":     fields.HairColor,
		"place_of_birth": fields.PlaceOfBirth,
		"citizenship":    fields.Citizenship,
		"ssn":            fields.SSN,
		"address":        fields.Address,
	}

	result := make(map[string]string)
	for name, rect := range fieldMap {
		cropped := eft.CropHeaderField(img, rect)
		b64, err := grayToPNGBase64(cropped)
		if err != nil {
			return nil, fmt.Errorf("encoding %s: %w", name, err)
		}
		result[name] = b64
	}

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return string(jsonBytes), nil
}
