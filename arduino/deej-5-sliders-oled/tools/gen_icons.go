// Generates ../icons.h (XBM, bit 0 = leftmost pixel) and icons_preview.png from the ASCII art
// below. Edit the art here, not the bytes in icons.h. The Arduino IDE ignores this folder.
//
//	go run gen_icons.go            (from this folder)
//
// The order of `icons` is the Icon enum order. ICON_NAMES in deej-5-sliders-oled.ino and
// displayIcons in pkg/deej/display.go must list the same names (deej sends the lowercase name,
// e.g. "micoff"); ICON_NAMES also in the same order.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
)

type icon struct {
	name  string
	small []string // 8x8
	large []string // 16x16
}

func globe(n int) []string {
	c, r := float64(n-1)/2, float64(n-1)/2
	rows := make([]string, n)
	for y := 0; y < n; y++ {
		var b strings.Builder
		for x := 0; x < n; x++ {
			dx, dy := float64(x)-c, float64(y)-c
			d := math.Hypot(dx, dy)
			on := math.Abs(d-r) < 0.55
			if d < r {
				w := 0.42 * r * math.Sqrt(math.Max(0, 1-(dy/r)*(dy/r))) // meridian ellipse
				on = on || math.Abs(math.Abs(dx)-w) < 0.5 || math.Abs(dx) < 0.5
				on = on || math.Abs(dy) < 0.5
			}
			if on {
				b.WriteByte('#')
			} else {
				b.WriteByte('.')
			}
		}
		rows[y] = b.String()
	}
	return rows
}

// slashed draws a "\" across an icon, with a 1 px gap on its right so it reads on top of
// filled shapes
func slashed(rows []string) []string {
	out := make([]string, len(rows))
	for y, row := range rows {
		b := []byte(row)
		b[y] = '#'
		if y+1 < len(b) {
			b[y+1] = '.'
		}
		out[y] = string(b)
	}
	return out
}

var mic8 = []string{
	"...##...",
	"..####..",
	"..####..",
	"#.####.#",
	"#.####.#",
	".##..##.",
	"...##...",
	"..####..",
}

var mic16 = []string{
	"......####......",
	".....######.....",
	".....######.....",
	".....######.....",
	".....######.....",
	"..#..######..#..",
	"..#..######..#..",
	"..#..######..#..",
	"..#...####...#..",
	"...#........#...",
	"....##....##....",
	"......####......",
	".......##.......",
	".......##.......",
	"....########....",
	"................",
}

var icons = []icon{
	{name: "SPEAKER",
		small: []string{
			"...#....",
			"..##..#.",
			"####...#",
			"####.#.#",
			"####.#.#",
			"####...#",
			"..##..#.",
			"...#....",
		},
		large: []string{
			"................",
			"........#.......",
			".......##...#...",
			"......###....#..",
			".....####.....#.",
			".########.#...#.",
			".########..#..#.",
			".########..#..#.",
			".########..#..#.",
			".########..#..#.",
			".########.#...#.",
			".....####.....#.",
			"......###....#..",
			".......##...#...",
			"........#.......",
			"................",
		}},
	{name: "MUSIC",
		small: []string{
			"...#####",
			"...#####",
			"...#...#",
			"...#...#",
			"...#...#",
			".###.###",
			"###.###.",
			"........",
		},
		large: []string{
			"................",
			"......#########.",
			"......#########.",
			"......#.......#.",
			"......#.......#.",
			"......#.......#.",
			"......#.......#.",
			"......#.......#.",
			"......#.......#.",
			"......#.......#.",
			"...####....####.",
			"..#####...#####.",
			"..####....####..",
			"...##......##...",
			"................",
			"................",
		}},
	{name: "GLOBE",
		small: []string{
			"..###...",
			".#.#.#..",
			"#..#..#.",
			"#######.",
			"#..#..#.",
			".#.#.#..",
			"..###...",
			"........",
		},
		large: globe(16)},
	{name: "HEADSET",
		small: []string{
			"..####..",
			".#....#.",
			"#......#",
			"#......#",
			"##....##",
			"##....##",
			"##....##",
			"........",
		},
		large: []string{
			"................",
			".....######.....",
			"...##......##...",
			"..#..........#..",
			".#............#.",
			".#............#.",
			".#............#.",
			"###..........###",
			"####........####",
			"####........####",
			"####........####",
			"####........####",
			"###..........###",
			"..............#.",
			".........#####..",
			"................",
		}},
	{name: "WINDOW",
		small: []string{
			"########",
			"########",
			"#......#",
			"#......#",
			"#......#",
			"#......#",
			"########",
			"........",
		},
		large: []string{
			"................",
			"################",
			"##.##.##.#######",
			"################",
			"#..............#",
			"#..............#",
			"#.....#........#",
			"#.....##.......#",
			"#.....###......#",
			"#.....####.....#",
			"#.....#####....#",
			"#.....##.......#",
			"#..............#",
			"#..............#",
			"################",
			"................",
		}},
	{name: "MIC", small: mic8, large: mic16},
	{name: "MUTED",
		small: []string{
			"...#....",
			"..##....",
			"####....",
			"####.#.#",
			"####..#.",
			"####.#.#",
			"..##....",
			"...#....",
		},
		large: []string{
			"................",
			"........#.......",
			".......##.......",
			"......###.......",
			".....####.......",
			".########.#....#",
			".########..#..#.",
			".########...##..",
			".########...##..",
			".########..#..#.",
			".########.#....#",
			".....####.......",
			"......###.......",
			".......##.......",
			"........#.......",
			"................",
		}},

	// added for deej-5-sliders-oled
	{name: "GAME",
		small: []string{
			"........",
			".######.",
			"##.###.#",
			"#...#.##",
			"##.#####",
			"########",
			"##....##",
			"........",
		},
		large: []string{
			"................",
			"................",
			"...##########...",
			"..############..",
			".###.##########.",
			".###.######.###.",
			".#.....########.",
			".###.########.#.",
			".###.##########.",
			".##############.",
			"..#####..#####..",
			"..####....####..",
			"...##......##...",
			"................",
			"................",
			"................",
		}},
	{name: "CHAT",
		small: []string{
			".######.",
			"#......#",
			"#.#..#.#",
			"#......#",
			".######.",
			".##.....",
			".#......",
			"........",
		},
		large: []string{
			"................",
			"................",
			"..############..",
			".#............#.",
			"#..............#",
			"#..............#",
			"#...##.##.##...#",
			"#...##.##.##...#",
			"#..............#",
			".#............#.",
			"..############..",
			"...###..........",
			"...##...........",
			"...#............",
			"................",
			"................",
		}},
	{name: "VIDEO",
		small: []string{
			".######.",
			"#......#",
			"#..#...#",
			"#..##..#",
			"#..#...#",
			"#......#",
			".######.",
			"........",
		},
		large: []string{
			"................",
			"................",
			".##############.",
			"#..............#",
			"#.....#........#",
			"#.....##.......#",
			"#.....###......#",
			"#.....####.....#",
			"#.....####.....#",
			"#.....###......#",
			"#.....##.......#",
			"#.....#........#",
			"#..............#",
			".##############.",
			"................",
			"................",
		}},
	{name: "MICOFF", small: slashed(mic8), large: slashed(mic16)},
}

func xbm(rows []string) []byte {
	n := len(rows)
	var out []byte
	for _, r := range rows {
		if len(r) != n {
			panic(fmt.Sprintf("row %q is not %d wide", r, n))
		}
		for b := 0; b < n; b += 8 {
			var v byte
			for i := 0; i < 8; i++ {
				if r[b+i] == '#' {
					v |= 1 << i
				}
			}
			out = append(out, v)
		}
	}
	return out
}

func main() {
	var h strings.Builder
	h.WriteString("// Channel icons, XBM format (bit 0 = leftmost pixel). Generated by tools/gen_icons.go from\n")
	h.WriteString("// ASCII art; edit the generator rather than the bytes if you want new icons.\n")
	h.WriteString("#pragma once\n#include <avr/pgmspace.h>\n\nenum Icon : uint8_t {\n")
	for _, ic := range icons {
		fmt.Fprintf(&h, "  ICON_%s,\n", ic.name)
	}
	h.WriteString("  ICON_COUNT\n};\n\n")

	for _, size := range []int{8, 16} {
		fmt.Fprintf(&h, "const uint8_t ICONS_%d[ICON_COUNT][%d] PROGMEM = {\n", size, size*size/8)
		for _, ic := range icons {
			rows := ic.small
			if size == 16 {
				rows = ic.large
			}
			if len(rows) != size {
				panic(fmt.Sprintf("%s: %d rows, want %d", ic.name, len(rows), size))
			}
			var parts []string
			for _, v := range xbm(rows) {
				parts = append(parts, fmt.Sprintf("0x%02x", v))
			}
			fmt.Fprintf(&h, "  {%s},  // %s\n", strings.Join(parts, ", "), ic.name)
		}
		h.WriteString("};\n\n")
	}

	if err := os.WriteFile("../icons.h", []byte(strings.TrimSuffix(h.String(), "\n")), 0o644); err != nil {
		panic(err)
	}

	// preview: both sizes for every icon, scaled up, white on black like the OLED
	const scale, cell = 6, 16*6 + 12
	img := image.NewRGBA(image.Rect(0, 0, len(icons)*cell, 2*cell))
	for i := range img.Pix {
		img.Pix[i] = 40
		if i%4 == 3 {
			img.Pix[i] = 255
		}
	}
	for j, ic := range icons {
		for row, rows := range [][]string{ic.small, ic.large} {
			for y, r := range rows {
				for x := range r {
					c := color.RGBA{0, 0, 0, 255}
					if r[x] == '#' {
						c = color.RGBA{255, 255, 255, 255}
					}
					for dy := 0; dy < scale; dy++ {
						for dx := 0; dx < scale; dx++ {
							img.Set(j*cell+6+x*scale+dx, row*cell+6+y*scale+dy, c)
						}
					}
				}
			}
		}
	}
	f, err := os.Create("icons_preview.png")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
	fmt.Println("wrote ../icons.h and icons_preview.png")
}
