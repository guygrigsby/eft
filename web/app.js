// app.js — Core logic for the EFT Generator web app.
// All processing runs in-browser via Go WASM. No server calls.

let cardImageBytes = null; // Raw file bytes stored after upload.

const FINGER_NAMES = [
  'R. Thumb', 'R. Index', 'R. Middle', 'R. Ring', 'R. Little',
  'L. Thumb', 'L. Index', 'L. Middle', 'L. Ring', 'L. Little',
];
const ATF_MAX_SIZE = 12 * 1024 * 1024;

// --- WASM initialization ---

async function initWasm() {
  const go = new Go();
  const result = await WebAssembly.instantiateStreaming(
    fetch('eft.wasm'), go.importObject
  );
  go.run(result.instance);
}

window.onEftReady = function () {
  document.getElementById('wasm-loading').classList.add('hidden');
  document.getElementById('step-upload').classList.remove('disabled');
};

initWasm().catch(err => {
  document.getElementById('wasm-loading').innerHTML =
    `<div class="status error">Failed to load EFT engine: ${err.message}</div>`;
});

// --- File upload ---

const dropZone = document.getElementById('drop-zone');
const fileInput = document.getElementById('file-input');

dropZone.addEventListener('click', () => fileInput.click());
dropZone.addEventListener('dragover', e => { e.preventDefault(); dropZone.classList.add('drag-over'); });
dropZone.addEventListener('dragleave', () => dropZone.classList.remove('drag-over'));
dropZone.addEventListener('drop', e => {
  e.preventDefault();
  dropZone.classList.remove('drag-over');
  if (e.dataTransfer.files.length) handleFile(e.dataTransfer.files[0]);
});
fileInput.addEventListener('change', () => {
  if (fileInput.files.length) handleFile(fileInput.files[0]);
});

async function handleFile(file) {
  if (!file.type.match(/^image\/(png|jpeg)$/)) {
    showStatus('upload-status', 'Please upload a PNG or JPEG image.', 'error');
    return;
  }

  showStatus('upload-status', 'Processing card image...', '');
  dropZone.querySelector('p').textContent = file.name;

  const arrayBuffer = await file.arrayBuffer();
  cardImageBytes = new Uint8Array(arrayBuffer);

  try {
    const jsonStr = await eftCropFD258(cardImageBytes);
    const crops = JSON.parse(jsonStr);
    displayPreviews(crops);
    showStatus('upload-status', '', '');
  } catch (err) {
    showStatus('upload-status', `Error cropping card: ${err.message}`, 'error');
  }
}

// --- Preview display ---

function displayPreviews(crops) {
  const row1 = document.getElementById('rolled-row-1');
  const row2 = document.getElementById('rolled-row-2');
  const flatRow = document.getElementById('flat-row');
  row1.innerHTML = '';
  row2.innerHTML = '';
  flatRow.innerHTML = '';

  // Rolled prints: row 1 = fingers 1-5, row 2 = fingers 6-10.
  for (let i = 0; i < 5; i++) {
    row1.appendChild(makePreviewCell(crops.rolled[i], FINGER_NAMES[i]));
  }
  for (let i = 5; i < 10; i++) {
    row2.appendChild(makePreviewCell(crops.rolled[i], FINGER_NAMES[i]));
  }

  // Flat prints.
  if (crops.flatLeft) {
    flatRow.appendChild(makePreviewCell(crops.flatLeft, 'Left Four', 'flat-preview'));
  }
  if (crops.flatThumbs) {
    flatRow.appendChild(makePreviewCell(crops.flatThumbs, 'Both Thumbs', 'flat-preview thumbs'));
  }
  if (crops.flatRight) {
    flatRow.appendChild(makePreviewCell(crops.flatRight, 'Right Four', 'flat-preview'));
  }

  document.getElementById('step-preview').classList.remove('hidden');
  document.getElementById('step-demographics').classList.remove('hidden');
  document.getElementById('step-generate').classList.remove('hidden');
}

function makePreviewCell(dataUri, label, extraClass) {
  const cell = document.createElement('div');
  cell.className = 'preview-cell' + (extraClass ? ' ' + extraClass : '');
  const img = document.createElement('img');
  img.src = dataUri || '';
  img.alt = label;
  const lbl = document.createElement('div');
  lbl.className = 'label';
  lbl.textContent = label;
  cell.appendChild(img);
  cell.appendChild(lbl);
  return cell;
}

// --- Demographics form ---

function collectDemographics() {
  const heightFt = document.getElementById('heightFt').value;
  const heightIn = document.getElementById('heightIn').value;
  let height = '';
  if (heightFt) {
    const inches = heightIn ? String(heightIn).padStart(2, '0') : '00';
    height = heightFt + inches;
  }

  return {
    lastName: document.getElementById('lastName').value.trim(),
    firstName: document.getElementById('firstName').value.trim(),
    middleName: document.getElementById('middleName').value.trim(),
    dob: document.getElementById('dob').value,
    sex: document.getElementById('sex').value,
    race: document.getElementById('race').value,
    placeOfBirth: document.getElementById('pob').value.trim().toUpperCase(),
    citizenship: document.getElementById('citizenship').value.trim().toUpperCase(),
    height: height,
    weight: document.getElementById('weight').value,
    eyeColor: document.getElementById('eyeColor').value,
    hairColor: document.getElementById('hairColor').value,
    ssn: document.getElementById('ssn').value.trim(),
    address: document.getElementById('address').value.trim(),
    compression: document.getElementById('compression-toggle').checked ? 'wsq' : 'none',
  };
}

function populateForm(data) {
  if (data.lastName) document.getElementById('lastName').value = data.lastName;
  if (data.firstName) document.getElementById('firstName').value = data.firstName;
  if (data.middleName) document.getElementById('middleName').value = data.middleName;
  if (data.dob) document.getElementById('dob').value = data.dob;
  if (data.sex) document.getElementById('sex').value = data.sex;
  if (data.race) document.getElementById('race').value = data.race;
  if (data.placeOfBirth) document.getElementById('pob').value = data.placeOfBirth;
  if (data.citizenship) document.getElementById('citizenship').value = data.citizenship;
  if (data.eyeColor) document.getElementById('eyeColor').value = data.eyeColor;
  if (data.hairColor) document.getElementById('hairColor').value = data.hairColor;
  if (data.ssn) document.getElementById('ssn').value = data.ssn;
  if (data.address) document.getElementById('address').value = data.address;
  if (data.weight) document.getElementById('weight').value = data.weight;
  if (data.height && data.height.length === 3) {
    document.getElementById('heightFt').value = data.height[0];
    document.getElementById('heightIn').value = parseInt(data.height.slice(1), 10);
  }
}

// --- Generate EFT ---

document.getElementById('btn-generate').addEventListener('click', handleGenerate);

async function handleGenerate() {
  const form = document.getElementById('demo-form');
  if (!form.reportValidity()) return;

  if (!cardImageBytes) {
    showStatus('generate-status', 'No card image uploaded.', 'error');
    return;
  }

  const btn = document.getElementById('btn-generate');
  btn.disabled = true;
  showStatus('generate-status', 'Generating EFT file (this may take a moment with WSQ compression)...', '');

  try {
    const demo = collectDemographics();
    const eftData = await eftGenerateEFT(cardImageBytes, JSON.stringify(demo));

    // Show size indicator.
    const size = eftData.length;
    updateSizeIndicator(size);

    // Create download link.
    const blob = new Blob([eftData], { type: 'application/octet-stream' });
    const url = URL.createObjectURL(blob);
    const link = document.getElementById('download-link');
    link.href = url;
    document.getElementById('download-area').classList.remove('hidden');

    if (size > ATF_MAX_SIZE) {
      showStatus('generate-status',
        `Warning: file is ${formatBytes(size)}, exceeds ATF 12 MB limit. Try enabling WSQ compression.`,
        'error');
    } else {
      showStatus('generate-status', `EFT file generated (${formatBytes(size)}).`, 'success');
    }
  } catch (err) {
    showStatus('generate-status', `Error: ${err.message}`, 'error');
  } finally {
    btn.disabled = false;
  }
}

function updateSizeIndicator(size) {
  const indicator = document.getElementById('size-indicator');
  const fill = document.getElementById('size-fill');
  const text = document.getElementById('size-text');
  indicator.classList.remove('hidden');

  const pct = Math.min((size / ATF_MAX_SIZE) * 100, 100);
  fill.style.width = pct + '%';

  if (pct < 83) {
    fill.style.background = 'var(--green)';
  } else if (pct < 96) {
    fill.style.background = 'var(--yellow)';
  } else {
    fill.style.background = 'var(--red)';
  }

  text.textContent = `${formatBytes(size)} / ${formatBytes(ATF_MAX_SIZE)} (${pct.toFixed(0)}%)`;
}

// --- OCR ---

document.getElementById('btn-ocr').addEventListener('click', handleOCR);

async function handleOCR() {
  if (!cardImageBytes) return;

  const btn = document.getElementById('btn-ocr');
  btn.disabled = true;
  showStatus('ocr-status', 'Loading OCR engine (~2 MB download)...', '');

  try {
    // Lazy-load ocr.js.
    const { runOCR } = await import('./ocr.js');
    showStatus('ocr-status', 'Reading card header...', '');

    const normalized = await runOCR(cardImageBytes);
    populateForm(normalized);

    showStatus('ocr-status', 'Demographics extracted. Review and correct if needed.', 'success');
  } catch (err) {
    showStatus('ocr-status', `OCR failed: ${err.message}`, 'error');
  } finally {
    btn.disabled = false;
  }
}

// --- Utilities ---

function showStatus(elementId, message, type) {
  const el = document.getElementById(elementId);
  if (!message) {
    el.innerHTML = '';
    return;
  }
  el.innerHTML = `<div class="status ${type || ''}">${escapeHtml(message)}</div>`;
}

function formatBytes(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}
