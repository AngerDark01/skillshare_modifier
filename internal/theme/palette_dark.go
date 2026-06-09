package theme

import "github.com/charmbracelet/lipgloss"

// darkPalette is tuned for dark terminal backgrounds.
//
// Notable choices vs the previous tc struct:
//   - Primary is 252 (near-white) instead of 15 (bright white). Bright
//     white disappears on light terminals in fallback scenarios and the
//     visual difference on dark terminals is negligible.
//   - BadgeFg is 252 (was 250) to match Primary.
//   - Selected is 255 (was 15) — brightest gray that still renders on
//     light terminals if fallback picks the wrong palette.
var darkPalette = palette{
	Primary: lipgloss.Color("252"),
	Muted:   lipgloss.Color("245"),
	Accent:  lipgloss.Color("6"),

	Selected:   lipgloss.Color("255"),
	SelectedBg: lipgloss.Color("237"),

	Success: lipgloss.Color("2"),
	Warning: lipgloss.Color("3"),
	Danger:  lipgloss.Color("1"),
	Info:    lipgloss.Color("6"),

	BadgeFg: lipgloss.Color("252"),
	BadgeBg: lipgloss.Color("237"),

	SeverityCritical: lipgloss.Color("1"),
	SeverityHigh:     lipgloss.Color("208"),
	SeverityMedium:   lipgloss.Color("3"),
	SeverityLow:      lipgloss.Color("12"),
	SeverityInfo:     lipgloss.Color("244"),
}
