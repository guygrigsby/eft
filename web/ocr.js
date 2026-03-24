// ocr.js — Lazy-loaded OCR module using tesseract.js.
// This file is dynamically imported when the user clicks "Try OCR".
//
// Strategy: Crop each demographic field individually via Go WASM (matching
// the CLI approach), then OCR each small image with tesseract.js in
// single-line mode (PSM 7). This is far more reliable than OCR'ing the
// entire header and trying to parse the output with regexes.

let worker = null;

async function initTesseract() {
  if (worker) return worker;

  const mod = await import(
    'https://cdn.jsdelivr.net/npm/tesseract.js@5/dist/tesseract.esm.min.js'
  );
  const createWorker = mod.createWorker || (mod.default && mod.default.createWorker);
  if (!createWorker) {
    throw new Error('Could not find createWorker in tesseract.js module');
  }
  worker = await createWorker('eng');
  return worker;
}

// ocrImage runs tesseract on a base64 data URI and returns trimmed text.
async function ocrImage(tess, dataUri) {
  const resp = await fetch(dataUri);
  const blob = await resp.blob();
  const result = await tess.recognize(blob, {}, {
    tessedit_pageseg_mode: '7', // Single text line.
  });
  return result.data.text.trim();
}

// runOCR takes card image bytes, crops each header field via WASM,
// runs tesseract.js on each field individually, and normalizes via WASM.
export async function runOCR(cardImageBytes) {
  // Step 1: Crop individual header fields using Go WASM.
  const fieldsJSON = await eftCropHeaderFields(cardImageBytes);
  const fieldImages = JSON.parse(fieldsJSON);

  // Step 2: Initialize tesseract.
  const tess = await initTesseract();

  // Step 3: OCR each field image individually.
  const fieldNames = [
    'name', 'address', 'dob', 'sex', 'race', 'height', 'weight',
    'eye_color', 'hair_color', 'place_of_birth', 'citizenship', 'ssn',
  ];

  const raw = {};
  for (const name of fieldNames) {
    if (fieldImages[name]) {
      raw[name] = await ocrImage(tess, fieldImages[name]);
    }
  }

  // Step 4: Normalize via Go WASM.
  const normalizedJSON = await eftNormalizeDemographics(JSON.stringify(raw));
  return JSON.parse(normalizedJSON);
}
