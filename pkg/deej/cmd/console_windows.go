package main

import (
	"os"
	"syscall"
)

// attachConsole gives the windowless (GUI) release build a console for command-line output such
// as -version: the terminal it was started from, or a new window.
func attachConsole() {
	if h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE); err == nil && h != 0 && h != syscall.InvalidHandle {
		return // already has a console (e.g. the dev build)
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	const attachParentProcess = ^uintptr(0)
	if r, _, _ := kernel32.NewProc("AttachConsole").Call(attachParentProcess); r == 0 {
		kernel32.NewProc("AllocConsole").Call()
	}
	if f, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
	if f, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
		os.Stdin = f
	}
}
