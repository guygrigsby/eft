// ocr.js — Lazy-loaded OCR module using tesseract.js.
// This file is dynamically imported when the user clicks "Try OCR".
//
// Strategy: OCR the entire FD-258 header as one image (full-page mode),
// then parse the text output to extract demographic fields. This is more
// robust than cropping individual fields, since the FD-258 header layout
// varies between card printings.

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

// runOCR takes card image bytes, crops the header via WASM,
// runs tesseract.js on the full header, and parses the text.
export async function runOCR(cardImageBytes) {
  // Step 1: Crop the full header area using Go WASM.
  const headerDataUri = await eftCropHeader(cardImageBytes);

  // Step 2: Initialize tesseract.
  const tess = await initTesseract();

  // Step 3: OCR the full header image.
  const resp = await fetch(headerDataUri);
  const blob = await resp.blob();
  const result = await tess.recognize(blob);
  const text = result.data.text;

  // Step 4: Parse the OCR text into demographic fields.
  const parsed = parseHeaderText(text);

  // Step 5: Normalize via Go WASM.
  const normalizedJSON = await eftNormalizeDemographics(JSON.stringify(parsed));
  return JSON.parse(normalizedJSON);
}

// parseHeaderText extracts demographic fields from the full header OCR text.
// The FD-258 header has labeled fields — we look for known labels and extract
// the data that follows them.
function parseHeaderText(text) {
  const raw = {};
  const lines = text.split('\n').map(l => l.trim()).filter(l => l.length > 0);
  const fullText = lines.join('\n');

  // Name: look for text after "LAST NAME" or "NAME" label on early lines.
  // The name is typically on the first or second line of data.
  const nameMatch = fullText.match(/(?:LAST\s*NAME|NAME)[,:\s]*([^\n]+)/i);
  if (nameMatch) {
    // The matched line might contain "FIRST NAME, MIDDLE NAME" labels too.
    let nameLine = nameMatch[1].trim();
    // Remove label fragments.
    nameLine = nameLine
      .replace(/FIRST\s*NAME/gi, '')
      .replace(/MIDDLE\s*NAME/gi, '')
      .replace(/ALIASES?\s*(AKA)?/gi, '')
      .replace(/[|]/g, ',')
      .trim();
    if (nameLine) raw.name = nameLine;
  }

  // If name wasn't found via label, try the first substantial line
  // (often the name is the first line of text).
  if (!raw.name && lines.length > 0) {
    const firstLine = lines[0].replace(/LAST\s*NAME|FIRST\s*NAME|MIDDLE\s*NAME|ALIASES?\s*AKA/gi, '').trim();
    if (firstLine.length > 2 && !/^\d+$/.test(firstLine)) {
      raw.name = firstLine;
    }
  }

  // Address: after "RESIDENCE" label.
  const addrMatch = fullText.match(/RESIDENCE[^:\n]*[:\s]*([^\n]+)/i);
  if (addrMatch) {
    const addr = addrMatch[1].replace(/OF\s*PERSON\s*FINGERPRINTED/gi, '').trim();
    if (addr.length > 2) raw.address = addr;
  }

  // DOB: after "DATE OF BIRTH" or "DOB" label, or a date pattern.
  const dobMatch = fullText.match(/(?:DATE\s*OF\s*BIRTH|DOB)[:\s]*([0-9/\-.\s]{6,10})/i);
  if (dobMatch) {
    raw.dob = dobMatch[1].trim();
  } else {
    // Try to find a date-like pattern in the text.
    const datePattern = fullText.match(/\b(\d{1,2}[/\-]\d{1,2}[/\-]\d{2,4})\b/);
    if (datePattern) raw.dob = datePattern[1];
  }

  // Sex: after "SEX" label.
  const sexMatch = fullText.match(/\bSEX[:\s]*([MFX])\b/i);
  if (sexMatch) raw.sex = sexMatch[1].toUpperCase();

  // Race: after "RACE" label.
  const raceMatch = fullText.match(/\bRACE[:\s]*([A-Z])\b/i);
  if (raceMatch) raw.race = raceMatch[1].toUpperCase();

  // Height: after "HGT" or "HEIGHT" label.
  const hgtMatch = fullText.match(/(?:HGT|HEIGHT)[:\s]*([0-9'"\- ]{2,7})/i);
  if (hgtMatch) raw.height = hgtMatch[1].trim();

  // Weight: after "WGT" or "WEIGHT" label.
  const wgtMatch = fullText.match(/(?:WGT|WEIGHT)[:\s]*(\d{2,3})/i);
  if (wgtMatch) raw.weight = wgtMatch[1];

  // Eye color: after "EYES" or "EYE" label.
  const eyeMatch = fullText.match(/\bEYES?[:\s]*([A-Z]{2,10})\b/i);
  if (eyeMatch) {
    const val = eyeMatch[1].toUpperCase();
    if (val !== 'HAIR' && val !== 'COLOR') raw.eye_color = val;
  }

  // Hair color: after "HAIR" label.
  const hairMatch = fullText.match(/\bHAIR[:\s]*([A-Z]{2,10})\b/i);
  if (hairMatch) {
    const val = hairMatch[1].toUpperCase();
    if (val !== 'COLOR') raw.hair_color = val;
  }

  // Place of birth: after "POB" or "PLACE OF BIRTH" label.
  const pobMatch = fullText.match(/(?:POB|PLACE\s*OF\s*BIRTH)[:\s]*([A-Z]{2})\b/i);
  if (pobMatch) raw.place_of_birth = pobMatch[1].toUpperCase();

  // Citizenship: after "CTZ" or "CITIZENSHIP" label.
  const ctzMatch = fullText.match(/(?:CTZ|CITIZENSHIP)[:\s]*([A-Z]{2,20})/i);
  if (ctzMatch) {
    const val = ctzMatch[1].trim();
    // Filter out label text that might follow.
    if (!/DATE|BIRTH|SEX|RACE/i.test(val)) raw.citizenship = val;
  }

  // SSN: after "SOCIAL SECURITY" or "SOC" label, or a 9-digit / XXX-XX-XXXX pattern.
  const ssnMatch = fullText.match(/(?:SOCIAL\s*SECURITY\s*(?:NO\.?)?|SOC)[:\s#]*([0-9\- ]{9,11})/i);
  if (ssnMatch) {
    raw.ssn = ssnMatch[1].replace(/[\s-]/g, '');
  } else {
    const ssnPattern = fullText.match(/\b(\d{3}-?\d{2}-?\d{4})\b/);
    if (ssnPattern) raw.ssn = ssnPattern[1].replace(/-/g, '');
  }

  return raw;
}
