// Checks arduino-app-cli's daemon built for js/wasm (cmd/arduino-app-cli/wasmdaemon, task
// build:wasm) in Node, where Go's os package works on Node's own files:
// the daemon's logic, before a browser gives it OPFS instead.
//
//   node scripts/wasmdaemon-check.cjs <arduino-app-cli.wasm> <assets dir of one runner version> <examples dir>
//
// task test:wasm runs it on build/wasm.
//
// It lays the files out as App Lab's page does: the daemon's home is a
// root that holds ArduinoApps, tmp and var/lib/arduino-app-cli side by side. It sets up the
// data dir as the board's package does (the bricks assets, the examples, a platform.json
// for an UNO Q), runs the daemon, and goes through what App Lab asks of a
// board without one: create, list, open, edit, add a brick, export, delete; and what needs
// a board must say so. Then the editor service (the daemon's /v1/edit) on a MessagePort,
// as App Lab's page opens one: the app's files, writes, and the changes a watch reports,
// the editor's own and the daemon's, through Node's fs.watch. EDITOR_DEBUG=1 prints each
// notification of the editor's as it arrives.
"use strict";

const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { execFileSync } = require("node:child_process");

const [wasmPath, assetsDir, examplesDir] = process.argv.slice(2);
if (!wasmPath || !assetsDir || !examplesDir) {
  console.error("usage: node scripts/wasmdaemon-check.cjs <arduino-app-cli.wasm> <assets dir of one runner version> <examples dir>");
  process.exit(2);
}

// Resolved: the editor watches canonical paths, and macOS's tmpdir is a symlink.
const root = fs.mkdtempSync(path.join(fs.realpathSync(os.tmpdir()), "aac-wasm-"));
const home = root;
const data = path.join(root, "var", "lib", "arduino-app-cli");
fs.mkdirSync(path.join(home, "ArduinoApps"), { recursive: true });
fs.mkdirSync(path.join(home, ".arduino-bricks", "models"), { recursive: true });
fs.mkdirSync(path.join(root, "tmp"), { recursive: true });
// The assets dir is named after its runner version, as under the board's data dir.
fs.cpSync(assetsDir, path.join(data, "assets", path.basename(path.resolve(assetsDir))), { recursive: true });
for (const entry of ["core-and-foundational", "inspirational", "bricks", "examples.json"]) {
  fs.cpSync(path.join(examplesDir, entry), path.join(data, "examples", entry), { recursive: true });
}
fs.writeFileSync(path.join(data, "platform.json"), JSON.stringify({ fqbn: "arduino:zephyr:unoq" }));

// As Go's wasm_exec_node.js sets them up.
globalThis.require = require;
globalThis.fs = fs;
globalThis.path = path;
globalThis.TextEncoder = require("node:util").TextEncoder;
globalThis.TextDecoder = require("node:util").TextDecoder;
globalThis.crypto ??= require("node:crypto");
const goroot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf-8" }).trim();
require(path.join(goroot, "lib", "wasm", "wasm_exec.js"));

/** One request through the daemon: the head, then the chunks, then the end. */
function request(method, url, body, { stream = false, contentType = "application/json" } = {}) {
  return new Promise((resolve, reject) => {
    const chunks = [];
    let status = 0;
    let headers = [];
    let cancel;
    const done = (error) => {
      if (error) return reject(new Error(error));
      const bytes = Buffer.concat(chunks);
      const text = bytes.toString("utf8");
      let json;
      try {
        json = JSON.parse(text);
      } catch {}
      resolve({ status, headers, text, json, bytes });
    };
    const sink = {
      head(s, h) {
        status = s;
        headers = h;
        // A stream stays open: what arrived in its first moments is enough here.
        if (stream) setTimeout(() => cancel(), 300);
      },
      chunk(b) {
        chunks.push(Buffer.from(b));
      },
      end(error) {
        done(error);
      },
    };
    const encoded =
      body === undefined ? null : body instanceof Uint8Array ? body : new TextEncoder().encode(typeof body === "string" ? body : JSON.stringify(body));
    cancel = globalThis.arduinoAppCli.serve({ method, url, headers: [["Content-Type", contentType]], body: encoded }, sink);
  });
}

/**
 * An editor session on a MessagePort, as a page opens one: JSON-RPC in strings, null to
 * hang up. changed(match) waits for an fs.changed with an event that matches.
 */
function editorSession() {
  const { port1, port2 } = new MessageChannel();
  const pending = new Map();
  const notes = [];
  const waiting = new Set();
  let nextId = 0;
  port1.addEventListener("message", (event) => {
    const message = JSON.parse(event.data);
    if (message.id !== undefined && pending.has(message.id)) {
      pending.get(message.id)(message);
      pending.delete(message.id);
      return;
    }
    notes.push(message);
    if (process.env.EDITOR_DEBUG) console.log("editor:", event.data.slice(0, 300));
    for (const w of waiting) w();
  });
  port1.start();
  globalThis.arduinoAppCli.editor(port2);
  return {
    call(method, params) {
      const id = ++nextId;
      port1.postMessage(JSON.stringify({ jsonrpc: "2.0", id, method, params }));
      return new Promise((resolve) => pending.set(id, resolve));
    },
    changed(match, ms = 3000) {
      const find = () => notes.find((n) => n.method === "fs.changed" && n.params.events.some(match));
      return new Promise((resolve) => {
        const found = find();
        if (found) return resolve(found);
        const timer = setTimeout(() => (waiting.delete(w), resolve(undefined)), ms);
        const w = () => {
          const n = find();
          if (n) (clearTimeout(timer), waiting.delete(w), resolve(n));
        };
        waiting.add(w);
      });
    },
    close() {
      port1.postMessage(null);
      port1.close();
    },
  };
}

const results = [];
function info(name, detail) {
  console.log(`INFO  ${name}  (${detail})`);
}
function check(name, ok, detail) {
  results.push({ name, ok });
  console.log(`${ok ? "PASS" : "FAIL"}  ${name}${detail ? `  (${detail})` : ""}`);
}

(async () => {
  const go = new Go();
  go.argv = ["wasmdaemon"];
  go.env = {
    HOME: home,
    TMPDIR: path.join(root, "tmp"),
    ARDUINO_APP_CLI__DATA_DIR: data,
    ARDUINO_APP_CLI__APPS_DIR: path.join(home, "ArduinoApps"),
  };
  const ready = new Promise((resolve) => (globalThis.arduinoAppCliReady = resolve));
  const started = performance.now();
  const { instance } = await WebAssembly.instantiate(fs.readFileSync(wasmPath), go.importObject);
  go.run(instance).then(() => console.log("the daemon exited"));
  await ready;
  console.log(`daemon up in ${Math.round(performance.now() - started)} ms (${globalThis.arduinoAppCli.version}), data in ${root}`);

  let r = await request("GET", "/v1/version");

  // A page whose stream is already closed throws on the next chunk, as a cancelled
  // ReadableStream does: the request fails, the daemon keeps answering.
  await new Promise((resolve) => {
    const sink = { head() {}, chunk() { throw new TypeError("Cannot enqueue a chunk into a closed readable stream"); }, end: resolve };
    globalThis.arduinoAppCli.serve({ method: "GET", url: "/v1/version", headers: [], body: null }, sink);
  });
  r = await request("GET", "/v1/version");
  check("a page that throws on a chunk doesn't stop the daemon", r.status === 200, r.text.trim());
  check("version", r.status === 200 && r.json?.version === globalThis.arduinoAppCli.version, r.text.trim());

  r = await request("GET", "/v1/apps?filter=apps");
  check("no app at first", r.status === 200 && (r.json?.apps ?? []).length === 0, r.text.trim().slice(0, 120));

  r = await request("GET", "/v1/apps?filter=examples");
  check("the examples among the apps", r.status === 200 && (r.json?.apps ?? []).length > 0, `${(r.json?.apps ?? []).length} examples`);

  r = await request("POST", "/v1/apps", { name: "My Blink", icon: "💡", description: "made by the daemon in wasm" });
  const id = r.json?.id;
  check("create an app", r.status === 201 && Boolean(id), r.text.trim());
  const appDir = path.join(home, "ArduinoApps", "my-blink");
  check("its files are on disk", fs.existsSync(path.join(appDir, "app.yaml")) && fs.existsSync(path.join(appDir, "python", "main.py")) && fs.existsSync(path.join(appDir, "sketch", "sketch.ino")), fs.existsSync(appDir) ? fs.readdirSync(appDir).join(", ") : "no folder");

  r = await request("GET", "/v1/apps?filter=apps");
  const listed = (r.json?.apps ?? []).find((a) => a.id === id);
  check("listed, never run: without docker no app has a container", r.status === 200 && ["uninitialized", "stopped"].includes(listed?.status), listed ? `status ${listed.status}` : r.text.trim().slice(0, 160));

  r = await request("GET", `/v1/apps/${id}`);
  check("its details", r.status === 200 && r.json?.name === "My Blink", r.text.trim().slice(0, 160));

  r = await request("PATCH", `/v1/apps/${id}`, { description: "edited in wasm" });
  check("edit it", r.status === 200 && fs.readFileSync(path.join(appDir, "app.yaml"), "utf8").includes("edited in wasm"), `${r.status} ${r.text.trim().slice(0, 120)}`);

  r = await request("GET", "/v1/bricks");
  check("the bricks catalog", r.status === 200 && (r.json?.bricks ?? []).length > 20, `${(r.json?.bricks ?? []).length} bricks`);

  r = await request("PUT", `/v1/apps/${id}/bricks/arduino:weather_forecast`, {});
  check("add a brick", r.status === 200 && fs.readFileSync(path.join(appDir, "app.yaml"), "utf8").includes("arduino:weather_forecast"), `${r.status} ${r.text.trim().slice(0, 120)}`);

  r = await request("GET", `/v1/apps/${id}/bricks`);
  check("the app's bricks", r.status === 200 && JSON.stringify(r.json).includes("arduino:weather_forecast"), r.text.trim().slice(0, 120));

  r = await request("GET", "/v1/examples");
  const examples = Array.isArray(r.json) ? r.json : (r.json?.examples ?? r.json?.apps ?? Object.values(r.json ?? {}).flat());
  check("the examples catalog", r.status === 200 && examples.length > 0, `${examples.length} entries: ${r.text.trim().slice(0, 100)}`);

  r = await request("GET", "/v1/models");
  check("the models catalog, none installed", r.status === 200, `${r.status} ${r.text.trim().slice(0, 100)}`);

  r = await request("GET", `/v1/apps/${id}/export`);
  check("export", r.status === 200 && r.text.length > 0, `${r.status}, ${r.text.length} bytes`);
  const exported = r.bytes;

  r = await request("PUT", "/v1/properties/setup-board-name", "This browser");
  const prop = await request("GET", "/v1/properties/setup-board-name");
  check("a property", r.status < 300 && prop.text.includes("This browser"), `${r.status} / ${prop.status} ${prop.text.trim()}`);

  r = await request("GET", "/v1/system/update/check");
  // Under Node, Go uses no fetch (it talks to its own fake network), so the updater finds no
  // internet; in a browser it does, and finds nothing to upgrade.
  check("no update to apply", r.status === 204 || r.status === 200 || r.json?.code === "NO_INTERNET_CONNECTION", `${r.status} ${r.text.trim().slice(0, 80)}`);

  r = await request("POST", `/v1/apps/${id}/start`);
  check("start says it needs a board", /docker|board/i.test(r.text), `${r.status} …${r.text.trim().replace(/\s+/g, " ").slice(-220)}`);

  r = await request("GET", "/v1/apps/events", undefined, { stream: true });
  check("the app events stream opens", r.status === 200, `${r.status}, ${headerValue(r.headers, "content-type")}`);

  r = await request("POST", `/v1/apps/${id}/stop`);
  // Every event stream of the daemon ends with an "error" event of code SERVER_CLOSED, then "close".
  check("stop: nothing runs, so it is stopped", r.status < 300 && /"progress":100/.test(r.text) && !/INTERNAL_SERVER_ERROR/.test(r.text), `${r.status} …${r.text.trim().replace(/\s+/g, " ").slice(-120)}`);

  r = await request("POST", `/v1/apps/${id}/clone`, { name: "My Blink copy" });
  check("clone", r.status === 201 && fs.existsSync(path.join(home, "ArduinoApps", "my-blink-copy", "app.yaml")), `${r.status} ${r.text.trim().slice(0, 100)}`);

  r = await request("GET", `/v1/apps/${id}/sketch/libraries`);
  check("sketch libraries say they need a board", r.status >= 400 && /arduino-cli|board/i.test(r.text), `${r.status} ${r.text.trim().slice(0, 120)}`);

  r = await request("GET", "/v1/system/resources", undefined, { stream: true });
  info("system resources", `${r.status} ${r.text.trim().replace(/\s+/g, " ").slice(0, 160)}`);

  r = await request("GET", "/v1/libraries?search=servo");
  info("libraries search (an Arduino api over the network)", `${r.status} ${r.text.trim().slice(0, 120)}`);

  // The editor service, as App Lab's editor uses a board's.
  const editor = editorSession();
  const event = (type, file) => (e) => e.type === type && e.path === file;
  const short = (n) => (n ? n.params.events.map((e) => `${e.type} ${path.relative(home, e.path)}`).join(", ") : "nothing within 3 s");
  let e = await editor.call("hello", { clientName: "aac-wasm-check", protocol: "editor.v1" });
  check("editor: hello", e.result?.protocol === "editor.v1" && e.result.capabilities?.includes("fs.watch"), JSON.stringify(e.result ?? e.error).slice(0, 140));

  e = await editor.call("fs.walk", { path: appDir });
  const walked = (e.result?.entries ?? []).map((x) => path.relative(appDir, x.path));
  check("editor: walk the app", ["app.yaml", "python/main.py", "sketch/sketch.ino"].every((f) => walked.includes(f)), `${walked.length} entries: ${walked.slice(0, 6).join(", ")}`);

  e = await editor.call("fs.read", { path: path.join(appDir, "app.yaml") });
  const appYaml = e.result;
  check("editor: read app.yaml, as the daemon wrote it", appYaml?.content?.includes("edited in wasm") && Boolean(appYaml.etag), JSON.stringify(e.result ?? e.error).slice(0, 120));

  e = await editor.call("fs.read", { path: path.join(appDir, "app.yaml"), baseEtag: appYaml?.etag });
  check("editor: an unchanged file is not sent again", e.result?.notModified === true, JSON.stringify(e.result ?? e.error).slice(0, 120));

  e = await editor.call("fs.watch", { path: appDir, recursive: true });
  check("editor: watch the app", Boolean(e.result?.subscriptionId), JSON.stringify(e.result ?? e.error));
  e = await editor.call("fs.watch", { path: path.join(home, "ArduinoApps") });
  check("editor: watch the apps folder", Boolean(e.result?.subscriptionId), JSON.stringify(e.result ?? e.error));

  // Node's fs.watch on macOS can report a write to a file made moments before as a rename,
  // since FSEvents' flags add up: in Node, create stands for update too.
  const mainPy = path.join(appDir, "python", "main.py");
  e = await editor.call("fs.write", { path: mainPy, encoding: "utf-8", content: "print('written by the editor')\n" });
  let n = await editor.changed((x) => x.path === mainPy && ["update", "create"].includes(x.type));
  check("editor: a write, then its change", Boolean(e.result?.etag) && fs.readFileSync(mainPy, "utf8").includes("by the editor") && Boolean(n), short(n));

  const helperPy = path.join(appDir, "python", "helper.py");
  await editor.call("fs.write", { path: helperPy, encoding: "utf-8", content: "x = 1\n" });
  n = await editor.changed(event("create", helperPy));
  check("editor: a new file, then its create", Boolean(n), short(n));

  // The daemon saves app.yaml by renaming a temporary file onto it, which inotify reports
  // as a create; Node's fs.watch, guessed by a stat, can make it a rename onto app.yaml.
  r = await request("PATCH", `/v1/apps/${id}`, { description: "edited again, seen by the editor" });
  n = await editor.changed((x) => x.path === path.join(appDir, "app.yaml") && ["update", "create", "rename"].includes(x.type));
  check("editor: the daemon's edit of app.yaml reaches the watch", r.status === 200 && Boolean(n), `${r.status}; ${short(n)}`);

  e = await editor.call("fs.write", { path: path.join(appDir, "app.yaml"), encoding: "utf-8", content: "name: stale\n", baseEtag: appYaml?.etag });
  check("editor: a write on the etag from before the daemon's edit is refused", e.error?.code === -32002, JSON.stringify(e.error ?? e.result).slice(0, 120));

  await editor.call("fs.delete", { path: helperPy });
  n = await editor.changed(event("delete", helperPy));
  check("editor: a delete, then its change", Boolean(n) && !fs.existsSync(helperPy), short(n));

  r = await request("DELETE", `/v1/apps/${id}`);
  check("delete it", r.status < 300 && !fs.existsSync(appDir), `${r.status} ${r.text.trim().slice(0, 120)}`);

  n = await editor.changed((x) => x.type === "delete" && x.path === appDir);
  check("editor: the daemon's delete reaches the apps folder's watch", Boolean(n), short(n));
  editor.close();

  const again = editorSession();
  e = await again.call("hello", { clientName: "aac-wasm-check", protocol: "editor.v1" });
  check("editor: a new session after one hung up", e.result?.protocol === "editor.v1", JSON.stringify(e.result ?? e.error).slice(0, 80));
  again.close();

  // Where the browser's storage can be cleared, apps are kept as files: the export taken above,
  // imported back as a page uploads a file the user picked.
  const boundary = "aac-wasm-check";
  const upload = Buffer.concat([
    Buffer.from(`--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="my-blink.zip"\r\nContent-Type: application/zip\r\n\r\n`),
    exported,
    Buffer.from(`\r\n--${boundary}--\r\n`),
  ]);
  r = await request("POST", "/v1/apps/import", upload, { contentType: `multipart/form-data; boundary=${boundary}` });
  const reimported = fs.existsSync(path.join(appDir, "app.yaml")) ? fs.readFileSync(path.join(appDir, "app.yaml"), "utf8") : "";
  check(
    "import the export back",
    r.status < 300 && reimported.includes("edited in wasm") && reimported.includes("arduino:weather_forecast") && fs.existsSync(path.join(appDir, "python", "main.py")),
    `${r.status} ${r.text.trim().slice(0, 100)}`,
  );

  const failed = results.filter((c) => !c.ok).length;
  console.log(`\n${results.length - failed}/${results.length} passed`);
  process.exit(failed ? 1 : 0);
})().catch((error) => {
  console.error(error);
  process.exit(1);
});

function headerValue(headers, name) {
  const pair = (headers ?? []).find(([k]) => k.toLowerCase() === name);
  return pair ? pair[1] : "";
}
