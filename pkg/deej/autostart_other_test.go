//go:build !windows

package deej

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAutostartEntry(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	if autostartEnabled() || autostartPath() != "" {
		t.Fatal("autostart is on before enabling it")
	}

	if err := setAutostart(true); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if !autostartEnabled() || autostartPath() != exe {
		t.Fatalf("autostartPath() = %q, want %q", autostartPath(), exe)
	}

	b, err := os.ReadFile(autostartFile())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\nPath="+stringEscape(filepath.Dir(exe))+"\n") {
		t.Errorf("entry has no Path= for the program's folder:\n%s", b)
	}

	if err := setAutostart(false); err != nil {
		t.Fatal(err)
	}
	if autostartEnabled() {
		t.Fatal("autostart still on after disabling it")
	}
	if err := setAutostart(false); err != nil {
		t.Fatalf("disabling twice: %v", err)
	}
}

func TestExecQuoting(t *testing.T) {
	// what the Desktop Entry spec says each path is written as
	for path, want := range map[string]string{
		"/usr/bin/deej":          `"/usr/bin/deej"`,
		"/opt/my deej/deej":      `"/opt/my deej/deej"`,
		"/home/a/$x/deej":        `"/home/a/\\$x/deej"`,
		`/home/a/b\c/deej`:       `"/home/a/b\\\\c/deej"`,
		`/home/a/"q"/deej`:       `"/home/a/\\"q\\"/deej"`,
		"/home/a/`t`/deej":       "\"/home/a/\\\\`t\\\\`/deej\"",
		"/home/a/100%/deej":      `"/home/a/100%%/deej"`,
		"/home/a/$HOME \"x\" \\": `"/home/a/\\$HOME \\"x\\" \\\\"`,
	} {
		got := execQuote(path)
		if got != want {
			t.Errorf("execQuote(%q) = %s, want %s", path, got, want)
		}
		if back := execProgram(got); back != path {
			t.Errorf("execProgram(%s) = %q, want %q", got, back, path)
		}
	}

	// entries written by hand or by other tools
	for exec, want := range map[string]string{
		"/usr/bin/deej --verbose": "/usr/bin/deej",
		`"/opt/my deej/deej" -v`:  "/opt/my deej/deej",
		`/opt/my\sdeej/deej`:      "/opt/my",
	} {
		if got := execProgram(exec); got != want {
			t.Errorf("execProgram(%s) = %q, want %q", exec, got, want)
		}
	}
}
