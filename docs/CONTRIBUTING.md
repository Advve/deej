# Contributing

Bug reports, ideas and pull requests are welcome here on GitHub. This is a fork of [omriharel/deej](https://github.com/omriharel/deej), so please report problems with this version here rather than to the original project.

## Ground rules

- **Stay compatible.** Existing `config.yaml` files, the original deej app and the vanilla firmware should keep working with whatever you change. New config keys should be optional.
- **Keep it simple to build.** Many people build deej as their first electronics project.
- **Keep the app and the firmware in sync.** The lines deej sends to the board are documented at the top of [`pkg/deej/display.go`](../pkg/deej/display.go). If you change them, update [`arduino/deej-5-sliders-oled`](../arduino/deej-5-sliders-oled) in the same pull request.

## Development

- Go 1.14 or newer (1.22 is known to work). Build with the scripts in [`pkg/deej/scripts`](../pkg/deej/scripts), which embed the version, or see [Building from source](../README.md#building-from-source).
- Run the tests with `go test ./pkg/deej/`.
- Firmware: open the sketch in the Arduino IDE, or compile it with `arduino-cli compile --fqbn arduino:avr:nano arduino/deej-5-sliders-oled`. The OLED sketch needs the U8g2 library.
- The OLED icons are generated: edit the art in [`tools/gen_icons.go`](../arduino/deej-5-sliders-oled/tools/gen_icons.go) and run `go run gen_icons.go` in that folder, rather than editing `icons.h` by hand.
- Every push and pull request to `master` is built for Windows and Linux by GitHub Actions.
