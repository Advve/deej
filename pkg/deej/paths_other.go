//go:build !windows

package deej

import (
	"os"
	"path/filepath"
)

// xdgDirs returns deej's folders under the XDG base directories: ~/.config/deej and
// ~/.local/state/deej, or wherever $XDG_CONFIG_HOME and $XDG_STATE_HOME point.
func xdgDirs() (configDir, stateDir string, ok bool) {
	base := func(env, fallback string) string {
		// the spec says relative paths in these variables are invalid and must be ignored
		if dir := os.Getenv(env); filepath.IsAbs(dir) {
			return dir
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(home, fallback)
	}
	config, state := base("XDG_CONFIG_HOME", ".config"), base("XDG_STATE_HOME", filepath.Join(".local", "state"))
	if config == "" || state == "" {
		return "", "", false
	}
	return filepath.Join(config, "deej"), filepath.Join(state, "deej"), true
}
