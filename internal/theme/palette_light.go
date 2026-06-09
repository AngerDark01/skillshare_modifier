package theme

import "github.com/charmbracelet/lipgloss"

// lightPalette is tuned for light terminal backgrounds.
//
// Key differences vs darkPalette:
//   - All foreground colors use dark shades (near black 232, dark blue 25,
//     etc.) to meet WCAG AA contrast against white backgrounds.
//   - Warning uses 130 (dark gold) instead of 3 (yellow) because yellow
//     on white has < 2:1 contrast ratio (unreadable).
//   - SeverityLow uses 25 (dark blue) instead of 12 (bright blue) for
//     the same reason.
var lightPalette = palette{
	Primary: lipgloss.Color("232"),
	Muted:   lipgloss.Color("240"),
	Accent:  lipgloss.Color("25"),

	Selected:   lipgloss.Color("232"),
	SelectedBg: lipgloss.Color("252"),

	Success: lipgloss.Color("28"),
	Warning: lipgloss.Color("130"),
	Danger:  lipgloss.Color("124"),
	Info:    lipgloss.Color("25"),

	BadgeFg: lipgloss.Color("232"),
	BadgeBg: lipgloss.Color("252"),

	SeverityCritical: lipgloss.Color("160"),
	SeverityHigh:     lipgloss.Color("208"),
	SeverityMedium:   lipgloss.Color("130"),
	SeverityLow:      lipgloss.Color("25"),
	SeverityInfo:     lipgloss.Color("240"),
}
