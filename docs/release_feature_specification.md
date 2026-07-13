# Arduino App Releases — Detailed Design & Behavior

This document explains, in depth, how the **Release** feature works end to end: what
`release build` produces, the structure of the release manifest, the exact `install` and
`app start` (execute) workflows, and the full story of how **models**, **containers**, and
**secrets** are handled.

It is a design/reference companion to the user-facing section in
`docs/user-documentation.md`. Code references use `file:line` and point at the
`internal/orchestrator/` package unless noted.

---

## 1. Concept and terminology

An **Arduino App** on a board (Arduino UNO Q / VENTUNOQ, Linux/arm64) is a bundle of:
Python code, an optional MCU sketch, "bricks" (reusable components), and references to AI
models. It runs as a set of containers orchestrated by `docker compose`, plus a sketch
flashed to the microcontroller.

There are **two** ways to package an app:

| | `app export` / `app import` (pre-existing) | `release build` / `install` / `clone` (this feature) |
|---|---|---|
| Contents | **source only** (excludes `.cache`) | source **+ build artifacts** (prebuilt sketch, provisioned venv, frozen compose, bundled models) |
| On the destination | stays **editable**; first `app start` **recompiles** the sketch and **re-runs pip** | **immutable / locked**; `app start` flashes the prebuilt binary and reuses the venv — **no compile, no pip** |
| Network at install | n/a | **never** downloads models; only **containers** are pulled — by default at install (the "prepare" step), otherwise at `start` |
| Format | `.zip` | `.tar.gz` (preserves symlinks + file modes, required for the venv) |

> **Naming note:** the concept was originally "package" and was renamed to **"release"**
> (the word "package" confused Python developers). Unrelated apt/`debian-package`
> concepts in the codebase were intentionally **not** renamed.

A release is a **frozen** app: everything that would otherwise be rebuilt on first start is
already inside the archive, so it runs on another identical board reproducibly.

---

## 2. `release build` — step by step

Entry point: `BuildRelease` (`release.go:127`). Invoked by the CLI
`arduino-app-cli release build <app> <out.tar.gz>` and the HTTP endpoint
`GET /v1/apps/{id}/release`.

Flags: `--release-number` (default `YYYYMMDDhhmmss`), `--overwrite`, `--no-models`,
`--keep-secrets`.

The steps, in order:

1. **Release number** (`release.go:149`). Identifies this build. Defaults to a UTC timestamp
   `20060102150405`.

2. **Validate the app is pre-built** — `validatePrebuilt` (`release.go:359`). A release must
   never compile or provision; it only *copies* what already exists. It therefore requires:
   - `.cache/app-compose.yaml` to exist → otherwise *"app has not been started yet"*.
   - if the app has a sketch, a **non-empty** `.cache/sketch` build dir (`SketchBuildPath()`).
   - a **provisioned Python venv**, detected by walking `.cache` for a `pyvenv.cfg` file
     (`hasProvisionedVenv`, `release.go:389` — the venv dir name is owned by the python
     runner image, so it is discovered, not hard-coded).

   If any is missing the command fails with a *"Start the app once before packaging"* error.

3. **Handle secrets** (`release.go:161`). Unless `--keep-secrets` is set, `scrubAppSecrets`
   (`release.go:239`) walks every brick variable flagged `secret: true` in the brick
   definition that currently has a **non-empty** value, replaces the value in `app.yaml` with
   a `${NAME}` placeholder, and returns both the list of `required_secrets` (name + owning
   brick) and a `name → plaintext-value` map. See [§9 Secrets](#9-secrets).

4. **Resolve models** — `resolveAppModels` (`release.go:277`). Walks the models referenced by
   the app's bricks and decides, per model, whether it is *preloaded* (rides in a container
   image → not bundled) or must be **bundled** from its on-disk artifacts. Produces the
   manifest `models` entries and the list of files to add to the archive. See
   [§7 Models](#7-models-the-full-story).

5. **Freeze the compose** — `buildFrozenCompose` (`release.go:423`). Runs
   `docker compose config` to flatten all includes and pin concrete image versions, tokenizes
   host-specific values to portable placeholders, and scrubs secret material. See
   [§6 Compose freezing](#6-compose-freezing--tokenization). Result is stored as
   `.cache/release-compose.yaml`.

6. **Build the manifest** (`release.go:199`) — see [§4 Manifest](#4-the-manifest-file). The
   image list is derived from the frozen compose via `extractComposeImages`
   (`release.go:547`).

7. **Write the archive** — `writeAppTarGz` (`release.go:582`). Walks the app folder into a
   gzip tarball rooted at a filesystem-safe folder name (`releaseRootFolderName`,
   `release.go:574`), **preserving symlinks (via `Lstat`/`Readlink`) and file modes**
   (required for the venv). The scrubbed `app.yaml`, the manifest, and the frozen compose are
   injected as synthetic entries (so the scrubbed versions win over what is on disk), and the
   bundled model artifacts are appended. Excluded from the archive:
   - `data/` (reserved, host-local — holds `secrets.env`),
   - the non-portable generated `.cache/app-compose.yaml` and `.cache/app-compose-overrides.yaml`.

---

## 3. Release archive layout

```
my-app.tar.gz
└── my-app/
    ├── arduino-app-release.yaml   # the manifest (ReleaseManifest)
    ├── app.yaml                   # descriptor; secret values scrubbed by default
    ├── README.md
    ├── python/  sketch/  bricks/  # user code
    ├── .cache/
    │   ├── release-compose.yaml   # frozen, version-pinned, RELOCATABLE compose (placeholders)
    │   ├── sketch/                # prebuilt MCU binary (flashed as-is)
    │   └── <venv>/                # provisioned Python venv (pip is NOT re-run)
    └── models/                    # bundled AI model artifacts (only if any are bundled)
```

`data/`, `.cache/app-compose.yaml`, and `.cache/app-compose-overrides.yaml` are **not** in
the archive.

---

## 4. The manifest file

File name: `arduino-app-release.yaml` (constant `ReleaseManifestFileName`, `release.go:46`),
placed at the root of the release next to `app.yaml`. Struct: `ReleaseManifest`
(`release.go:61`). It records **provenance and compatibility** so `install` can validate the
release and restore its models/secrets.

```yaml
format_version: "1"                      # ReleaseFormatVersion; bump on breaking layout changes
app_name: "My App"
release: "20260702120000"                # the release number from create
created_at: 2026-07-02T12:00:00Z         # UTC
source_cli_version: "0.11.0rc6"          # CLI that built the release
runner_version: "0.11.0rc6"              # asset/runner version on the source board
python_image: "ghcr.io/arduino/.../python-runner:..."   # pinned python base image
board_name: "ventunoq"                   # <-- compatibility key (see §5, §8)
fqbn: "arduino:zephyr:..."               # source board FQBN
arch: "arm64"                            # GOARCH of the source
images:                                  # every container image referenced by the frozen compose
  - "ghcr.io/arduino/app-bricks/...:0.11.0rc6"
  - "artifacts.codelinaro.org/.../genai-llm-vlm-service:1.2.0"
models:                                  # one entry per model the app requires (see §7)
  - id: "genie:qwen3_4b_instruct_2507"
    name: "Qwen 3-4B Instruct"
    runner: "..."
    preloaded: false                     # false => its files are bundled under models/
    bundled: true
    paths:                               # bundled artifact paths, relative to the custom-models dir
      - "genai"
required_secrets:                        # secret variables whose VALUES are NOT in the release (see §9)
  - name: "OPENAI_API_KEY"
    brick: "arduino:llm"
```

Field-by-field responsibility:

| Field | Meaning | Used by install to… |
|---|---|---|
| `format_version` | release layout version | reject unknown/incompatible layouts |
| `app_name` | display name | name the installed app |
| `release` | release number | stamp `frozen_release.number` in `app.yaml` |
| `created_at`, `source_cli_version`, `runner_version`, `python_image` | provenance / diagnostics | (informational) |
| `board_name`, `fqbn`, `arch` | source hardware identity | **board compatibility check** (`board_name`; see §5/§8) |
| `images` | pinned container images | (informational; the compose is authoritative) |
| `models` | which models the app needs & how each is provided | **restore bundled model files** |
| `required_secrets` | secret variable names (value NOT in release) | write the `data/secrets.env` template + drive run-time resolution |

---

## 5. `install` workflow

Entry point: `InstallRelease` (`release.go:751`). CLI:
`arduino-app-cli release install <file.tar.gz>` (accepts `-` for stdin) with `--force`.
HTTP: `POST /v1/apps/release/install` (multipart) with `?force=`.

Steps:

1. **Read metadata** — `readReleaseMetadata` (`release.go:1132`) scans the archive for its
   root prefix and parses the manifest; it also validates that the archive really is a
   release (rejects non-releases).

2. **Board compatibility check** (`release.go:767`):
   ```go
   if !force && manifest.BoardName != "" && plat.BoardName != "" && manifest.BoardName != plat.BoardName {
       return ..., fmt.Errorf("%w: release was built for board %q but this board is %q (use --force to override)",
           ErrIncompatibleRelease, manifest.BoardName, plat.BoardName)
   }
   ```
   A mismatch fails fast with a message naming both boards and pointing at `--force`. See
   [§8](#8-board-compatibility--force) for why only `board_name` is compared and how the
   error is surfaced (CLI fatal / HTTP **409 Conflict**).

3. **Pick the destination name.** Uses the archive root folder name; on a name collision,
   appends a `-YYYYMMDD-HHMMSS` suffix. The folder name is validated.

4. **Extract to a temp dir** — `extractTarGz` (`release.go`, see [§10 Security](#10-security-hardening)).
   Extraction is hardened against tar-slip, malicious symlinks, and zip-bomb-style oversized
   files.

5. **Restore bundled models** — `restoreBundledModels` (`release.go:845`). Copies the
   archived `models/…` tree into the destination's **custom-models dir** (outside the app
   folder), preserving layout so the compose placeholder resolves. **Copy only — never
   downloads.** Existing artifacts are kept unless `--force`. The staging `models/` folder is
   then removed so it is not moved into the apps directory.

6. **Localize the compose** — `localizeInstalledRelease` (`release.go:921`). Reads
   `.cache/release-compose.yaml`, resolves the **stable** host paths for *this* machine
   (`${APP_HOME}` → the final app path, `${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}` → this
   board's custom-models dir), and writes the standard `.cache/app-compose.yaml`. `${HOST_IP}`
   is **left as a placeholder** — it is filled fresh at every `app start`. The frozen compose
   file is then removed (superseded). Finally it **stamps** `frozen_release.number` into
   `app.yaml` so the normal start path recognizes it as a release.

7. **Write the secrets template** — `writeSecretsTemplate` (`release.go:981`). If the manifest
   declares `required_secrets` and no `data/secrets.env` exists yet, writes a template listing
   the required keys with empty values.

8. **Validate & finalize.** `app.Load` validates the extracted app; then the temp dir is
   **atomically renamed** to the final path (so a failed install never leaves a half-written
   app). An app ID is derived and returned along with the manifest.

9. **Prepare (default)** — unless `--no-prepare` (CLI) / `?prepare=false` (HTTP) is set,
   install then runs the [prepare step](#10-containers):
   `PrepareRelease` (`release_prepare.go`) pre-pulls the container images the app needs so the
   first `app start` finds them locally. The install is already committed on disk at this
   point, so a prepare failure is **non-fatal** — it is surfaced as a warning and the user can
   re-run `release prepare` later (or a plain `app start` will pull whatever is still missing).

After install the app is a **regular app** in `app list` — there is no separate "release
start". It is launched, stopped, and inspected with the ordinary `app start/stop/logs`
commands; the release-specific behavior is triggered by the `frozen_release` marker in
`app.yaml`.

---

## 6. Compose freezing & tokenization

This is the mechanism that makes the compose **relocatable** across boards while keeping the
volatile network address dynamic. Three host-specific values are handled:

| Placeholder | Meaning | Resolved when |
|---|---|---|
| `${APP_HOME}` | the app's folder on disk | at **install** (stable per machine) |
| `${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}` | the custom-models dir | at **install** (stable per machine) |
| `${HOST_IP}` | the board's LAN IP | at **every `app start`** (volatile) |

### At create — `buildFrozenCompose` (`release.go:423`)

1. Run `docker compose -f app-compose.yaml [-f overrides] config`, but with the three
   host-specific env vars set to **unique sentinel tokens** (`release.go:413`):
   ```go
   frozenEnvs["HOST_IP"]                              = sentinelHostIP    // "__ARDUINO_RELEASE_PLACEHOLDER_HOST_IP__"
   frozenEnvs["APP_HOME"]                             = sentinelAppHome   // "/__ARDUINO_..._APP_HOME__"
   frozenEnvs["ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR"] = sentinelModelsDir // "/__ARDUINO_..._CUSTOM_MODEL_DIR__"
   ```
   Compose interpolation therefore expands those vars to the sentinels, not to real host
   values.

2. `tokenizeHostSpecificValues` (`release.go:474`) reverse-maps each sentinel to its portable
   `${...}` placeholder.

   **Why sentinels?** A naïve `strings.ReplaceAll` of the *real* host IP `192.168.1.5` would
   also corrupt `192.168.1.50` (substring over-match). Routing `${HOST_IP}` *references*
   through unique tokens eliminates that. As a **safety net** the function *also* replaces any
   literally baked `customModelsDir` / `appHome` strings (long, unique absolute paths that
   cannot over-match a substring), guarded against empty/`/` values.

   **Baked host IP.** The provisioner writes the app's `main` service `environment:` with the
   *resolved* env map (`provision.go:392`), so the generated `.cache/app-compose.yaml` carries
   `HOST_IP: <literal-ip>` — **not** a `${HOST_IP}` reference. The sentinel indirection never
   sees that literal, so `tokenizeHostSpecificValues` also replaces the **known** host IP
   (passed in from `envs["HOST_IP"]`) using a **word-boundary-anchored regex**
   (`\b<ip>\b` via `ReplaceAllLiteralString`), which tokenizes `192.168.1.5` while leaving
   `192.168.1.50` intact — the exact over-match the sentinel was introduced to avoid. Without
   this, the real board IP leaks into `release-compose.yaml` inside the tar (fixed).

   > This safety net is load-bearing for local LLMs: the genie/llamacpp runner service
   > composes mount the models dir as a **hardcoded absolute path**
   > (`/var/lib/arduino-app-cli/models/genai`) rather than via the env var, so only the
   > literal-path replace turns them into `${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}/genai`.
   > This works board-to-board because the custom-models dir is the same fixed path on every
   > board (and is a prefix of the mount). See [§7](#7-models-the-full-story).

3. Scrub secret material from the flattened compose (see [§9](#9-secrets)).

### At install — `localizeInstalledRelease` (`release.go:921`)

```go
localized = strings.ReplaceAll(localized, "${APP_HOME}",                          finalAppPath.String())
localized = strings.ReplaceAll(localized, "${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}", cfg.CustomModelsDir().String())
// ${HOST_IP} intentionally left for run time
```

### At run — shared with normal apps

`getAppEnvironmentVariables` (`orchestrator.go:253`) sets `APP_HOME` and, crucially,
`HOST_IP` **fresh** via `helpers.GetHostIP()` (`orchestrator.go:293`). `StartApp` then runs
`docker compose up` with those env vars (`orchestrator.go:230`), so Docker Compose
interpolates the `${HOST_IP}` still present in `app-compose.yaml`. The IP thus always
reflects the *current* board and can change between reboots without re-installing.

---

## 7. Models — the full story

Governing rule: **`install` never downloads a model; the only thing ever pulled from the
network is containers, and only at `start`.** How each model is provided depends on its
deployment type, decided in `resolveAppModels` (`release.go:277`) using
`ModelsIndex.GetModelLocalArtifacts` (`modelsindex/models_index.go:289`).

| Model type | Detection | Release behavior |
|---|---|---|
| **Preloaded** | `deployment.pre-loaded: true` (e.g. the EI vision/audio models) | Lives **inside** the version-pinned container image → **not bundled**. Manifest: `preloaded: true`. |
| **Custom / Edge Impulse** (self-contained folder) | `model.ModelFolderPath` exists | The whole folder is **bundled** under `models/…`. |
| **Handler-downloaded** (AI Hub / Edge Impulse / Hugging Face) | `deployment.handler` set, not preloaded | Located via the handler's bind volume `${CUSTOM_MODEL_DIR}/${models_repository}` and **bundled** from disk. |

If a required, non-preloaded model cannot be located, `create` **fails** (the release would
be incomplete) — unless `--no-models`, which records the model in the manifest but leaves
installing it to the destination.

At install, `restoreBundledModels` (`release.go:845`) copies the bundled files back into the
destination's custom-models dir (never downloads); the frozen compose references that dir via
`${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}`, so it resolves wherever the destination keeps its
models.

### Local LLMs (Qwen3, Gemma via `genie` / `llamacpp`)

On-device LLMs are **the third row above** — they are **not preloaded** and are
handler-backed, so **their multi-GB weights are bundled into the `.tar.gz`**. Three pieces
cooperate:

- an `arduino:llm` / `arduino:vlm` brick in the app,
- a model entry like `genie:qwen3_4b_instruct_2507` (`ai-hub-handler`) or
  `llamacpp:gemma-4-E2B_q4_0-it` (`hf-handler`), with no `pre-loaded`,
- a **runner service** merged into the compose: `arduino:genie` (port 9001) or
  `arduino:llamacpp` (port 9999). The runner container loads the weights from a mounted
  directory and serves inference over localhost; the brick talks to it.

Weights live on disk under the runner's repository folder:
`<CustomModelsDir>/genai/…` (genie) or `<CustomModelsDir>/llamacpp/…` (llamacpp), which on a
board is `/var/lib/arduino-app-cli/models/…`.

Lifecycle for a local LLM:
- **create**: `GetModelLocalArtifacts` resolves the weights under the repo folder and bundles
  them into `models/genai|llamacpp/…`; `create` fails fast if they are not already on the
  source board. The runner's hardcoded mount path is tokenized by the safety-net replace (see
  [§6](#6-compose-freezing--tokenization)).
- **install**: weights are copied into the destination's repo folder (no download);
  `${ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR}` is localized.
- **start**: only the **runner container image** is pulled (`--pull missing`); the weights are
  already on disk. The runner mounts them and serves the model.

**Caveats specific to local LLMs (real, worth knowing):**

1. **Huge archives.** Qwen3-4B ≈ 3 GB, Qwen2.5-VL-7B ≈ 6.5 GB, Gemma-4-E4B ≈ 5.3 GB — all
   carried inside the tarball.
2. **Over-bundling.** The "refine to a single model file" step in `GetModelLocalArtifacts`
   keys on the `model_name` deployment variable
   (`modelsindex/models_index.go:320`), but genie's weights actually sit in a directory named
   by `model_directory` (a different string), and llamacpp declares **no `model_name` at
   all**. So the refine misses and the code bundles the **entire repository folder** — i.e.
   *every* genie/llamacpp model present on the source board, not just the one the app uses.
   Not incorrect (the needed model is always included), but bloats the release.
3. **Relocation relies on the custom-models dir being a prefix** of the runner's hardcoded
   mount. True board-to-board (both `/var/lib/arduino-app-cli/models`), so it works for the
   intended use case; fragile if a host overrides `ARDUINO_APP_BRICKS__CUSTOM_MODEL_DIR`.
4. **Board homogeneity.** The runner mounts Qualcomm firmware/NPU devices (`/dev/dma_heap/*`,
   `/usr/share/qcom`, `/sys/firmware/...`) left literal (correct — identical across boards of
   the same type). genie/VLM releases are **ventunoq-only**; llamacpp supports unoq+ventunoq.

> **Cloud LLMs** (e.g. the `chatbot-cloud-llm` example) are the opposite case: no local model
> artifact at all — the app calls a remote API. Nothing is bundled; the only sensitive thing
> is the API key, handled by the [secrets](#9-secrets) path.

---

## 8. Board compatibility & `--force`

Because a release carries a **prebuilt sketch binary** (FQBN/arch-specific) and
**arch-specific container images**, it is only valid on a matching board. `create` records
the source board identity (`board_name`, `fqbn`, `arch`) in the manifest; `InstallRelease`
enforces it (`release.go:767`):

- If the release's `board_name` differs from the current board's, install **fails** with
  *"release was built for board X but this board is Y (use --force to override)"*
  (`ErrIncompatibleRelease`).
- `--force` (CLI flag) / `?force=true` (HTTP) skips the check and proceeds. `--force` also
  makes `restoreBundledModels` overwrite existing model artifacts.
- The check is **skipped** if either side's board name is empty (unknown platform → don't
  block).

Error surfacing:
- **CLI** (`cmd/arduino-app-cli/apprelease/install.go`): `ErrIncompatibleRelease` →
  `feedback.Fatal(..., ErrBadArgument)` (clean message, no stack).
- **HTTP** (`internal/api/handlers/release_install.go`): `ErrIncompatibleRelease` → **HTTP 409
  Conflict** with the same message.

**Design note:** only `board_name` is compared, though `fqbn` and `arch` are also recorded.
For these boards `board_name` (`unoq`/`ventunoq`) maps 1:1 to arch and FQBN, so a name match
implies an arch match. Tightening to also compare `arch` would be a one-line addition in the
same `!force` block if ever needed.

---

## 9. Secrets

**Invariant: a release, unlike a model, must never carry credentials.** Brick variables that
the brick definition flags `secret: true` (API keys, passwords, …) are handled specially so
the shareable archive contains no secret material.

### At create (default)

1. `scrubAppSecrets` (`release.go:239`) replaces each non-empty secret variable's value in
   `app.yaml` with a `${NAME}` placeholder and records it under `required_secrets` (name +
   owning brick). Empty secret variables are left as-is (nothing to hide). Returns the
   `name → value` map.
2. The frozen compose is scrubbed in **two passes**:
   - `scrubComposeSecretKeys` (`release.go:492`) rewrites the env entries **by key** (both the
     map form `NAME: value` and the list form `- NAME=value`) to `NAME: ${NAME}`.
   - `scrubComposeSecretValues` (`release.go:518`) then strips any **literal secret value**
     still present (e.g. a password embedded inside a `postgres://user:pass@host` connection
     string), longest-value-first so a short secret can't corrupt a longer one. As
     defense-in-depth it **returns an error and aborts the build** if any secret value still
     appears afterwards — the release is never written with a leaked secret.

### At install

`writeSecretsTemplate` (`release.go:981`) drops a `data/secrets.env` template (only if the
release declares secrets and the file doesn't already exist), listing the required keys with
empty values and a per-key comment naming the owning brick.

### At run (`app start`) — shared by releases and scrubbed exports

`applyReleaseSecrets` (`release.go:1051`):
- Determines the **authoritative set of required secret names** via `declaredSecretNames`
  (`release.go:1007`): the union of the release manifest's `required_secrets` and the keys in
  `data/secrets.env`. This is deliberately *not* "any `${NAME}`-shaped env value", which would
  misclassify a legitimate non-secret `${FOO}` as a required secret. A placeholder-scan
  fallback is used **only** when neither a manifest nor a secrets template exists (e.g. a
  hand-crafted app).
- Loads `data/secrets.env` (`parseEnvFile`, `release.go:1095`) and injects values into the
  env, but **only for declared secrets this app actually references and hasn't resolved yet**
  (value still a `${NAME}` placeholder).
- Returns the list of **still-missing** secrets. `StartApp` (`orchestrator.go:176`) then
  **fails fast** — the app does not start with blank credentials — with
  *"missing required secret(s) …: set them in data/secrets.env"*.

This is a **no-op** for apps with no secret placeholders.

### Escape hatches & related

- `release build --keep-secrets` embeds the values in the release instead (no placeholders,
  no `required_secrets`). The resulting archive is self-contained but **sensitive**.
- `app export --scrub-secrets` applies the identical placeholder scrubbing to a plain source
  export and drops a `data/secrets.env` template into the zip; the re-imported app resolves
  them at `app start` exactly as a release does. (`ExportAppZip` gained a `scrubSecrets`
  param.)
- **Caveat:** secrets still sit in plaintext in `data/secrets.env` on the destination — that
  is unavoidable without an external secret manager. The guarantee is only that the **release
  archive** carries no secrets.

---

## 10. Containers

- **Version pinning.** `buildFrozenCompose` runs `docker compose config`, which resolves and
  flattens all compose includes and pins concrete image references. The pinned images are
  also listed in the manifest's `images` (via `extractComposeImages`, `release.go:547`) for
  visibility, though the frozen compose is the authoritative source.
- **Pre-pulled at install by the "prepare" step (default).** After extraction, `install` runs
  `PrepareRelease` (`release_prepare.go`) unless `--no-prepare` / `?prepare=false` is given.
  Prepare reads the installed app's localized compose (`AppComposeFilePath()`), lists the image
  references (`extractComposeImages`), skips those already present locally
  (`listImagesAlreadyPulled`) and pulls the rest (`pullImage`, with retry) — exactly the
  `--pull missing` set `start` would need, but **without launching the app**. It is also
  available standalone as `release prepare <installed-app>` (CLI) / `POST /v1/apps/{id}/prepare`
  (HTTP) so a user who skipped it at install can run it later. It never downloads AI models —
  only containers — since model weights are already on disk from install.
- **Also pulled at start.** `StartApp` runs
  `docker compose up -d --remove-orphans --pull missing` (`orchestrator.go:210`). Only images
  **not already present** are pulled — so after a successful prepare this pulls nothing, and if
  prepare was skipped or partially failed, start still pulls whatever is missing. Model weights
  are never pulled here — they are already on disk from install.
- **Runner containers.** AI runners (python-runner, genie `genai-llm-vlm-service`,
  `llamacpp-npu-runner`, the models-downloader handler image, etc.) are ordinary services in
  the frozen compose and are pulled the same way.

---

## 11. Execute — `app start` on a frozen release

There is **no `release start`**. An installed release runs through the normal `StartApp`
(`orchestrator.go:93`), which branches on `appToStart.IsFrozenRelease()` (true when
`Descriptor.FrozenRelease != nil`, set by `localizeInstalledRelease`).

`frozenRelease := appToStart.IsFrozenRelease()` (`orchestrator.go:143`) then changes exactly
three things versus a normal app:

1. **Sketch** (`orchestrator.go:145`): if the app has a sketch, `uploadPrebuiltSketch`
   (`release.go:1348`) flashes the already-compiled binary from `.cache/sketch` **as-is** (an
   upload-only step via the arduino-cli RPC server) — it does **not** run
   `compileUploadSketch`.
2. **Provisioning** (`orchestrator.go:190`): the frozen branch **skips** `provisioner.App`
   entirely — the venv is already on disk and the compose is already localized and
   version-pinned. A normal app would provision (pip) here.
3. Everything else is **identical** to a normal app:
   - `getAppEnvironmentVariables` builds the env (including a fresh `HOST_IP`);
   - `applyReleaseSecrets` resolves `${NAME}` secrets and fails fast on a missing one;
   - `docker compose up -d --remove-orphans --pull missing` brings the app up.

Because install wrote the standard `.cache/app-compose.yaml`, `app stop`, `app logs`, and
status all behave exactly as for any app.

> **Linchpin assumption (unverified on hardware):** skipping provisioning relies on the
> external python-runner image's entrypoint **not** reinstalling deps when the venv already
> exists in `.cache`. If it always reinstalls, bundling the venv is pointless. Confirm on a
> board / with the image maintainers.

---

## 12. `clone` — turning a release back into an editable app

`release clone <installed-app> <new-name>` → `CloneApp` with `StripFrozenRelease = true`
(`orchestrator.go:762`). The clone:

- copies the app tree **excluding `.cache` and `data`** (so it has no prebuilt artifacts),
- also excludes the release manifest (`ReleaseManifestFileName`) so it is no longer
  recognized as a release,
- clears `frozen_release` from the cloned `app.yaml` (`descriptor.FrozenRelease = nil`,
  `orchestrator.go:849`).

The result is a normal, editable app: its next `app start` **recompiles** the sketch and
**re-provisions** the venv, exactly like a freshly imported source app. (HTTP:
`POST /v1/apps/{id}/clone` with `strip_release` in the JSON body.)

---

## 13. Security hardening (extraction)

`extractTarGz` (`release.go`) defends against hostile archives:

- **Tar-slip:** every entry's resolved target must stay within the destination
  (`strings.HasPrefix(target, destClean)`), else *"illegal file path in release"*.
- **Malicious symlinks:** a **two-pass** approach. First `collectArchiveNames` gathers all
  entry names; then, for each symlink:
  - **relative** targets must resolve **inside** the destination;
  - **absolute** targets are rejected **only if** another archive entry is nested under the
    symlink's path (the real tar-slip-via-symlink vector). A bare absolute *leaf* symlink is
    allowed, because the **Python venv legitimately needs** one
    (`bin/python -> /usr/local/bin/python` baked into the base image). This was a real bug
    found during on-board testing and fixed here.
- **Oversized files:** each regular file is copied through an
  `io.LimitReader(tr, maxReleaseFileSize+1)` and rejected if it exceeds `maxReleaseFileSize`
  (zip-bomb / runaway-size guard).

Plus the create-side guard: `scrubComposeSecretValues` aborts the build if any secret value
survives scrubbing (see [§9](#9-secrets)).

---

## 14. CLI & HTTP surface

**CLI** (`cmd/arduino-app-cli/apprelease/`):
- `release build <app> <out.tar.gz>` — `--release-number`, `--overwrite`, `--no-models`,
  `--keep-secrets`.
- `release install <file.tar.gz>` — `-` reads from stdin; `--force`; `--no-prepare` (skip the
  default image pre-pull).
- `release prepare <installed-app>` — pre-pull the app's container images (no start, no model
  download); the same step `install` runs by default.
- `release clone <installed-app> <new-name>`.
- No `release start` (use `app start`).

**HTTP daemon API** (`internal/api/api.go`):
- `GET  /v1/apps/{id}/release` → builds + streams the `.tar.gz`
  (`?models=`, `?release_number=`, `?keep_secrets=`); uses `render.EncodeTarGzResponse`.
- `POST /v1/apps/release/install` → multipart upload install (`?force=`, `?prepare=`
  [default true; blocks until images are pulled, pull failure is non-fatal]).
- `POST /v1/apps/{id}/prepare` → pre-pull the installed app's container images (`204` on success).
- `GET  /v1/apps/{id}/export?scrub_secrets=` → export with placeholder scrubbing.
- `POST /v1/apps/{id}/clone` with `strip_release` in the body.

OpenAPI: `internal/api/docs/openapi.yaml` (`buildAppRelease` / `installAppRelease` /
`prepareAppRelease`). The e2e client is generated — after any OpenAPI change run
`go generate ./internal/e2e/` (or `task generate`).

---

## 15. Known caveats & unverified items

Compiled + unit-tested; **not yet verified on hardware** beyond a `create`/`install` smoke
test. Open items:

1. **venv-skip linchpin** — depends on the python-runner image not reinstalling deps when the
   venv exists (see [§11](#11-execute--app-start-on-a-frozen-release)).
2. **Local-LLM over-bundling** — a Qwen3/Gemma release carries the whole `genai/`/`llamacpp/`
   repo folder, not just the one model (see [§7](#7-models-the-full-story)). Fix: have
   `GetModelLocalArtifacts` also match on `model_directory`.
3. **Compose secret scrub** is regex-on-key + literal-value strip against `docker compose
   config` output — grep a generated `release-compose.yaml` for known secret values to
   confirm nothing leaks.
4. **Handler-model artifact resolution** is derived from `models-handlers.yaml` /
   `models-list.yaml` conventions — verify it resolves to the right files for a real
   handler-downloaded model.
5. **Secrets remain plaintext** in `data/secrets.env` on the destination (expected; the
   guarantee is only about the archive).

---

### Key files

- `internal/orchestrator/release.go` (+ `release_test.go`) — build/install/clone core,
  tar/scrub/secrets/localize/models helpers, `ReleaseManifest`/`ReleaseModel`/`ReleaseSecret`.
- `internal/orchestrator/release_prepare.go` (+ `release_prepare_test.go`) — `PrepareRelease`
  (pre-pull the app's container images) and the `imagesToPull` helper.
- `internal/orchestrator/orchestrator.go` — `StartApp` frozen branch; `CloneApp` /
  `StripFrozenRelease`; `getAppEnvironmentVariables` (`HOST_IP`/`APP_HOME`).
- `internal/orchestrator/modelsindex/models_index.go` — `GetModelLocalArtifacts`.
- `internal/orchestrator/archive.go` — `ExportAppZip(..., scrubSecrets)`.
- `internal/orchestrator/app/parser.go` / `app.go` — `FrozenReleaseInfo`, `IsFrozenRelease()`,
  release path helpers.
- `cmd/arduino-app-cli/apprelease/` — CLI commands.
- `internal/api/handlers/` — `app_release_build.go`, `release_install.go`,
  `app_release_prepare.go`, `app_clone.go`, `app_export.go`; routes in `internal/api/api.go`;
  spec in `internal/api/docs/openapi.yaml`.
- `docs/user-documentation.md` — user-facing "Arduino App Releases" section.
