const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const vm = require("node:vm");

const script = fs.readFileSync(path.join(__dirname, "save-actions.js"), "utf8");
const savePrefix = "gd-wolf.savegame.";

function playerSession(embedded) {
  const storage = new Map();
  const downloads = [];
  const alerts = [];
  const objectURLs = new Map();
  let picker;
  let focusCount = 0;
  const window = {
    localStorage: {
      get length() { return storage.size; },
      key: (index) => [...storage.keys()][index] ?? null,
      getItem: (key) => storage.get(key) ?? null,
      setItem: (key, value) => storage.set(key, value),
    },
    btoa,
    atob,
    alert: (message) => alerts.push(message),
    setTimeout: (callback) => callback(),
  };
  // Embedded saves must work even when the parent has no save bridge.
  window.parent = embedded ? {} : window;
  const document = {
    getElementById: () => ({ textContent: "" }),
    body: { appendChild() {} },
    createElement(tag) {
      if (tag === "input") {
        const listeners = new Map();
        picker = {
          files: [],
          addEventListener: (name, callback) => listeners.set(name, callback),
          click() { this.opened = true; },
          select(file) {
            this.files = [file];
            listeners.get("change")();
          },
        };
        return picker;
      }
      assert.equal(tag, "a");
      return {
        click() {
          downloads.push({ name: this.download, blob: objectURLs.get(this.href) });
        },
        remove() {},
      };
    },
  };
  vm.runInNewContext(script, {
    window, document, File, Blob,
    URL: {
      createObjectURL(blob) {
        const url = `blob:save-${objectURLs.size}`;
        objectURLs.set(url, blob);
        return url;
      },
      revokeObjectURL: (url) => objectURLs.delete(url),
    },
    focusPlayerSurface: () => { focusCount += 1; },
  });
  return { window, storage, downloads, alerts,
    get picker() { return picker; },
    get focusCount() { return focusCount; },
  };
}

for (const embedded of [false, true]) {
  const mode = embedded ? "embedded desktop" : "standalone mobile";

  test(`${mode}: importing stores the file and refreshes the player`, { timeout: 2000 }, async () => {
    const session = playerSession(embedded);
    const bytes = Buffer.concat([Buffer.from("GWLFpayloadB3CK"), Buffer.alloc(32)]);
    let imported;
    for (let attempt = 0; attempt < 2; attempt += 1) {
      const refreshed = new Promise((resolve) => { session.window.gdwolfOnSaveImport = resolve; });
      assert.equal(session.window.gdwolfImportSave(), true);
      assert.equal(session.picker.opened, true);
      session.picker.select(new File([bytes], "Floor 1.sav"));
      const key = await refreshed;
      assert.ok(key.startsWith(savePrefix));
      assert.ok(key.endsWith(".sav"));
      assert.notEqual(key, imported);
      assert.deepEqual(Buffer.from(session.storage.get(key), "base64"), bytes);
      imported = key;
    }
    assert.equal(session.storage.size, 2);
    assert.equal(session.focusCount, 2);
    assert.deepEqual(session.alerts, []);
  });

  test(`${mode}: exporting downloads the original save and an archive`, async () => {
    const session = playerSession(embedded);
    const first = Buffer.from("first saved game");
    const second = Buffer.from("second saved game");
    session.storage.set(`${savePrefix}floor-1.sav`, first.toString("base64"));
    session.storage.set(`${savePrefix}floor-2.sav`, second.toString("base64"));
    session.storage.set("unrelated.setting", "value");
    assert.equal(session.window.gdwolfExportSave(`${savePrefix}floor-1.sav`), true);
    assert.equal(session.downloads[0].name, "floor-1.sav");
    assert.deepEqual(Buffer.from(await session.downloads[0].blob.arrayBuffer()), first);
    assert.equal(session.window.gdwolfExportAllSaves(), true);
    assert.ok(session.downloads[1].name.endsWith(".zip"));
    const archive = Buffer.from(await session.downloads[1].blob.arrayBuffer());
    assert.equal(archive.readUInt32LE(0), 0x04034b50);
    assert.equal(archive.readUInt16LE(archive.length - 12), 2);
    assert.ok(archive.includes(Buffer.from("floor-1.sav")));
    assert.ok(archive.includes(Buffer.from("floor-2.sav")));
    assert.ok(archive.includes(first));
    assert.ok(archive.includes(second));
    assert.equal(session.focusCount, 2);
    assert.deepEqual(session.alerts, []);
  });

  test(`${mode}: a missing save is rejected without a download`, () => {
    const session = playerSession(embedded);
    assert.equal(session.window.gdwolfExportSave(`${savePrefix}missing.sav`), false);
    assert.equal(session.window.gdwolfExportAllSaves(), false);
    assert.equal(session.downloads.length, 0);
    assert.equal(session.alerts.length, 2);
  });
}
