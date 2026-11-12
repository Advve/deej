package deej

import (
	"os"
	"path/filepath"

	"github.com/omriharel/deej/pkg/deej/util"
)

// Where deej keeps its files. LocateFiles looks for config.yaml, in order:
//
//  1. in the current folder (how deej has always worked)
//  2. next to the program; deej then switches to that folder, so logs/ goes there too. This is
//     also what makes autostart work: Windows starts Run-key programs in System32.
//  3. on Linux, in $XDG_CONFIG_HOME/deej (~/.config/deej), with the log and preferences.yaml in
//     $XDG_STATE_HOME/deej (~/.local/state/deej)
//
// If none has it, Linux uses the folders from 3, so the "can't find" message points there, and
// Windows keeps the current folder.
var (
	userConfigFilepath = "config.yaml"
	logDirectory       = "logs"

	configMissingHint = "config.yaml must be in the same folder as deej."
)

// LocateFiles decides where config.yaml and the logs are. Call it before NewLogger.
func LocateFiles() {
	if util.FileExists(userConfigFilepath) {
		return
	}
	if exe, err := os.Executable(); err == nil {
		if exe, err = filepath.EvalSymlinks(exe); err == nil {
			dir := filepath.Dir(exe)
			if util.FileExists(filepath.Join(dir, userConfigFilepath)) && os.Chdir(dir) == nil {
				return
			}
		}
	}
	if configDir, stateDir, ok := xdgDirs(); ok {
		userConfigFilepath = filepath.Join(configDir, "config.yaml")
		logDirectory = stateDir
		configMissingHint = "Put config.yaml in " + configDir + ", or in the same folder as deej."
	}
}

// ConfigFile returns the path of config.yaml that deej uses.
func ConfigFile() string {
	if p, err := filepath.Abs(userConfigFilepath); err == nil {
		return p
	}
	return userConfigFilepath
}

// LogDirectory returns the folder for the log file, crash logs and preferences.yaml.
func LogDirectory() string {
	if p, err := filepath.Abs(logDirectory); err == nil {
		return p
	}
	return logDirectory
}
