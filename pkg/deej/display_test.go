package deej

import (
	"reflect"
	"testing"

	"github.com/omriharel/deej/pkg/deej/util"
)

func TestBuildDisplayLines(t *testing.T) {
	config := &CanonicalConfig{
		SliderMapping: sliderMapFromConfigs(map[string][]string{
			"0": {"master"},
			"1": {"spotify.exe"},
			"2": {"chrome.exe", "msedge.exe"},
			"3": {"discord.exe"},
			"4": {"deej.current"},
			"5": {"Speakers (Realtek High Definition Audio)"},
			"6": {"system"},
			"7": {"game.exe", "spotify.exe"},
		}, nil),
		InvertSliders: true,
		SliderLabels:  map[int]string{3: "  chat  ", 7: "a very long label"},
		SliderIcons:   map[int]string{3: "MIC", 7: "nonsense"},
	}

	// on Linux, deej.current, device names and system can't be driven
	na := "0"
	osName := "windows"
	if util.Linux() {
		na = "1"
		osName = "linux"
	}

	want := []string{
		">H 9 1 " + osName,
		">S 0 speaker 0 MASTER",
		">S 1 music 0 SPOTIFY",
		">S 2 globe 0 BROWSER",
		">S 3 mic 0 CHAT",
		">S 4 window " + na + " ACTIVE",
		">S 5 speaker " + na + " SPEAKERS",
		">S 6 speaker " + na + " SYSTEM",
		">S 7 window 0 A VERY LON",
		">S 8 speaker 2 UNUSED",
		">E",
	}

	if got := buildDisplayLines(config, 9); !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%q\nwant\n%q", got, want)
	}
}

func TestDescribeNewCategories(t *testing.T) {
	cases := map[string]displaySlider{
		"vlc.exe":      {icon: "video", label: "VLC"},
		"telegram.exe": {icon: "chat", label: "TELEGRAM"},
		"steam":        {icon: "game", label: "STEAM"},
	}

	for target, want := range cases {
		got := describeSlider([]string{target})
		if got.icon != want.icon || got.label != want.label {
			t.Errorf("%s: got %+v, want icon %q label %q", target, got, want.icon, want.label)
		}
	}

	for _, icon := range []string{"game", "chat", "video", "micoff", "muted"} {
		if !isDisplayIcon(icon) {
			t.Errorf("%q should be a known icon", icon)
		}
	}
}

func TestDescribeMixedGroup(t *testing.T) {
	if got := describeSlider([]string{"game.exe", "spotify.exe"}).label; got != "GAME+" {
		t.Errorf("got %q, want GAME+", got)
	}
}
