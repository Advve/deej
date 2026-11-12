//go:build !windows

package deej

// Linux tray menus are drawn by the desktop itself, so they already follow its theme.
func followSystemMenuTheme() {}
func watchSystemTheme()      {}
