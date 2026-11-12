//go:build !windows

package deej

import (
	"os"
	"path/filepath"
	"testing"
)

// runLocate runs LocateFiles from an empty folder with fresh XDG folders and restores everything after.
func runLocate(t *testing.T, setup func(cwd, config, state string)) (cwd, config, state string) {
	t.Helper()
	cwd, config, state = t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", config)
	t.Setenv("XDG_STATE_HOME", state)

	prevDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	prevConfig, prevLogs, prevHint := userConfigFilepath, logDirectory, configMissingHint
	t.Cleanup(func() {
		_ = os.Chdir(prevDir)
		userConfigFilepath, logDirectory, configMissingHint = prevConfig, prevLogs, prevHint
	})
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}

	if setup != nil {
		setup(cwd, config, state)
	}
	LocateFiles()
	return cwd, config, state
}

func TestLocateFilesXDG(t *testing.T) {
	_, config, state := runLocate(t, func(_, config, _ string) {
		if err := os.MkdirAll(filepath.Join(config, "deej"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(config, "deej", "config.yaml"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if want := filepath.Join(config, "deej", "config.yaml"); ConfigFile() != want {
		t.Errorf("config = %q, want %q", ConfigFile(), want)
	}
	if want := filepath.Join(state, "deej"); LogDirectory() != want {
		t.Errorf("logs = %q, want %q", LogDirectory(), want)
	}
}

func TestLocateFilesCurrentFolderWins(t *testing.T) {
	cwd, _, _ := runLocate(t, func(cwd, config, _ string) {
		for _, dir := range []string{cwd, filepath.Join(config, "deej")} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "config.yaml"), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	})
	cwd, _ = filepath.EvalSymlinks(cwd) // macOS-style /tmp links
	if got, _ := filepath.EvalSymlinks(ConfigFile()); got != filepath.Join(cwd, "config.yaml") {
		t.Errorf("config = %q, want the one in the current folder", ConfigFile())
	}
	if userConfigFilepath != "config.yaml" || logDirectory != "logs" {
		t.Errorf("paths changed although config.yaml is in the current folder: %q, %q", userConfigFilepath, logDirectory)
	}
}

func TestLocateFilesNoConfigPointsToXDG(t *testing.T) {
	_, config, _ := runLocate(t, nil)
	if want := filepath.Join(config, "deej", "config.yaml"); ConfigFile() != want {
		t.Errorf("config = %q, want %q", ConfigFile(), want)
	}
	if want := "Put config.yaml in " + filepath.Join(config, "deej") + ", or in the same folder as deej."; configMissingHint != want {
		t.Errorf("hint = %q", configMissingHint)
	}
}

func TestXDGRelativePathIgnored(t *testing.T) {
	t.Setenv("HOME", "/home/someone")
	t.Setenv("XDG_CONFIG_HOME", "relative/dir")
	t.Setenv("XDG_STATE_HOME", "")
	config, state, ok := xdgDirs()
	if !ok || config != "/home/someone/.config/deej" || state != "/home/someone/.local/state/deej" {
		t.Errorf("xdgDirs() = %q, %q, %v", config, state, ok)
	}
}
