package deej

import (
	"os"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// "Start with Windows" is an entry in the current user's Run key, so it needs no admin rights
// and shows up (and can be turned off) in Task Manager's Startup apps.
const (
	runKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	runName = "deej"

	autostartLabel   = "Start with Windows"
	autostartTooltip = "Start deej when you sign in to Windows"
)

func autostartEnabled() bool {
	return autostartPath() != ""
}

// autostartPath returns the program path saved in the startup entry ("" if none).
func autostartPath() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, err := k.GetStringValue(runName)
	if err != nil {
		return ""
	}
	return strings.Trim(v, `"`)
}

func setAutostart(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if !enable {
		if err := k.DeleteValue(runName); err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return k.SetStringValue(runName, `"`+exe+`"`)
}
