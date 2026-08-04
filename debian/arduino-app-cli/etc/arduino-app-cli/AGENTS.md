# Agent Context File for Arduino Uno Q and Ventuno Q boards
You are an AI coding agent working with an Arduino **UNO Q** or **VENTUNO Q** board.
Your job is to help the user **build, run, and debug Arduino Apps** and operate the
board.

If you are **not running on the board itself** (e.g. on the user's PC), every
`arduino-app-cli` command and filesystem path in this document refers to the
board — run them there via App Lab, `adb shell`, or SSH.

## Prime directives

1. **Query the board, don't memorize.** Catalogs, versions, paths, and Brick APIs
   change between releases. The live commands and on-board files below are always
   authoritative over anything written here.
2. **Identify the board before any board-dependent choice** (see §1).
3. **Learn from the bundled examples.** `/var/lib/arduino-app-cli/examples/` holds
   dozens of complete, working apps. Find the one closest to the user's goal and read its
   `app.yaml`, `python/main.py`, and `sketch/sketch.ino` before writing new code.
4. **`--help` is the truth for the CLI.** Any `arduino-app-cli <cmd> --help` prints
   exact flags for the installed version.

---

## 1. The board

You are almost certainly on a **UNO Q** — treat it as the default. **VENTUNO Q** is
the larger sibling; assume it only once identification says so.

Both run the same stack: a Linux MPU side + an Arduino/Zephyr MCU side (STM32
Cortex-M33), `arduino-app-cli`, Bricks, the Router Bridge, and an 8×13 LED matrix.
The **MPU (Linux side)** runs the Python app, AI models, web servers, and network
I/O; the **MCU** runs the sketch (`*.ino`) for real-time GPIO, sensors, actuators,
and the LED matrix. They talk over the **Router Bridge** (§5).

**Identify the current board — run this first, without asking, but explain to the
user what you are doing:**

```bash
cat /sys/firmware/devicetree/base/compatible 2>/dev/null | tr '\0' '\n'
```

Match the **first** (`arduino,*`) token: `arduino,imola` → **UNO Q**,
`arduino,monza` → **VENTUNO Q**.

Software-relevant differences (clocks, wireless, storage, form factor are in the
on-board datasheet):

| | **UNO Q** (default) | **VENTUNO Q** |
|---|---|---|
| RAM | ~2–4 GB | ~16 GB |
| NPU | — | ✓ |
| RGB LEDs (active-low) | 4 (2 MCU-controllable) | 4 |

---

## 2. `arduino-app-cli` — the control surface

The board ships `arduino-app-cli` (runs as the `arduino` user). It manages the
full lifecycle of every App. Global flags: `--format text|json`,
`--log-level debug|info|warn|error`.

```bash
arduino-app-cli --help            # top-level command groups
arduino-app-cli version           # CLI + daemon version
arduino-app-cli config get        # data dir, apps dir, examples dir
```

Command groups include `app`, `brick`, `model`, `monitor`, `properties`, `system`,
`config` — see `arduino-app-cli [<group>] --help` for the full list and exact flags.

**App lifecycle** (lifecycle commands take a **path**; cache/properties take an
**ID** like `user:my-app` or `examples:blink`):

```bash
arduino-app-cli app list                              # apps + STATUS + IDs
arduino-app-cli app start   ~/ArduinoApps/my-app      # stops whatever was running!
arduino-app-cli app stop    ~/ArduinoApps/my-app
arduino-app-cli app restart ~/ArduinoApps/my-app
arduino-app-cli app logs    ~/ArduinoApps/my-app --follow   # Python stdout/Logger
arduino-app-cli app logs    ~/ArduinoApps/my-app --tail 200
arduino-app-cli monitor                               # MCU serial (Serial.print)
```

> **Only one App runs at a time.** `app start` implicitly stops the current one —
> confirm with the user first.

**Read-only / safe to run anytime:** `app list`, `app logs`, `brick list`,
`brick details`, `config get`, `version`, `properties get`.

**State-changing / confirm with the user first:** `app start|stop|restart|new|
destroy|clean-cache|import`, `model delete`, `system *`,
`properties set default …` (sets the app that auto-runs on boot).

If a sketch build acts stale after edits:
`arduino-app-cli app clean-cache user:<name> --force`, then restart.

---

## 3. Bricks — catalog & docs

Bricks are pre-built building blocks (Python module + optional Docker image +
optional AI model). You **declare** a Brick in `app.yaml` and **import** it in
`python/main.py`.

**Fetch the live catalog and per-Brick docs — never guess IDs:**

```bash
arduino-app-cli brick list                       # exact IDs (arduino:<id> form)
arduino-app-cli brick details arduino:<brick_id> # full README: API, vars, hardware
```

**Read a Brick's bundled README** (Python class, methods, examples). The first
dir under `assets/` is the installed Bricks-package version — glob it with `*`:

```bash
ls  /var/lib/arduino-app-cli/assets/*/docs/arduino/                 # all bricks
cat /var/lib/arduino-app-cli/assets/*/docs/arduino/<brick>/README.md
```

> Path note: the README lives **directly** in `…/docs/arduino/<brick>/README.md`
> (no `/doc/` subfolder). The folder name is the bare brick name with underscores
> (e.g. `video_object_detection`) — no `arduino:` prefix.

**Golden rules for Bricks:**

- Every `from arduino.app_bricks.<x> import …` **must** have a matching
  `arduino:<x>` entry under `bricks:` in `app.yaml`. Edit both together, or the
  app imports fine in the editor but fails at launch.
- Brick IDs are **verbatim from `brick list`** — don't pluralize, abbreviate, or
  invent them. Note underscores: `arduino:video_image_classification`,
  `arduino:video_object_detection`.
- Some Bricks are **board-specific** — `brick list` on the current board is the
  authoritative list of what's available there. Trust it over docs folders or memory.
- **Secrets** (API keys, bot tokens, device IDs) go in **Brick Configuration**
  (env vars set in App Lab), **never** in `app.yaml` or source.

---

## 4. Anatomy of an Arduino App

Each folder under `~/ArduinoApps/` is a self-contained App. Don't mix files
between apps.

```
my-app/
├── app.yaml            # manifest: name, icon, description, bricks
├── README.md           # optional human docs
├── python/
│   ├── main.py         # REQUIRED — entry point; ends with App.run()
│   └── requirements.txt# optional extra PyPI deps
├── sketch/             # OPTIONAL — only when the MCU is used
│   ├── sketch.ino      # entry point; Bridge.begin() in setup()
│   └── sketch.yaml     # build profile + extra Arduino libraries
├── assets/             # OPTIONAL — static files served by web_ui at :7000
└── .cache/             # build output — never edit; gitignore it
```

The folder names `python/`, `sketch/`, `assets/` are fixed — don't rename them.

**Create a new App** (the preferred way to scaffold):

```bash
arduino-app-cli app new my-app -d "What it does" -i "🚀" \
    -b arduino:web_ui -b arduino:video_object_detection
arduino-app-cli app new my-app --no-sketch                  # Python-only app
arduino-app-cli app new my-app --from-app /var/lib/arduino-app-cli/examples/blink
```

**`app.yaml`** — `name`, `icon`, `description` required; `bricks` optional:

```yaml
name: Face Detector
icon: ☺️
description: Detect faces in a live USB-camera feed.
bricks:
  - arduino:video_object_detection:      # per-brick config: model / variables
      model: face-detection
  - arduino:web_ui
```

**`sketch/sketch.yaml`** — build profile. Extra Arduino libraries the sketch
needs (Modulino, ArduinoGraphics, sensor drivers…) are declared here with pinned
versions — there is no `lib install` step:

```yaml
profiles:
  default:
    platforms:
      - platform: arduino:zephyr
    libraries:
      - Arduino_Modulino (0.7.0)
default_profile: default
```

**`python/main.py`** — initialize Bricks/Bridge at module level, then end with
`App.run()`. Anything after `App.run()` is dead code.

```python
from arduino.app_utils import App, Bridge, Logger   # + optional: Leds, FrameDesigner
# instantiate Bricks, register Bridge providers / UI handlers ...
App.run()                       # or App.run(user_loop=loop_fn) for a repeating loop
```

**App rules for agents:**

1. Read `app.yaml` + `main.py` (+ `sketch.ino`) before editing an existing app.
2. Keep imports and `bricks:` in sync (§3).
3. `App.run(...)` stays last. Use `Logger` (or `print`) for Python logs — they
   surface in `app logs`.
4. Preserve existing `# SPDX-…` license headers when editing files.
5. Verify before reporting "done": `app start` then `app logs --follow`. If you
   can't test a UI in a browser, say so.

---

## 5. The Bridge — MPU ↔ MCU communication

The Python app (MPU) and the sketch (MCU) communicate over the **Router Bridge**
(RPC). Two verbs — `call` (synchronous, returns a value) and `notify`
(fire-and-forget) — usable in each direction:

| Pattern | Caller | Returns? |
|---------|--------|----------|
| Python → MCU, sync  | `Bridge.call("name", *args)`   | yes — the return value |
| Python → MCU, async | `Bridge.notify("name", *args)` | no — fire-and-forget |
| MCU → Python, sync  | `Bridge.call("name", *args)`   | yes — read via `.result(out)` |
| MCU → Python, async | `Bridge.notify("name", *args)` | no — fire-and-forget |

The receiver always registers the handler with `Bridge.provide("name", fn)` on the
opposite side. Use `notify` for high-rate streams (sensor data) so the sender never
blocks; use `call` when you need the return value.

**Name-matching contract.** The name passed to `provide` on one side is the lookup
key for `call`/`notify` from the other — **names must match exactly**. How a
mismatch surfaces depends on the verb: Python-side `Bridge.call` **raises** (per the
library docs: `ValueError` if the method doesn't exist on the MCU, `TimeoutError`
on timeout); sketch-side `Bridge.call(...).result(out)` reports failure **only
through its bool — always check it**; `notify` in either direction is
fire-and-forget, so a mismatched `notify` **fails silently**. When a Bridge
interaction "does nothing" with no error, check the names on both sides first.

**Return values differ per side:** Python's `Bridge.call` returns the value
directly; sketch-side `Bridge.call` returns a handle — read the reply with
`.result(out)`, which returns `true` on success:

```cpp
String state;
bool ok = Bridge.call("get_state").result(state);   // sketch-side call idiom
```

**Sketch side** — include the library, call `Bridge.begin()` in `setup()`, and
log with `Serial` (e.g. `Serial.println(...)`), viewable via `arduino-app-cli monitor`:

```cpp
#include <Arduino_RouterBridge.h>

void set_led(bool on) { digitalWrite(LED_BUILTIN, on ? LOW : HIGH); }  // active-low: LOW turns the LED on

void setup() {
  pinMode(LED_BUILTIN, OUTPUT);
  Bridge.begin();
  Bridge.provide("set_led", set_led);     // expose to Python
}
void loop() {
  // optional: Bridge.notify("sensor", value);  // stream up to Python
  // Serial.println("debug line");              // shows in `arduino-app-cli monitor`
}
```

**Python side:**

```python
from arduino.app_utils import App, Bridge

def loop():
    Bridge.call("set_led", True)                 # invoke the sketch handler

Bridge.provide("sensor", lambda v: print(v))     # receive notify/call from sketch
App.run(user_loop=loop)
```

Common patterns (study the matching example): Python loop drives the sketch
(`blink`), sketch streams sensor data up (`real-time-accelerometer`,
`home-climate-monitoring-and-storage`), Web UI + WebSocket (`blink-with-ui`),
event-driven AI Brick (`video-face-detection`).

---

## 6. LED matrix

An **8×13 monochrome blue** matrix (104 pixels, **3-bit grayscale** → 8 levels),
driven by the MCU via `Arduino_LED_Matrix.h`. It shows the boot logo for ~20–30 s
during startup — **don't drive it before boot completes**, or you may disturb the
MCU.

Two frame formats — don't mix them up:

- **Per-pixel** `uint8_t[104]` (one value per LED, `0..7`) → `matrix.draw(frame)`.
- **Bit-packed** `uint32_t[4]` (on/off) → `matrix.loadFrame(frame)`; a sequence is
  `uint32_t[][5]`, played with `matrix.loadSequence()` + `matrix.playSequence()`
  (**`playSequence()` blocks** until the sequence finishes).

Learn the rest from the examples — `keyword-spotting` (native sequence),
`mascot-jump-game` (frame-by-frame + Bridge state), `led-matrix-painter` (web
editor), `air-quality-monitoring` (status display) — and the on-board library
source (the API ground truth):
`~/.arduino15/packages/arduino/hardware/zephyr/*/libraries/Arduino_LED_Matrix/`.

---

## 7. Ground rules (do / don't)

- **Do** identify the board (§1) and query the live catalog/help before deciding.
- **Do** start from the closest example in `/var/lib/arduino-app-cli/examples/`.
- **Do** keep `app.yaml` `bricks:` in sync with Python imports.
- **Do** use `Bridge.call/notify/provide` for MPU↔MCU, and `Serial.println(...)`
  for sketch logging (view it with `arduino-app-cli monitor`).
- **Do** confirm before any state-changing command (§2) — especially `app start`
  (stops the running app) and anything under `system`.
- **Do** verify signatures before using `App`, `Logger`, `Bridge`, or `FrameDesigner`
  — the installed source is authoritative:
  `python3 -c "from arduino.app_utils import App; help(App)"`.
- **Don't** invent Brick IDs, rename the fixed folders, edit `.cache/`, or put
  secrets in `app.yaml`/source (use Brick Configuration env vars).
- **Don't** write code after `App.run()` — it never executes.
- **Don't** claim something works until you've started the app and checked the
  logs; if you can't verify (e.g. a browser UI), say so plainly.

---

## 8. Where to look next (all on the board)

| Need | Source |
|------|--------|
| Exact CLI flags | `arduino-app-cli <cmd> --help` |
| Live Brick catalog | `arduino-app-cli brick list` |
| One Brick's API | `arduino-app-cli brick details arduino:<id>` or `…/assets/*/docs/arduino/<brick>/README.md` |
| Full `app.yaml` / App format spec | `docs/app-specification.md` in the `arduino-app-cli` repository |
| Working app patterns | `/var/lib/arduino-app-cli/examples/` |
| Paths (apps/data/examples) | `arduino-app-cli config get` |
| Python logs / MCU serial | `arduino-app-cli app logs … --follow` / `arduino-app-cli monitor` |
| MCU core + bundled library sources | `~/.arduino15/packages/arduino/hardware/zephyr/*/libraries/` |

Online:
- UNO Q docs: <https://docs.arduino.cc/hardware/uno-q/>
- General Arduino docs: <https://docs.arduino.cc/llms.txt>
- Brick examples: <https://github.com/arduino/app-bricks-examples>
- Router Bridge library: <https://github.com/arduino-libraries/Arduino_RouterBridge>
- Modulino (Qwiic/I²C peripherals): <https://github.com/arduino-libraries/Arduino_Modulino>
- Zephyr-based MCU core: <https://github.com/arduino/ArduinoCore-zephyr>
