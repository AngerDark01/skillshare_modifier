package theme

import (
	"math"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// contrastRatio returns the WCAG contrast ratio between fg and bg.
// Both colors may be 256-palette indices (e.g. "252") or hex strings
// (e.g. "#FFFFFF"). Returns 1.0 on parse failure, which is the minimum
// possible ratio (identical colors).
func contrastRatio(fg, bg lipgloss.Color) float64 {
	fgLum, ok1 := relativeLuminance(string(fg))
	bgLum, ok2 := relativeLuminance(string(bg))
	if !ok1 || !ok2 {
		return 1.0
	}
	lighter, darker := fgLum, bgLum
	if darker > lighter {
		lighter, darker = darker, lighter
	}
	return (lighter + 0.05) / (darker + 0.05)
}

// relativeLuminance computes the WCAG relative luminance (0..1) of a color
// given either as a 256-palette index string or a hex string.
func relativeLuminance(s string) (float64, bool) {
	r, g, b, ok := parseColorRGB(s)
	if !ok {
		return 0, false
	}
	// WCAG formula: https://www.w3.org/WAI/GL/wiki/Relative_luminance
	return 0.2126*channelLum(r) + 0.7152*channelLum(g) + 0.0722*channelLum(b), true
}

// channelLum converts an 8-bit channel value to a WCAG linearized component.
func channelLum(c int) float64 {
	cs := float64(c) / 255.0
	if cs <= 0.03928 {
		return cs / 12.92
	}
	return math.Pow((cs+0.055)/1.055, 2.4)
}

// parseColorRGB converts either "#RRGGBB" or a 256-palette index like "232"
// into 8-bit RGB channels.
func parseColorRGB(s string) (r, g, b int, ok bool) {
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		rv, err := strconv.ParseInt(s[1:3], 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		gv, err := strconv.ParseInt(s[3:5], 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		bv, err := strconv.ParseInt(s[5:7], 16, 32)
		if err != nil {
			return 0, 0, 0, false
		}
		return int(rv), int(gv), int(bv), true
	}
	// 256-palette index
	idx, err := strconv.Atoi(s)
	if err != nil || idx < 0 || idx > 255 {
		return 0, 0, 0, false
	}
	return palette256ToRGB(idx)
}

// palette256ToRGB maps an xterm 256-color index to approximate 8-bit RGB.
// Ranges:
//
//	  0-  7: standard ANSI (hardcoded)
//	  8- 15: bright ANSI   (hardcoded)
//	 16-231: 6x6x6 color cube
//	232-255: 24-step grayscale
func palette256ToRGB(idx int) (r, g, b int, ok bool) {
	standard := [16][3]int{
		{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
		{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
		{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
		{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
	}
	if idx < 16 {
		c := standard[idx]
		return c[0], c[1], c[2], true
	}
	if idx < 232 {
		// 6x6x6 cube: colors at 0, 95, 135, 175, 215, 255
		levels := [6]int{0, 95, 135, 175, 215, 255}
		n := idx - 16
		r = levels[(n/36)%6]
		g = levels[(n/6)%6]
		b = levels[n%6]
		return r, g, b, true
	}
	// Grayscale: 232..255 → 8, 18, 28, ..., 238
	v := 8 + (idx-232)*10
	return v, v, v, true
}
