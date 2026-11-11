//go:build !windows

package main

// attachConsole is only needed on Windows, where the release build has no console.
func attachConsole() {}
