package deej

// Windows keeps config.yaml and logs/ next to deej.exe.
func xdgDirs() (configDir, stateDir string, ok bool) { return "", "", false }
