//go:build !windows

package deej

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// "Start at login" is a freedesktop.org autostart entry, so deej starts with the desktop session.
const (
	autostartLabel   = "Start at login"
	autostartTooltip = "Start deej when you log in"
)

func autostartFile() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "autostart", "deej.desktop")
}

func autostartEnabled() bool {
	_, err := os.Stat(autostartFile())
	return err == nil
}

// autostartPath returns the program path saved in the autostart entry ("" if none).
func autostartPath() string {
	b, err := os.ReadFile(autostartFile())
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if exec, ok := strings.CutPrefix(strings.TrimSpace(line), "Exec="); ok {
			return execProgram(exec)
		}
	}
	return ""
}

// The Desktop Entry spec quotes an Exec argument in two layers: inside double quotes, " ` $ \ are
// escaped with a backslash (and % is written %%), and then the whole value, like any string value,
// has each backslash doubled. So a literal $ is written \\$ and a literal \ is written \\\\.

// execQuote returns the Exec value that runs the program at path.
func execQuote(path string) string {
	q := `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`).Replace(path) + `"`
	return stringEscape(strings.ReplaceAll(q, "%", "%%"))
}

// execProgram returns the program path from an Exec value: its first argument.
func execProgram(exec string) string {
	exec = strings.ReplaceAll(stringUnescape(exec), "%%", "%")
	rest, quoted := strings.CutPrefix(exec, `"`)
	if !quoted {
		return strings.Fields(exec + " ")[0]
	}
	var path strings.Builder
	for i := 0; i < len(rest); i++ {
		switch c := rest[i]; {
		case c == '"':
			return path.String()
		case c == '\\' && i+1 < len(rest):
			i++
			path.WriteByte(rest[i])
		default:
			path.WriteByte(c)
		}
	}
	return path.String() // no closing quote; take what's there
}

// stringEscape escapes a Desktop Entry string value.
func stringEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, "\t", `\t`, "\r", `\r`).Replace(s)
}

// stringUnescape reverses stringEscape (and \s, a space).
func stringUnescape(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			out.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 's':
			out.WriteByte(' ')
		case 'n':
			out.WriteByte('\n')
		case 't':
			out.WriteByte('\t')
		case 'r':
			out.WriteByte('\r')
		case '\\':
			out.WriteByte('\\')
		default: // not an escape the spec knows; keep it as written
			out.WriteByte('\\')
			out.WriteByte(s[i])
		}
	}
	return out.String()
}

func setAutostart(enable bool) error {
	if !enable {
		if err := os.Remove(autostartFile()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Path is the folder deej starts in, where config.yaml is
	entry := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=deej
Comment=Hardware volume mixer
Exec=%s
Path=%s
Terminal=false
X-GNOME-Autostart-enabled=true
`, execQuote(exe), stringEscape(filepath.Dir(exe)))
	if err := os.MkdirAll(filepath.Dir(autostartFile()), 0o755); err != nil {
		return err
	}
	return os.WriteFile(autostartFile(), []byte(entry), 0o644)
}
