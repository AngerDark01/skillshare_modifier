package theme

import "github.com/charmbracelet/lipgloss"

// palette holds every color token used across skillshare's TUIs and CLI.
// Two instances exist: darkPalette and lightPalette. The active one is
// stored on Theme and accessed via exported style constructors.
type palette struct {
	// Text hierarchy
	Primary lipgloss.Color // main text (skill names, headings)
	Muted   lipgloss.Color // secondary text (paths, metadata)
	Accent  lipgloss.Color // interactive highlights (filter match, cursor)

	// Selected list row
	Selected   lipgloss.Color
	SelectedBg lipgloss.Color

	// Status colors
	Success lipgloss.Color
	Warning lipgloss.Color
	Danger  lipgloss.Color
	Info    lipgloss.Color

	// Badge
	BadgeFg lipgloss.Color
	BadgeBg lipgloss.Color

	// Severity family (audit results)
	SeverityCritical lipgloss.Color
	SeverityHigh     lipgloss.Color
	SeverityMedium   lipgloss.Color
	SeverityLow      lipgloss.Color
	SeverityInfo     lipgloss.Color
}
