package deej

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/omriharel/deej/pkg/deej/util"
)

// Display info is sent from deej to the board, so a board with a screen can show what each
// slider controls. Boards that don't read their serial input just ignore it.
//
// After every (re)connect and every config reload, on the first slider line that comes in,
// deej sends one text line per item, each ending in "\n":
//
//	>H <sliders> <invert 0|1> <os>     header, e.g. ">H 5 1 linux"
//	>S <index> <icon> <flags> <label>  one per slider, e.g. ">S 4 window 1 ACTIVE"
//	>E                                 end: the board applies (and may save) what it got
//
// flags: 0 = works, 1 = not supported on this OS, 2 = slider not mapped in config.yaml.
// icon is one of displayIcons. label is printable ASCII, at most displayLabelMaxLen chars,
// and runs to the end of the line, so it can contain spaces.

const (
	displayLabelMaxLen = 10

	displayFlagOK          = 0
	displayFlagUnsupported = 1
	displayFlagUnmapped    = 2

	// the board's serial RX buffer is small (64 bytes on an ATmega328), so space the lines out
	displayLineGap = 40 * time.Millisecond
)

// icon names the board knows about
var displayIcons = []string{"speaker", "music", "globe", "headset", "window", "mic"}

// app categories, used to pick an icon (and a group label) for plain process names
var displayCategories = []struct {
	label string
	icon  string
	names []string
}{
	{"BROWSER", "globe", []string{"chrome", "google-chrome", "chromium", "msedge", "microsoft-edge", "firefox",
		"librewolf", "waterfox", "brave", "opera", "vivaldi", "zen", "floorp"}},
	{"MUSIC", "music", []string{"spotify", "itunes", "applemusic", "vlc", "foobar2000", "musicbee", "aimp", "tidal",
		"deezer", "rhythmbox", "amarok", "clementine", "strawberry", "audacious", "elisa", "cider"}},
	{"VOICE", "headset", []string{"discord", "vesktop", "webcord", "teamspeak", "ts3client_win64", "ts3client",
		"teams", "ms-teams", "zoom", "skype", "mumble", "slack"}},
}

type displaySlider struct {
	icon  string
	flags int
	label string
}

// buildDisplayLines returns the lines to send for the given number of sliders
func buildDisplayLines(config *CanonicalConfig, numSliders int) []string {
	invert := 0
	if config.InvertSliders {
		invert = 1
	}

	lines := []string{fmt.Sprintf(">H %d %d %s", numSliders, invert, runtime.GOOS)}

	for idx := 0; idx < numSliders; idx++ {
		targets, _ := config.SliderMapping.get(idx)
		s := describeSlider(targets)

		if label := sanitizeDisplayLabel(config.SliderLabels[idx]); label != "" {
			s.label = label
		}

		if s.label == "" { // e.g. a device name with no ASCII in it
			s.label = fmt.Sprintf("SLIDER %d", idx)
		}

		if icon := strings.ToLower(strings.TrimSpace(config.SliderIcons[idx])); isDisplayIcon(icon) {
			s.icon = icon
		}

		lines = append(lines, fmt.Sprintf(">S %d %s %d %s", idx, s.icon, s.flags, s.label))
	}

	return append(lines, ">E")
}

// describeSlider derives a default label, icon and support flag from a slider's targets
func describeSlider(targets []string) displaySlider {
	if len(targets) == 0 {
		return displaySlider{icon: "speaker", flags: displayFlagUnmapped, label: "UNUSED"}
	}

	result := displaySlider{flags: displayFlagUnsupported}
	for _, target := range targets {
		if displayTargetSupported(target) {
			result.flags = displayFlagOK
			break
		}
	}

	first := strings.ToLower(strings.TrimSpace(targets[0]))
	result.label, result.icon = describeTarget(first)

	// a group: name it after its category if all of its apps share one, otherwise "FIRST+"
	if len(targets) > 1 {
		category := displayCategoryOf(first)
		for _, target := range targets[1:] {
			if displayCategoryOf(strings.ToLower(strings.TrimSpace(target))) != category {
				category = -1
				break
			}
		}

		if category >= 0 {
			result.label = displayCategories[category].label
		} else {
			result.label = truncateDisplayLabel(result.label, displayLabelMaxLen-1) + "+"
		}
	}

	return result
}

// describeTarget returns the default label and icon of a single (lowercase) target
func describeTarget(target string) (string, string) {
	switch target {
	case masterSessionName:
		return "MASTER", "speaker"
	case systemSessionName:
		return "SYSTEM", "speaker"
	case inputSessionName:
		return "MIC", "mic"
	case specialTargetTransformPrefix + specialTargetCurrentWindow:
		return "ACTIVE", "window"
	case specialTargetTransformPrefix + specialTargetAllUnmapped:
		return "OTHER", "speaker"
	}

	// a device name, e.g. "Speakers (Realtek High Definition Audio)" -> "SPEAKERS"
	if deviceSessionKeyPattern.MatchString(target) {
		name := target[:strings.LastIndex(target, " (")]
		icon := "speaker"
		if strings.Contains(target, "mic") {
			icon = "mic"
		}

		return sanitizeDisplayLabel(name), icon
	}

	icon := "window"
	if category := displayCategoryOf(target); category >= 0 {
		icon = displayCategories[category].icon
	}

	return sanitizeDisplayLabel(strings.TrimSuffix(target, ".exe")), icon
}

// displayTargetSupported reports whether deej can control this target on the current OS
func displayTargetSupported(target string) bool {
	target = strings.ToLower(strings.TrimSpace(target))

	if strings.HasPrefix(target, specialTargetTransformPrefix) {
		switch strings.TrimPrefix(target, specialTargetTransformPrefix) {
		case specialTargetAllUnmapped:
			return true
		case specialTargetCurrentWindow:
			return !util.Linux()
		default:
			return false // unknown special target, not supported anywhere
		}
	}

	if util.Linux() {
		// these only exist in the Windows session finder
		if target == systemSessionName || deviceSessionKeyPattern.MatchString(target) {
			return false
		}
	}

	return true
}

func displayCategoryOf(target string) int {
	name := strings.TrimSuffix(target, ".exe")
	for idx, category := range displayCategories {
		for _, candidate := range category.names {
			if name == candidate {
				return idx
			}
		}
	}

	return -1
}

func isDisplayIcon(name string) bool {
	for _, icon := range displayIcons {
		if name == icon {
			return true
		}
	}

	return false
}

// sanitizeDisplayLabel keeps printable ASCII, uppercases it and limits its length
func sanitizeDisplayLabel(label string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(label) {
		if r >= 0x20 && r <= 0x7e {
			b.WriteRune(r)
		}
	}

	return truncateDisplayLabel(strings.TrimSpace(b.String()), displayLabelMaxLen)
}

func truncateDisplayLabel(label string, max int) string {
	if len(label) > max {
		label = strings.TrimSpace(label[:max])
	}

	return label
}

// intKeyedStringMap turns viper's string-keyed map ("0": "MASTER") into a slider-indexed one
func intKeyedStringMap(m map[string]string) map[int]string {
	result := make(map[int]string, len(m))
	for key, value := range m {
		if idx, err := strconv.Atoi(key); err == nil {
			result[idx] = value
		}
	}

	return result
}

// sendDisplayInfo writes the display lines to the board. It runs on its own goroutine so the
// read loop never waits on it
func (sio *SerialIO) sendDisplayInfo(logger *zap.SugaredLogger, numSliders int) {
	lines := buildDisplayLines(sio.deej.config, numSliders)

	sio.writeLock.Lock()
	defer sio.writeLock.Unlock()

	if sio.conn == nil {
		return
	}

	for _, line := range lines {
		if _, err := sio.conn.Write([]byte(line + "\n")); err != nil {
			logger.Warnw("Failed to send display info", "error", err)
			return
		}

		time.Sleep(displayLineGap)
	}

	logger.Infow("Sent display info", "lines", strconv.Quote(strings.Join(lines, " / ")))
}
