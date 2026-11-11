## Developer scripts

This document lists the various scripts in the project and their purposes.

> Note: All scripts are meant to be run from the root of the repository, i.e. from the _root_ `deej` directory: `.\pkg\deej\scripts\...\whatever.bat`. They're not guaranteed to work correctly if run from another directory.

### Windows

- [`build-dev.bat`](./windows/build-dev.bat): Builds deej with a console window, for development purposes
- [`build-release.bat`](./windows/build-release.bat): Builds deej as a standalone tray application without a console window, for releases
- [`build-all.bat`](./windows/build-all.bat): Helper script to build all variants
- [`make-icon.bat`](./windows/make-icon.bat): Converts a .ico file to an icon byte array in a Go file. Used by our systray library. You shouldn't need to run this unless you change the deej logo
- [`make-rsrc.bat`](./windows/make-rsrc.bat): Runs `go generate ./pkg/deej/cmd`, which rewrites `rsrc_windows_amd64.syso` next to `main.go` from [`winres/winres.json`](../../../winres/winres.json): the icon, manifest and version information of `deej.exe`. Run it after changing that file.

Releases aren't made with these scripts: pushing a `vX.Y.Z` tag makes GitHub Actions build and publish one. See [Releasing](../../../docs/CONTRIBUTING.md#releasing).

### Linux

- [`build-dev.sh`](./linux/build-dev.sh): Builds deej for development purposes
- [`build-release.sh`](./linux/build-release.sh): Builds deej for releases (a static binary: no cgo, no system libraries needed)
- [`build-all.sh`](./linux/build-all.sh): Helper script to build all variants
