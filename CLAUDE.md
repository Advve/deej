# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A fork of [omriharel/deej](https://github.com/omriharel/deej), published as **Advve/deej** (default branch
`main`).

deej is a Go tray app (Windows + Linux). It reads slider/knob values from an Arduino over serial and sets
per-app volumes. The original is unmaintained. This fork adds:
- a fix for new CH340 drivers
- pure-Go Linux builds
- automatic reconnecting
- a protocol for boards with a screen

`arduino/` holds the firmware:
- `deej-5-sliders-vanilla`: the original sketch.
- `deej-5-sliders-oled`: the main sketch, for a Nano with a 1.3" SH1106 OLED.

The Go module path is still `github.com/omriharel/deej`, kept from the original on purpose; the folder name
doesn't matter.

License: MIT, with the original © Omri Harel kept. The 3D-printed case design is published separately on
MakerWorld under CC BY-SA. Keep model files out of this repo, or label them CC BY-SA if they ever go in.

## Commands

Always build with `CGO_ENABLED=0`. Everything is pure Go and cross-compiles from one machine.

```bash
GOOS=windows go vet ./... && GOOS=linux go vet ./...   # *_linux.go / *_other.go aren't checked otherwise
go test ./...
go test -run TestAutostartEntry ./pkg/deej/              # single test (this one is Linux-only)

GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.buildType=release -H=windowsgui" -o dist/deej.exe ./pkg/deej/cmd
GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.buildType=release" -o dist/deej-linux-amd64 ./pkg/deej/cmd
GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w -X main.buildType=release" -o dist/deej-linux-arm64 ./pkg/deej/cmd

go run ./tools/notices          # after any dependency change: regenerates THIRD-PARTY-NOTICES.txt (CI checks it)
go generate ./pkg/deej/cmd      # after changing winres/winres.json: rewrites rsrc_windows_amd64.syso
```

- **Build types:** without `buildType=release` the build is a console "dev" build that logs to the terminal.
  The release build logs to `logs/deej-latest-run.log`.
- **Flags:** `-v` gives verbose logs (every serial line); `-version` prints the version.
- **Firmware:** `arduino-cli compile --fqbn arduino:avr:nano:cpu=atmega328 arduino/deej-5-sliders-oled`
  (needs U8g2). Use `cpu=atmega328old` if the upload fails. CI compiles every sketch in `arduino/` for a Nano.
- **OLED icons are generated:** edit the art in `arduino/deej-5-sliders-oled/tools/gen_icons.go` and run
  `go run gen_icons.go` in that folder. Don't edit `icons.h` by hand.
- **Testing next to a running deej:** a second copy started alongside the user's own would fight it for the
  board's port. Give the test copy a `config.yaml` with a dummy `com_port` (e.g. `COM99`).

## Releasing

Same scheme as the author's wled-audio-sync and RivalGUI repos. Full steps are in
`docs/CONTRIBUTING.md#releasing`.

- **Version:** lives in `pkg/deej/cmd/version.go` and in four fields of `winres/winres.json`. After changing
  those, run `go generate ./pkg/deej/cmd`. Both must equal the tag `vX.Y.Z`.
- **Changelog:** the README's `## Changelog`. One `- **X.Y.Z** summary` bullet per version, with
  two-space-indented sub-bullets. The release notes are generated from it.
  - Unreleased work goes under `- **Unreleased**` until the author picks a version.
- **Check:** `go run ./tools/release --check X.Y.Z` checks the version fields and the changelog entry.
  `go run ./tools/release X.Y.Z` prints the notes; the downloads table is a template in that tool.
- **CI** (`.github/workflows/build.yml`, ubuntu-24.04):
  - **Every push and PR:** gofmt, vet for both OSes, tests, notices up to date, the three builds, and the
    firmware compile.
  - **On a `v*` tag:** publishes "deej X.Y.Z" with the programs, `config.yaml`, `SHA256SUMS.txt` and the
    notices.
- **Targets:** only Windows amd64, Linux amd64 and Linux arm64.
- **Signing:** the exe is not code-signed, by choice.

## Architecture (`pkg/deej`)

- **Start-up** (`cmd/main.go`):
  - Order: flags, then `useExecutableDir()`, then the logger, then `NewDeej` and `Initialize`.
  - `config.yaml`, `logs/` and `preferences.yaml` are relative paths. If `config.yaml` isn't in the current
    folder but is next to the exe, main switches to the exe's folder.
  - This matters for autostart: Windows runs Run-key programs with System32 as the current folder.
- **`deej.go`:** wires config, serial, the session map and the tray. `systray.Run` must stay on the main
  goroutine. `DEEJ_NO_TRAY_ICON` skips the tray.
- **`config.go`:** viper loads `config.yaml` and watches it for changes; subscribers reload on change.
  - New config keys must stay optional: old configs, the original deej app and the vanilla firmware must
    keep working.
- **`serial.go`:** opens the port and reads `a|b|c|d|e` lines.
  - Reconnects every 2 s after unplug or sleep, and waits for a missing port at startup.
  - Lets go of a busy port, so flashing works while deej runs.
  - `emptyReadRetrier` works around CH340 drivers 3.8+ (e.g. 3.9.2024), whose reads return no data and no
    error. Stock deej reads nothing with those drivers.
- **`display.go`:** the PC → board protocol, documented at the top of the file.
  - `>H` header, `>S` per slider (icon, flags, label), `>E` end.
  - `>A` is a running-apps mask that doubles as the keepalive.
  - If you change the protocol, change `arduino/deej-5-sliders-oled` in the same commit.
- **`session_*` / `session_finder_*`:** WASAPI (go-wca) on Windows, PulseAudio/PipeWire on Linux.
  `session_map.go` maps slider targets (including `master`, `deej.current`, `deej.unmapped`) to sessions.
- **`tray.go`:** uses fyne.io/systray, which on Linux talks D-Bus StatusNotifierItem and needs no GTK.
  Menu items:
  - Edit configuration
  - Re-scan sessions
  - Start with Windows / Start at login
  - Version
  - Quit
- **Platform split by build tags:** `_windows.go` vs `_other.go` (`//go:build !windows`), or `_linux.go`.
  - **`theme_*`:** the Windows tray menu follows the dark/light theme via uxtheme ordinals 135/136/104. The
    registry is polled every 2 s, so a theme change applies live. On Linux the desktop draws the menu.
  - **`autostart_*`:** on Windows, an HKCU `...\CurrentVersion\Run` value `deej`. On Linux,
    `~/.config/autostart/deej.desktop`, with spec-correct two-layer quoting in `Exec=` and `Path=` set to the
    exe's folder.
    - `repairAutostart` re-points the entry only when its stored path no longer exists.
    - Mirrors wled-audio-sync's implementation.
  - **`util_*`:** foreground-window lookup and opening files.
  - **`cmd/console_*`:** `attachConsole` for `-version` in the `-H=windowsgui` build.

## Firmware and hardware

- **Board:** a USB-C Arduino Nano clone with a CH340 (USB VID 1A86, PID 7523). deej holds the serial port,
  so close it before flashing.
- **Screen:** 1.3" SH1106 on I2C (A4/A5).
- **Knobs:** `KNOB_PINS` = `{A6, A3, A2, A1, A0}` in slider order, left to right on the bar. The pots are wired
  mirrored, so A0 is next to the screen.
- **Assembly guide and print profiles** are on MakerWorld (linked from the README's OLED section).
  Troubleshooting lives in the README and `docs/faq`.

## Project rules

- **Stage files by name, never `git add .` / `-A`.** The checkout can hold untracked local files that must not
  be published.
- **Keep personal data out of committed files:** no real IPs, usernames or local paths.
- **Credit the original author** (Omri Harel) and link the original repo. Report this fork's problems here,
  not to the original Discord.
- **Commit or push only when the author asks.**
