# Contributing

Bug reports, ideas and pull requests are welcome here on GitHub. This is a fork of [omriharel/deej](https://github.com/omriharel/deej), so please report problems with this version here rather than to the original project.

## Ground rules

- **Stay compatible.** Existing `config.yaml` files, the original deej app and the vanilla firmware should keep working with whatever you change. New config keys should be optional.
- **Keep it simple to build.** Many people build deej as their first electronics project.
- **Keep the app and the firmware in sync.** The lines deej sends to the board are documented at the top of [`pkg/deej/display.go`](../pkg/deej/display.go). If you change them, update [`arduino/deej-5-sliders-oled`](../arduino/deej-5-sliders-oled) in the same pull request.

## Development

- Go 1.22 or newer. Build with the scripts in [`pkg/deej/scripts`](../pkg/deej/scripts), or see [Building from source](../README.md#building-from-source).
- Before pushing, run what CI runs: `gofmt -l .` (should print nothing), `go vet ./...` with `GOOS=windows` and `GOOS=linux`, and `go test ./...`.
- After adding, removing or updating a dependency, run `go run ./tools/notices` to regenerate [`THIRD-PARTY-NOTICES.txt`](../THIRD-PARTY-NOTICES.txt). CI fails if it's out of date.
- The Windows icon, manifest and version information come from [`winres/winres.json`](../winres/winres.json). After changing it, run `go generate ./pkg/deej/cmd`, which rewrites `pkg/deej/cmd/rsrc_windows_amd64.syso` with [go-winres](https://github.com/tc-hib/go-winres).
- Firmware: open the sketch in the Arduino IDE, or compile it with `arduino-cli compile --fqbn arduino:avr:nano arduino/deej-5-sliders-oled`. The OLED sketch needs the U8g2 library.
- The OLED icons are generated: edit the art in [`tools/gen_icons.go`](../arduino/deej-5-sliders-oled/tools/gen_icons.go) and run `go run gen_icons.go` in that folder, rather than editing `icons.h` by hand.

## Releasing

GitHub Actions ([`.github/workflows/build.yml`](../.github/workflows/build.yml)) builds and checks every push and pull request on one Linux runner. It cross-compiles `deej.exe`, `deej-linux-amd64` and `deej-linux-arm64`, and keeps them as a downloadable artifact for a while.

To publish version X.Y.Z:

1. Set the version in both places that hold it:
   - `pkg/deej/cmd/version.go` (`const version`, shown in the tray menu and by `deej -version`)
   - `winres/winres.json`: four fields. Then run `go generate ./pkg/deej/cmd` so the `.exe` properties match.
2. Add a changelog entry to the README's `## Changelog`: one `- **X.Y.Z** summary` bullet, with two-space-indented sub-bullets. The release notes are generated from it.
3. Check: `go run ./tools/release --check X.Y.Z`. `go run ./tools/release X.Y.Z` shows the release notes. The downloads table and install notes are a template inside that tool.
4. Commit, then tag and push: `git tag vX.Y.Z && git push origin vX.Y.Z`.

On the tag, CI checks the tag against the version, builds, and publishes the release "deej X.Y.Z" with the three programs, `config.yaml`, `SHA256SUMS.txt`, `SHA256SUMS.txt.sig`, `THIRD-PARTY-NOTICES.txt` and the generated notes.

deej's updater installs a release only if `SHA256SUMS.txt.sig` matches the public key built into deej (`updatePublicKey` in `pkg/deej/update.go`), so the repository needs the `UPDATE_SIGNING_KEY` secret. The key pair was made once with `go run ./tools/release keygen <file>`. Keep a backup of the private key: if it's lost, users have to update by hand once more to get a build with a new public key.
