// ocr.js — Lazy-loaded OCR module using tesseract.js.
// This file is dynamically imported when the user clicks "Try OCR".

let worker = null;

async function initTesseract() {
  if (worker) return worker;

  const Tesseract = await import(
    'https://cdn.jsdelivr.net/npm/tesseract.js@5/dist/tesseract.esm.min.js'
  );
  worker = await Tesseract.createWorker('eng');
  return worker;
}

// runOCR takes card image bytes, crops header fields via WASM,
// runs tesseract.js on each field, and returns normalized demographics.
export async function runOCR(cardImageBytes) {
  // Step 1: Crop header fields using Go WASM.
  const fieldsJSON = await eftCropHeaderFields(cardImageBytes);
  const fields = JSON.parse(fieldsJSON);

  // Step 2: Initialize tesseract.
  const tess = await initTesseract();

  // Step 3: OCR each field image.
  const raw = {};
  const fieldNames = Object.keys(fields);
  for (const name of fieldNames) {
    const dataUri = fields[name];
    if (!dataUri) continue;

    try {
      // Convert data URI to blob for tesseract.
      const resp = await fetch(dataUri);
      const blob = await resp.blob();

      const result = await tess.recognize(blob, {
        tessedit_pageseg_mode: '7', // Single text line.
      });
      raw[name] = result.data.text.trim();
    } catch {
      // Skip fields that fail OCR.
    }
  }

  // Step 4: Normalize via Go WASM.
  const normalizedJSON = await eftNormalizeDemographics(JSON.stringify(raw));
  return JSON.parse(normalizedJSON);
}
