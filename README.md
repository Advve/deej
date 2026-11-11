# deej

deej is an **open-source hardware volume mixer** for Windows and Linux PCs. Turn real knobs or sliders to **control the volume of individual apps** (your music player, the game you're playing, your voice chat) without leaving what you're doing.

It has two parts: a small desktop app written in Go, and an Arduino board with a few potentiometers that's cheap and simple to build.

> **This is a maintained fork of [omriharel/deej](https://github.com/omriharel/deej)**, created by [Omri Harel](https://github.com/omriharel). The original hasn't been updated since 2021 (last release v0.9.10, August 2020). This fork keeps it working with current hardware and drivers and adds support for boards with a screen. See [What's new in this fork](#whats-new-in-this-fork).

![deej](assets/build-3d-annotated.png)

> _No 3D printer? [No problem](./assets/build-shoebox.jpg)._ deej works just as well built into cardboard, a shoebox or on a breadboard.

## Table of contents

- [What's new in this fork](#whats-new-in-this-fork)
- [Features](#features)
- [How it works](#how-it-works)
- [Configuration](#configuration)
- [Boards with a screen](#boards-with-a-screen)
- [Build your own](#build-your-own)
- [How to run](#how-to-run)
- [Building from source](#building-from-source)
- [Troubleshooting](#troubleshooting)
- [Credits](#credits)
- [License](#license)

## What's new in this fork

- **Builds and runs on current Linux distributions.** The original's tray icon needed WebKit2GTK 4.0 and libappindicator3, which Ubuntu 24.04 and other current distributions no longer ship. This fork uses a tray library that talks to the desktop over D-Bus, so the Linux build is a single static binary that needs no extra packages.
- **Works with current CH340 drivers on Windows.** Many Arduino Nano clones use a CH340 USB chip. Since driver version 3.8 (2023), Windows returns empty reads from it, and the original deej connects but never reads a single slider value. This fork waits those out, so deej works on both old and new drivers.
- **Support for boards with a screen.** deej now tells the board what each slider controls: a label, an icon, whether the current OS can drive it, and whether its app is running. It also sends a keepalive, so the board can tell when deej isn't running. See [Boards with a screen](#boards-with-a-screen).
- **[`deej-5-sliders-oled`](./arduino/deej-5-sliders-oled) firmware** for an Arduino Nano with 5 knobs and a 1.3" OLED, which shows all of that.
- **Two new optional config keys,** `slider_labels` and `slider_icons`, to choose what the screen shows.
- **Fewer session glitches.** Refreshing the list of audio sessions can no longer happen at the same moment a slider is using them.

Everything stays compatible:

- Existing `config.yaml` files work unchanged. The original deej ignores the new keys, so one file works with both.
- The [original firmware](./arduino/deej-5-sliders-vanilla/deej-5-sliders-vanilla.ino) works with this deej. It ignores what deej sends.
- The OLED firmware works with the original deej. It just shows the labels from its own `config.h`.

## Features

- Bind apps to sliders
  - Several apps on one slider (for example, all your games)
  - The master volume
  - Your microphone's input level
  - All apps that aren't bound to any other slider
  - The app in focus (Windows only)
  - "System sounds" (Windows only)
  - A specific audio device, by name (Windows only)
- Edit the config while deej is running: it reloads automatically
- Show labels, icons and app status on a board with a screen
- Runs from the system tray, uses around 10 MB of memory
- Notifications when something's wrong (wrong COM port, broken config)

## How it works

### Hardware

- The sliders or knobs are wired to the analog pins of an Arduino Nano, Pro Micro or Uno, and powered from the board's 5V pin.
- The board connects to the PC over USB.

![Hardware schematic](assets/schematic.png)

### Software

- The firmware on the board reads the sliders and keeps sending their values over USB serial, one line at a time: `512|1023|0|300|800`, each from 0 to 1023.
- The deej app runs in the background on the PC. It reads those lines and sets each app's volume according to `config.yaml`.
- With a screen, deej also sends a few short lines back to the board (see [Boards with a screen](#boards-with-a-screen)).

Two firmwares are included:

| Firmware | Hardware | What it does |
|---|---|---|
| [`deej-5-sliders-vanilla`](./arduino/deej-5-sliders-vanilla/deej-5-sliders-vanilla.ino) | Any board, 5 sliders on A0–A4 | Sends slider values. The original firmware, unchanged. |
| [`deej-5-sliders-oled`](./arduino/deej-5-sliders-oled) | Arduino Nano, 5 knobs, SH1106 or SSD1306 I2C OLED | Smoothed knob values, plus a screen with levels, labels and app status. |

## Configuration

deej reads `config.yaml` from the folder it runs in. Changes are picked up as soon as you save the file, with no restart needed.

```yaml
slider_mapping:
  0: master
  1: chrome.exe
  2: spotify.exe
  3:
    - pathofexile_x64.exe
    - rocketleague.exe
  4: discord.exe

# set this to true if you want the controls inverted (i.e. top is 0%, bottom is 100%)
invert_sliders: false

# settings for connecting to the arduino board
com_port: COM4
baud_rate: 9600

# adjust the amount of signal noise reduction depending on your hardware quality
# supported values are "low" (excellent hardware), "default" (regular hardware) or "high" (bad, noisy hardware)
noise_reduction: default
```

Slider numbers start at 0, whichever analog pins you use. What you can put in `slider_mapping`:

| Target | Controls |
|---|---|
| `chrome.exe`, `spotify`, … | That app (case doesn't matter; on Linux, use the name without `.exe`) |
| a list of apps | All of them, or whichever of them is running |
| `master` | The default playback device |
| `mic` | The default recording device's input level |
| `deej.unmapped` | Every app not bound to another slider |
| `deej.current` | The app in focus (Windows only) |
| `system` | "System sounds" in the Windows mixer (Windows only) |
| `Speakers (Realtek High Definition Audio)` | That device, by its full name as shown in the Windows volume menu. Works for inputs and outputs (Windows only) |

On Linux, `com_port` is usually `/dev/ttyUSB0` (CH340 boards) or `/dev/ttyACM0`.

### Screen labels and icons (optional)

For boards with a screen, deej works out a label and an icon for each slider from `slider_mapping`:

- `master` → MASTER, `mic` → MIC, `deej.current` → ACTIVE, `deej.unmapped` → OTHER
- `spotify.exe` → SPOTIFY. The `.exe` is dropped and the name is uppercased.
- A group of the same kind of app → BROWSER, MUSIC, VOICE, CHAT, VIDEO or GAMES. A mixed group → first app's name with a `+`.
- Known browsers, music players, voice and chat apps, video players and game launchers get a matching icon. Other apps get a window icon.

To pick your own, add either key and list only the sliders you want to change:

```yaml
slider_labels:
  3: GAMES
slider_icons:
  3: game
```

Labels are cut to 10 characters. Icons: `speaker`, `music`, `globe`, `headset`, `window`, `mic`, `muted`, `game`, `chat`, `video`, `micoff`.

## Boards with a screen

![OLED firmware icons](arduino/deej-5-sliders-oled/tools/icons_preview.png)

The [`deej-5-sliders-oled`](./arduino/deej-5-sliders-oled) firmware drives a 1.3" SH1106 (or 0.96" SSD1306) I2C OLED on an Arduino Nano, mounted portrait next to 5 knobs. It shows:

- **Overview:** one bar per knob with its icon and level.
  - A **dotted bar with `NA`**: this OS can't drive that slider, for example `deej.current` on Linux.
  - A **hatched bar**: the slider's app isn't running.
  - **`--`**: the slider isn't mapped.
- **Focus view:** the knob you're turning, shown big with its name and level.
- **"NO DEEJ"** and an X in the corner when deej isn't running.
- A dimmed, moving screensaver after 5 seconds, then the screen turns off.

What deej sent last is saved in the board's EEPROM, so labels are there even before deej starts. Only changed bytes are written, so the EEPROM won't wear out.

**Setup**

1. Install the **U8g2** library in the Arduino IDE (Tools → Manage Libraries).
2. Open [`config.h`](./arduino/deej-5-sliders-oled/config.h) and set your knob pins (`KNOB_PINS`, in slider order), screen type and rotation. The OLED uses A4 (SDA) and A5 (SCL).
3. Upload to the Nano.
   - Board: "Arduino Nano".
   - Processor: "ATmega328P", or "ATmega328P (Old Bootloader)" if the upload fails.
4. Set `com_port` in `config.yaml` and start deej.

With the original deej, set `SHOW_LINK_STATUS = false` in `config.h`. The original never sends the keepalive, so the screen would always say "NO DEEJ".

**Changing the icons:** they're drawn as text art in [`tools/gen_icons.go`](./arduino/deej-5-sliders-oled/tools/gen_icons.go). Edit the art and run `go run gen_icons.go` in that folder. It rewrites `icons.h` and the preview image. A new icon also needs its name in `ICON_NAMES` in the sketch and in `displayIcons` in [`pkg/deej/display.go`](./pkg/deej/display.go).

**Protocol:** the lines deej sends are documented at the top of [`pkg/deej/display.go`](./pkg/deej/display.go). Any firmware can read them, and they never look like slider values.

## Build your own

### Parts

- An Arduino Nano, Pro Micro or Uno (or any board with USB serial). The Nano and Pro Micro are smaller and cheaper.
- Slider or rotary potentiometers, as many as you have free analog pins. Cheap 10K ones work fine. Make sure they're **linear**, not logarithmic.
- Wires.
- Optional: a 1.3" SH1106 or 0.96" SSD1306 I2C OLED, for the [screen firmware](#boards-with-a-screen).
- A box: 3D-printed, cardboard, anything.

Looking for ideas? See builds by the original deej community in the [community showcase](./community.md), many with enclosure designs. Tech Always also made a [build video](https://youtu.be/x2yXbFiiAeI) covering parts, assembly and software.

### Assembly

1. Wire everything according to the [schematic](#hardware). Check your pots with a multimeter.
2. Flash one of the [firmwares](#software). With the vanilla one, edit the sketch if you don't have exactly 5 sliders.
3. Open the Arduino serial monitor at 9600 baud. You should see lines like `0|240|1023|0|483`, and each number should go from 0 to 1023 as you move its slider.
4. Close the serial monitor, since only one program can use the port, and run deej.

The [FAQ](./docs/faq/faq.md) covers wiring, uploading, finding your COM port and editing the config in more detail.

## How to run

This fork doesn't publish releases yet. [Build deej from source](#building-from-source). On GitHub, every push to `master` also builds it, and you can download the result from the repository's **Actions** tab when signed in.

### Windows

1. Put `deej.exe` and `config.yaml` in the same folder.
2. Set `com_port` to your board's port. Device Manager → Ports (COM & LPT) shows it.
3. Run `deej.exe`. It appears in the system tray.
4. Optional: to start deej with Windows, put a shortcut to it in `%APPDATA%\Microsoft\Windows\Start Menu\Programs\Startup`.

### Linux

1. Build deej (see below) and put `config.yaml` next to it.
2. Set `com_port`, usually `/dev/ttyUSB0`.
3. Make sure your user can open serial ports: `sudo usermod -aG dialout $USER`, then log out and back in.
4. Use Linux app names in `slider_mapping` (`spotify`, not `spotify.exe`).

The tray icon uses the StatusNotifierItem (AppIndicator) standard. KDE Plasma, Xfce, Cinnamon, MATE, Budgie and most other desktops show it. GNOME needs the "AppIndicator and KStatusNotifierItem Support" extension, which Ubuntu enables by default. Without a tray, deej still works but has no icon. Stop it with `pkill deej-release` (or whatever you named the file). "Edit configuration" in the tray menu opens `config.yaml` in your default text editor.

## Building from source

You need Go 1.19 or newer (1.22 is known to work).

- **Windows:** run `pkg\deej\scripts\windows\build-release.bat`. It writes `deej-release.exe` in the repository root.
- **Linux:** run `pkg/deej/scripts/linux/build-release.sh` from the repository root. It writes `deej-release`.

The Linux build needs no system packages. It's pure Go and produces a static binary that runs on any distribution, so you can also build it on Windows or macOS: `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./pkg/deej/cmd` (or `GOARCH=arm64` for a Raspberry Pi).

`build-dev.bat` / `build-dev.sh` build a console version that shows logs. `deej -v` turns on verbose logging, which shows every line read from the board.

## Troubleshooting

- **"Can't connect to COMx"**: the port in `config.yaml` is wrong, or the board isn't plugged in. The port can change when you use a different USB socket.
- **deej connects but nothing happens when you move a slider**: check the board in the Arduino serial monitor first. With the original deej and a CH340 board, it can also be the driver problem this fork fixes.
- **"This serial port is busy"**: close the Arduino serial monitor, or any other program using the port.
- deej writes a log to `logs/deej-latest-run.log` next to the executable.

## Credits

- deej was created by **[Omri Harel](https://github.com/omriharel)**: the original [omriharel/deej](https://github.com/omriharel/deej). If you'd like to thank him, he has a [Ko-fi page](https://ko-fi.com/omriharel).
- The original deej community is still on [Discord](https://discord.gg/nf88NJu). It's about the original deej, not this fork, so report this fork's problems here on GitHub.
- The community builds in [community.md](./community.md) are by their respective makers.

## License

MIT, like the original. See [LICENSE](./LICENSE).
