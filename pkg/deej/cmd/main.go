package main

// The Windows resources (icon, manifest, version info) come from winres/winres.json. After changing
// it, run `go generate ./pkg/deej/cmd` from the repository root to rewrite rsrc_windows_amd64.syso.
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in ../../../winres/winres.json --arch amd64 --out rsrc

import (
	"flag"
	"fmt"
	"os"

	"github.com/omriharel/deej/pkg/deej"
)

var (
	// set by the build (-ldflags "-X main.buildType=release -X main.gitCommit=..."); the release
	// build type logs to a file instead of the console
	gitCommit string
	buildType string

	verbose     bool
	showVersion bool
)

func init() {
	flag.BoolVar(&verbose, "verbose", false, "show verbose logs (useful for debugging serial)")
	flag.BoolVar(&verbose, "v", false, "shorthand for --verbose")
	flag.BoolVar(&showVersion, "version", false, "print the version and exit")
	flag.Parse()
}

func main() {
	if showVersion {
		attachConsole()
		fmt.Println("deej", version)
		os.Exit(0)
	}

	// where config.yaml and the logs are: the current folder, next to deej, or on Linux ~/.config/deej
	deej.LocateFiles()

	// first we need a logger
	logger, err := deej.NewLogger(buildType)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logger: %v", err))
	}

	named := logger.Named("main")
	named.Debug("Created logger")

	named.Infow("Version info",
		"version", version,
		"gitCommit", gitCommit,
		"buildType", buildType)

	named.Infow("Files", "config", deej.ConfigFile(), "logs", deej.LogDirectory())

	// provide a fair warning if the user's running in verbose mode
	if verbose {
		named.Debug("Verbose flag provided, all log messages will be shown")
	}

	// create the deej instance
	d, err := deej.NewDeej(logger, verbose)
	if err != nil {
		named.Fatalw("Failed to create deej object", "error", err)
	}

	// shown (greyed out) in the tray menu
	versionString := "Version " + version
	if buildType != "" && buildType != "release" {
		versionString += " (" + buildType + ")"
	}
	d.SetVersion(versionString)

	// onwards, to glory
	if err = d.Initialize(); err != nil {
		named.Fatalw("Failed to initialize deej", "error", err)
	}
}
