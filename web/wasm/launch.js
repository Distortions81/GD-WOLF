const splash = document.getElementById("splash");
const shell = document.getElementById("game-shell");
const startButton = document.getElementById("start-button");
const fullscreenButton = document.getElementById("fullscreen-button");
const buildPill = document.getElementById("build-pill");
const statusAnnouncer = document.getElementById("status-announcer");

const browserSaveSlotPrefix = "gd-wolf.savegame.";
const saveFileExtension = ".sav";
const saveFileMagic = "GWLF";
const saveFooterMagic = "B3CK";
const exportAllSavesChoice = "__all__";

let splashDismissed = false;
let crc32Table;

function announceStatus(message) {
  if (!statusAnnouncer) {
    return;
  }
  statusAnnouncer.textContent = "";
  window.setTimeout(() => {
    statusAnnouncer.textContent = message;
  }, 0);
}

function getBuildID() {
  return typeof window.__gdwolfBuildID === "string" ? window.__gdwolfBuildID : "";
}

function isMobileLike() {
  if (typeof navigator === "undefined") {
    return false;
  }
  return /Android|iPad|iPhone|iPod/i.test(navigator.userAgent) || (
    navigator.platform === "MacIntel" && navigator.maxTouchPoints > 1
  );
}

function getPlayerURL() {
  const url = new URL("./player.html", window.location.href);
  const buildID = getBuildID();
  if (buildID) {
    url.searchParams.set("v", buildID);
  }
  return url.toString();
}

function isInteractiveTarget(target) {
  if (!(target instanceof Element)) {
    return false;
  }
  return Boolean(target.closest("a, button, input, select, textarea, summary, [role='button'], [tabindex]"));
}

function hideSplash() {
  if (splashDismissed || !splash) {
    return;
  }
  splashDismissed = true;
  splash.hidden = true;
}

function focusPlayer() {
  if (!shell) {
    return;
  }
  try {
    shell.focus({ preventScroll: true });
  } catch (_err) {
  }
  if (!shell.contentWindow) {
    return;
  }
  try {
    shell.contentWindow.focus();
    shell.contentWindow.postMessage({ type: "gdwolf-claim-focus" }, window.location.origin);
  } catch (_err) {
  }
}

async function requestFullscreen() {
  if (!shell) {
    return;
  }
  const target = shell;
  const request = target.requestFullscreen || target.webkitRequestFullscreen;
  if (typeof request !== "function") {
    return;
  }
  try {
    await request.call(target);
    focusPlayer();
  } catch (_err) {
  }
}

function claimFocusAndStart() {
  hideSplash();
  focusPlayer();
}

function initializePlayerFrame() {
  if (!shell) {
    return;
  }
  const target = getPlayerURL();
  if (shell.src !== target) {
    shell.src = target;
  }
}

function updateBuildPill() {
  if (!buildPill) {
    return;
  }
  const buildID = getBuildID();
  buildPill.textContent = buildID ? `Build: ${buildID}` : "Build: local";
}

function listSaveStorageKeys() {
  if (typeof window.localStorage === "undefined" || window.localStorage === null) {
    return [];
  }
  const keys = [];
  for (let i = 0; i < window.localStorage.length; i += 1) {
    const key = window.localStorage.key(i);
    if (typeof key !== "string") {
      continue;
    }
    if (!key.startsWith(browserSaveSlotPrefix) || !key.endsWith(saveFileExtension)) {
      continue;
    }
    keys.push(key);
  }
  keys.sort((a, b) => b.localeCompare(a));
  return keys;
}

function trimSaveStorageKey(key) {
  if (!key.startsWith(browserSaveSlotPrefix)) {
    return key;
  }
  return key.slice(browserSaveSlotPrefix.length);
}

function describeSaveStorageKey(key, index) {
  return `${index + 1}. ${trimSaveStorageKey(key)}`;
}

function bytesToBase64(bytes) {
  let binary = "";
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    const chunk = bytes.subarray(offset, Math.min(offset + chunkSize, bytes.length));
    binary += String.fromCharCode(...chunk);
  }
  return window.btoa(binary);
}

function base64ToBytes(base64) {
  const binary = window.atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function decodeASCII(bytes, start, length) {
  let text = "";
  for (let i = 0; i < length; i += 1) {
    text += String.fromCharCode(bytes[start + i]);
  }
  return text;
}

function encodeASCII(text) {
  const bytes = new Uint8Array(text.length);
  for (let i = 0; i < text.length; i += 1) {
    bytes[i] = text.charCodeAt(i) & 0xff;
  }
  return bytes;
}

function getCRC32Table() {
  if (crc32Table) {
    return crc32Table;
  }
  crc32Table = new Uint32Array(256);
  for (let i = 0; i < 256; i += 1) {
    let crc = i;
    for (let bit = 0; bit < 8; bit += 1) {
      crc = (crc & 1) !== 0 ? (0xedb88320 ^ (crc >>> 1)) : (crc >>> 1);
    }
    crc32Table[i] = crc >>> 0;
  }
  return crc32Table;
}

function crc32(bytes) {
  const table = getCRC32Table();
  let crc = 0xffffffff;
  for (let i = 0; i < bytes.length; i += 1) {
    crc = table[(crc ^ bytes[i]) & 0xff] ^ (crc >>> 8);
  }
  return (crc ^ 0xffffffff) >>> 0;
}

function concatUint8Arrays(chunks) {
  let total = 0;
  for (const chunk of chunks) {
    total += chunk.length;
  }
  const out = new Uint8Array(total);
  let offset = 0;
  for (const chunk of chunks) {
    out.set(chunk, offset);
    offset += chunk.length;
  }
  return out;
}

function writeUint16LE(target, offset, value) {
  target[offset] = value & 0xff;
  target[offset + 1] = (value >>> 8) & 0xff;
}

function writeUint32LE(target, offset, value) {
  target[offset] = value & 0xff;
  target[offset + 1] = (value >>> 8) & 0xff;
  target[offset + 2] = (value >>> 16) & 0xff;
  target[offset + 3] = (value >>> 24) & 0xff;
}

function createStoredZIP(entries) {
  const localParts = [];
  const centralParts = [];
  let offset = 0;

  for (const entry of entries) {
    const fileNameBytes = encodeASCII(entry.name);
    const fileData = entry.data;
    const checksum = crc32(fileData);

    const localHeader = new Uint8Array(30 + fileNameBytes.length);
    writeUint32LE(localHeader, 0, 0x04034b50);
    writeUint16LE(localHeader, 4, 20);
    writeUint16LE(localHeader, 6, 0);
    writeUint16LE(localHeader, 8, 0);
    writeUint16LE(localHeader, 10, 0);
    writeUint16LE(localHeader, 12, 0);
    writeUint32LE(localHeader, 14, checksum);
    writeUint32LE(localHeader, 18, fileData.length);
    writeUint32LE(localHeader, 22, fileData.length);
    writeUint16LE(localHeader, 26, fileNameBytes.length);
    writeUint16LE(localHeader, 28, 0);
    localHeader.set(fileNameBytes, 30);
    localParts.push(localHeader, fileData);

    const centralHeader = new Uint8Array(46 + fileNameBytes.length);
    writeUint32LE(centralHeader, 0, 0x02014b50);
    writeUint16LE(centralHeader, 4, 20);
    writeUint16LE(centralHeader, 6, 20);
    writeUint16LE(centralHeader, 8, 0);
    writeUint16LE(centralHeader, 10, 0);
    writeUint16LE(centralHeader, 12, 0);
    writeUint16LE(centralHeader, 14, 0);
    writeUint32LE(centralHeader, 16, checksum);
    writeUint32LE(centralHeader, 20, fileData.length);
    writeUint32LE(centralHeader, 24, fileData.length);
    writeUint16LE(centralHeader, 28, fileNameBytes.length);
    writeUint16LE(centralHeader, 30, 0);
    writeUint16LE(centralHeader, 32, 0);
    writeUint16LE(centralHeader, 34, 0);
    writeUint16LE(centralHeader, 36, 0);
    writeUint32LE(centralHeader, 38, 0);
    writeUint32LE(centralHeader, 42, offset);
    centralHeader.set(fileNameBytes, 46);
    centralParts.push(centralHeader);

    offset += localHeader.length + fileData.length;
  }

  const centralDirectory = concatUint8Arrays(centralParts);
  const endRecord = new Uint8Array(22);
  writeUint32LE(endRecord, 0, 0x06054b50);
  writeUint16LE(endRecord, 4, 0);
  writeUint16LE(endRecord, 6, 0);
  writeUint16LE(endRecord, 8, entries.length);
  writeUint16LE(endRecord, 10, entries.length);
  writeUint32LE(endRecord, 12, centralDirectory.length);
  writeUint32LE(endRecord, 16, offset);
  writeUint16LE(endRecord, 20, 0);

  return concatUint8Arrays([...localParts, centralDirectory, endRecord]);
}

function looksLikeSaveFile(bytes) {
  if (!(bytes instanceof Uint8Array)) {
    return false;
  }
  if (bytes.length < saveFileMagic.length + saveFooterMagic.length) {
    return false;
  }
  const footerStart = bytes.length - saveFooterMagic.length - 32;
  if (footerStart < saveFileMagic.length) {
    return false;
  }
  return decodeASCII(bytes, 0, saveFileMagic.length) === saveFileMagic &&
    decodeASCII(bytes, footerStart, saveFooterMagic.length) === saveFooterMagic;
}

function sanitizeImportedSaveName(name) {
  return name
    .replace(/\.[^.]+$/, "")
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^[-_]+|[-_]+$/g, "") || "imported-save";
}

function importedSaveStorageKey(fileName) {
  const stamp = new Date().toISOString()
    .replace(/[-:]/g, "")
    .replace("T", "-")
    .slice(0, 15);
  return `${browserSaveSlotPrefix}${stamp}-${sanitizeImportedSaveName(fileName)}${saveFileExtension}`;
}

function uniqueImportedSaveStorageKey(fileName) {
  const base = importedSaveStorageKey(fileName);
  let candidate = base;
  for (let i = 1; window.localStorage.getItem(candidate) !== null; i += 1) {
    candidate = `${base.slice(0, -saveFileExtension.length)}-${i}${saveFileExtension}`;
  }
  return candidate;
}

function chooseSaveKeyForExport(keys) {
  if (keys.length === 0) {
    window.alert("No browser saves are available to export.");
    return "";
  }
  if (keys.length === 1) {
    return keys[0];
  }
  const response = window.prompt(
    `Export which save?\n\n0. All saves (.zip)\n${keys.map(describeSaveStorageKey).join("\n")}\n\nEnter a number:`,
    "0",
  );
  if (response === null) {
    return "";
  }
  if (response.trim() === "0") {
    return exportAllSavesChoice;
  }
  const index = Number.parseInt(response, 10) - 1;
  if (!Number.isInteger(index) || index < 0 || index >= keys.length) {
    window.alert("Save export canceled: enter one of the listed numbers.");
    return "";
  }
  return keys[index];
}

function downloadBlob(blob, fileName) {
  const objectURL = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = objectURL;
  link.download = fileName;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.setTimeout(() => URL.revokeObjectURL(objectURL), 1000);
}

function notifyPlayerSaveImported(key) {
  if (!shell || !shell.contentWindow || typeof shell.contentWindow.gdwolfOnSaveImport !== "function") {
    return;
  }
  try {
    shell.contentWindow.gdwolfOnSaveImport(key);
  } catch (_err) {
  }
}

function exportSaveByStorageKey(key) {
  if (typeof key !== "string" || key.length === 0) {
    window.alert("The selected browser save is missing.");
    return false;
  }
  const stored = window.localStorage.getItem(key);
  if (typeof stored !== "string" || stored.length === 0) {
    window.alert("The selected browser save is missing.");
    return false;
  }
  let bytes;
  try {
    bytes = base64ToBytes(stored);
  } catch (_err) {
    window.alert("The selected browser save could not be decoded.");
    return false;
  }
  downloadBlob(new Blob([bytes], { type: "application/octet-stream" }), trimSaveStorageKey(key));
  announceStatus(`Exported ${trimSaveStorageKey(key)}.`);
  focusPlayer();
  return true;
}

function exportAllSaves() {
  const keys = listSaveStorageKeys();
  if (keys.length === 0) {
    window.alert("No browser saves are available to export.");
    return false;
  }
  const entries = [];
  for (const saveKey of keys) {
    const stored = window.localStorage.getItem(saveKey);
    if (typeof stored !== "string" || stored.length === 0) {
      continue;
    }
    try {
      entries.push({
        name: trimSaveStorageKey(saveKey),
        data: base64ToBytes(stored),
      });
    } catch (_err) {
    }
  }
  if (entries.length === 0) {
    window.alert("No browser saves could be exported.");
    return false;
  }
  const stamp = new Date().toISOString().replace(/[-:]/g, "").replace("T", "-").slice(0, 15);
  const zipBytes = createStoredZIP(entries);
  downloadBlob(new Blob([zipBytes], { type: "application/zip" }), `gdwolf-saves-${stamp}.zip`);
  announceStatus(`Exported ${entries.length} saves as a zip.`);
  focusPlayer();
  return true;
}

function exportSelectedSave() {
  const keys = listSaveStorageKeys();
  const key = chooseSaveKeyForExport(keys);
  if (!key) {
    return;
  }
  if (key === exportAllSavesChoice) {
    exportAllSaves();
    return;
  }
  exportSaveByStorageKey(key);
}

function importSaveFromFile(file) {
  if (!(file instanceof File)) {
    return;
  }
  file.arrayBuffer().then((buffer) => {
    const bytes = new Uint8Array(buffer);
    if (!looksLikeSaveFile(bytes)) {
      window.alert("That file does not look like a GD-WOLF .sav file.");
      return;
    }
    const key = uniqueImportedSaveStorageKey(file.name);
    window.localStorage.setItem(key, bytesToBase64(bytes));
    announceStatus(`Imported ${trimSaveStorageKey(key)}.`);
    notifyPlayerSaveImported(key);
    focusPlayer();
  }).catch(() => {
    window.alert("Save import failed while reading the selected file.");
  });
}

function openSaveImportPicker() {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = ".sav,application/octet-stream";
  input.addEventListener("change", () => {
    const [file] = input.files || [];
    if (!file) {
      return;
    }
    importSaveFromFile(file);
  }, { once: true });
  input.click();
}

window.gdwolfImportSave = function gdwolfImportSave() {
  openSaveImportPicker();
};

window.gdwolfExportSave = function gdwolfExportSave(key) {
  return exportSaveByStorageKey(key);
};

window.gdwolfExportAllSaves = function gdwolfExportAllSaves() {
  return exportAllSaves();
};

updateBuildPill();
initializePlayerFrame();

if (isMobileLike()) {
  window.location.replace(getPlayerURL());
}

if (splash) {
  splash.addEventListener("click", (event) => {
    if (isInteractiveTarget(event.target)) {
      return;
    }
    event.preventDefault();
    claimFocusAndStart();
  });
  splash.addEventListener("touchstart", (event) => {
    if (isInteractiveTarget(event.target)) {
      return;
    }
    event.preventDefault();
    claimFocusAndStart();
  }, { passive: false });
}

if (startButton) {
  startButton.addEventListener("click", () => {
    claimFocusAndStart();
  });
}

if (fullscreenButton) {
  fullscreenButton.addEventListener("click", () => {
    requestFullscreen();
  });
}

window.addEventListener("keydown", (event) => {
  if (isInteractiveTarget(event.target)) {
    return;
  }
  if (event.key !== "Enter" && event.key !== " " && event.key !== "Spacebar") {
    return;
  }
  if (splashDismissed) {
    return;
  }
  event.preventDefault();
  claimFocusAndStart();
});

window.addEventListener("message", (event) => {
  if (event.origin !== window.location.origin || !event.data) {
    return;
  }
  if (event.data.type !== "gdwolf-player-ready") {
    return;
  }
  window.requestAnimationFrame(() => {
    focusPlayer();
  });
});

document.addEventListener("fullscreenchange", () => {
  if (document.fullscreenElement) {
    window.requestAnimationFrame(() => {
      focusPlayer();
    });
    return;
  }
  if (splashDismissed) {
    window.requestAnimationFrame(() => {
      focusPlayer();
    });
  }
});
