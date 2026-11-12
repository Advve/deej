// Command release checks a version before tagging and writes the GitHub release notes.
//
//	go run ./tools/release --check 1.2.3   fails unless every version field equals 1.2.3 and
//	                                       the README's changelog has an entry for it
//	go run ./tools/release 1.2.3           prints the release notes (Markdown)
//	go run ./tools/release sign FILE       writes FILE.sig, signed with the UPDATE_SIGNING_KEY
//	                                       environment variable (CI signs SHA256SUMS.txt)
//	go run ./tools/release keygen FILE     makes a new update signing key: writes the private key
//	                                       to FILE (for the UPDATE_SIGNING_KEY secret) and prints
//	                                       the public key for updatePublicKey in pkg/deej/update.go
//
// Run it from the repository root. CI runs --check, the notes and sign on version tags.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
)

const versionFile = "pkg/deej/cmd/version.go"

func main() {
	args := os.Args[1:]
	if len(args) == 2 && args[0] == "sign" {
		sign(args[1])
		return
	}
	if len(args) == 2 && args[0] == "keygen" {
		keygen(args[1])
		return
	}
	check := len(args) == 2 && args[0] == "--check"
	if !(check || len(args) == 1) || !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(args[len(args)-1]) {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/release [--check] X.Y.Z | sign FILE | keygen FILE")
		os.Exit(1)
	}
	v := args[len(args)-1]
	if check {
		if problems := checkVersions(v); len(problems) > 0 {
			for _, p := range problems {
				fmt.Fprintln(os.Stderr, p)
			}
			os.Exit(1)
		}
		fmt.Printf("All versions are %s and the changelog has an entry for it.\n", v)
		return
	}
	entry, ok := changelogEntry(v)
	if !ok {
		fmt.Fprintf(os.Stderr, "README.md has no changelog entry for %s\n", v)
		os.Exit(1)
	}
	fmt.Print(notes(v, entry))
}

func fail(err any) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// sign writes path+".sig": the base64 ed25519 signature of the file, which deej's updater
// checks against its built-in public key before installing anything.
func sign(path string) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("UPDATE_SIGNING_KEY")))
	if err != nil || len(key) != ed25519.PrivateKeySize {
		fail("UPDATE_SIGNING_KEY is missing or not a base64 ed25519 private key (see `go run ./tools/release keygen`)")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err)
	}
	sig := ed25519.Sign(ed25519.PrivateKey(key), data)
	if err := os.WriteFile(path+".sig", []byte(base64.StdEncoding.EncodeToString(sig)+"\n"), 0o644); err != nil {
		fail(err)
	}
	fmt.Printf("Signed %s with public key %s\n", path,
		base64.StdEncoding.EncodeToString(ed25519.PrivateKey(key).Public().(ed25519.PublicKey)))
}

// keygen makes a new signing key pair. The private key goes to a file, not the terminal.
func keygen(path string) {
	if _, err := os.Stat(path); err == nil {
		fail(path + " already exists; not overwriting a key")
	}
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(priv)+"\n"), 0o600); err != nil {
		fail(err)
	}
	fmt.Printf("Private key written to %s: store it as the UPDATE_SIGNING_KEY repository secret,\n", path)
	fmt.Println("keep a backup somewhere safe, and never commit it.")
	fmt.Printf("Public key, for updatePublicKey in pkg/deej/update.go:\n%s\n", base64.StdEncoding.EncodeToString(pub))
}

func read(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// checkVersions compares every place that holds the version with the wanted one.
func checkVersions(want string) []string {
	var problems []string
	mismatch := func(where, got string) {
		if got != want && got != want+".0" {
			problems = append(problems, fmt.Sprintf("%s has version %q, the tag wants %s", where, got, want))
		}
	}

	if m := regexp.MustCompile(`const version = "([^"]*)"`).FindStringSubmatch(read(versionFile)); m != nil {
		mismatch(versionFile, m[1])
	} else {
		problems = append(problems, versionFile+" has no `const version`")
	}

	var res struct {
		Version map[string]map[string]struct {
			Fixed struct {
				File    string `json:"file_version"`
				Product string `json:"product_version"`
			} `json:"fixed"`
			Info map[string]map[string]string `json:"info"`
		} `json:"RT_VERSION"`
	}
	if err := json.Unmarshal([]byte(read("winres/winres.json")), &res); err != nil {
		problems = append(problems, "winres/winres.json: "+err.Error())
	}
	for _, langs := range res.Version {
		for _, vi := range langs {
			mismatch("winres/winres.json file_version", vi.Fixed.File)
			mismatch("winres/winres.json product_version", vi.Fixed.Product)
			for _, info := range vi.Info {
				mismatch("winres/winres.json FileVersion", info["FileVersion"])
				mismatch("winres/winres.json ProductVersion", info["ProductVersion"])
			}
		}
	}
	if len(res.Version) == 0 {
		problems = append(problems, "winres/winres.json has no RT_VERSION")
	}

	for _, p := range problems {
		if strings.HasPrefix(p, "winres/") {
			problems = append(problems, "(after changing winres/winres.json, run `go generate ./pkg/deej/cmd` so the .exe picks it up)")
			break
		}
	}
	if _, ok := changelogEntry(want); !ok {
		problems = append(problems, "README.md has no changelog entry for "+want)
	}
	// a release without the key could never install the next one by itself
	if !regexp.MustCompile(`updatePublicKey = "[A-Za-z0-9+/=]{40,}"`).MatchString(read("pkg/deej/update.go")) {
		problems = append(problems, "pkg/deej/update.go has no updatePublicKey (see `go run ./tools/release keygen`)")
	}
	return problems
}

// changelogEntry finds "- **X.Y.Z** summary" in the README and the indented sub-bullets under it.
// It returns the summary and the sub-bullets (with two spaces of indent removed).
func changelogEntry(v string) ([]string, bool) {
	lines := strings.Split(read("README.md"), "\n")
	marker := "- **" + v + "**"
	for i, line := range lines {
		if !strings.HasPrefix(line, marker) {
			continue
		}
		entry := []string{strings.TrimLeft(strings.TrimPrefix(line, marker), " ,:-")}
		for _, l := range lines[i+1:] {
			if !strings.HasPrefix(l, "  ") {
				break
			}
			l = strings.TrimPrefix(l, "  ")
			if strings.HasPrefix(strings.TrimLeft(l, " "), "- ") {
				entry = append(entry, l) // a (possibly nested) bullet
			} else { // a wrapped line: join it to the summary or bullet above
				entry[len(entry)-1] += " " + strings.TrimSpace(l)
			}
		}
		return entry, true
	}
	return nil, false
}

func notes(v string, entry []string) string {
	// "(first release of this fork), with …" becomes "First release of this fork, with …"
	summary := entry[0]
	if strings.HasPrefix(summary, "(") {
		summary = strings.Replace(summary[1:], ")", "", 1)
	}
	if summary != "" {
		summary = strings.ToUpper(summary[:1]) + summary[1:]
	}
	var b strings.Builder
	if summary != "" {
		fmt.Fprintf(&b, "**%s**\n\n", summary)
	}
	for _, l := range entry[1:] {
		b.WriteString(l + "\n")
	}
	fmt.Fprintf(&b, `
## Downloads

| System | File |
|---|---|
| Windows 10/11 (64-bit) | `+"`deej.exe`"+` |
| Linux, x86-64 | `+"`deej-linux-amd64`"+` |
| Linux, 64-bit ARM (e.g. Raspberry Pi 4/5) | `+"`deej-linux-arm64`"+` |
| Example configuration | `+"`config.yaml`"+` |

Each program is a single file; nothing to install. Set `+"`com_port`"+` and `+"`slider_mapping`"+` in `+"`config.yaml`"+`, and start deej.

- **Windows:** put `+"`deej.exe`"+` and `+"`config.yaml`"+` in one folder somewhere permanent (e.g.`+"`%%LOCALAPPDATA%%\\Programs\\deej\\`"+`). deej isn't code-signed, so SmartScreen may warn the first time: **More info → Run anyway**. With Smart App Control switched on, Windows blocks unsigned apps completely.
- **Linux:** `+"`install -Dm755 deej-linux-amd64 ~/.local/bin/deej`"+` and `+"`install -Dm644 config.yaml ~/.config/deej/config.yaml`"+` (the log goes to `+"`~/.local/state/deej/`"+`), add yourself to the `+"`dialout`"+` group for the serial port, and use Linux app names in the config (`+"`spotify`"+`, not `+"`spotify.exe`"+`). Needs PulseAudio or PipeWire; plain GNOME needs the *AppIndicator and KStatusNotifierItem Support* extension for the tray icon.
- **Arduino:** flash one of the sketches in [`+"`arduino/`"+`](https://github.com/Advve/deej/tree/v%[1]s/arduino): `+"`deej-5-sliders-vanilla`"+` for any board, or `+"`deej-5-sliders-oled`"+` for a Nano with an OLED screen.

Verify a download with `+"`SHA256SUMS.txt`"+`: `+"`sha256sum -c SHA256SUMS.txt --ignore-missing`"+` (Linux) or `+"`Get-FileHash <file>`"+` (PowerShell). `+"`THIRD-PARTY-NOTICES.txt`"+` lists the licenses of the libraries inside the app.

---
deej %[1]s is a fork of [omriharel/deej](https://github.com/omriharel/deej), created by Omri Harel. The original is no longer maintained; this fork is not affiliated with it.
`, v)
	return b.String()
}
