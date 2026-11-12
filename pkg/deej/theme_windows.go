package deej

import (
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`

// Dark context menus use uxtheme.dll's undocumented-but-stable exports (by ordinal, Windows 10
// 1809+), the same ones many desktop apps use. If they're missing, the menu just stays light.
var (
	uxthemeOnce                      sync.Once
	setPreferredAppMode, flushThemes uintptr // ordinals 135, 136
	refreshColorPolicy               uintptr // ordinal 104
)

func loadUxtheme() {
	uxthemeOnce.Do(func() {
		h, err := windows.LoadLibraryEx("uxtheme.dll", 0, windows.LOAD_LIBRARY_SEARCH_SYSTEM32)
		if err != nil {
			return
		}
		setPreferredAppMode, _ = windows.GetProcAddressByOrdinal(h, 135)
		flushThemes, _ = windows.GetProcAddressByOrdinal(h, 136)
		refreshColorPolicy, _ = windows.GetProcAddressByOrdinal(h, 104)
	})
}

// followSystemMenuTheme makes the tray menu follow the Windows theme (dark/light).
// Call it before the menu is created.
func followSystemMenuTheme() {
	loadUxtheme()
	if setPreferredAppMode == 0 || flushThemes == 0 {
		return
	}
	const allowDark = 1 // follow the system setting
	syscallN(setPreferredAppMode, allowDark)
	syscallN(flushThemes)
}

// watchSystemTheme re-reads the theme whenever it changes in Windows settings, so the next time
// the menu opens it's drawn in the new colours. It runs until the process exits.
func watchSystemTheme() {
	loadUxtheme()
	if flushThemes == 0 {
		return
	}
	go func() {
		last := themeState()
		for range time.Tick(2 * time.Second) {
			if now := themeState(); now != last {
				last = now
				if refreshColorPolicy != 0 {
					syscallN(refreshColorPolicy)
				}
				syscallN(flushThemes)
			}
		}
	}()
}

// themeState returns the "apps" and "Windows" (taskbar) light-theme settings, as one value to compare.
func themeState() [2]uint64 {
	k, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if err != nil {
		return [2]uint64{}
	}
	defer k.Close()
	apps, _, _ := k.GetIntegerValue("AppsUseLightTheme")
	system, _, _ := k.GetIntegerValue("SystemUsesLightTheme")
	return [2]uint64{apps, system}
}

func syscallN(fn uintptr, args ...uintptr) { _, _, _ = syscall.SyscallN(fn, args...) }
