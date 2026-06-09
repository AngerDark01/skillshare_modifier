package theme

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Primary returns the style for main text such as skill names and headings.
func Primary() lipgloss.Style { return styleFg(Get().palette.Primary) }

// Muted returns the style for secondary text such as paths or metadata.
func Muted() lipgloss.Style { return styleFg(Get().palette.Muted) }

// Dim returns a terminal-agnostic "faint" style using the SGR dim flag.
// Unlike Muted, this does not use a specific foreground color, so it
// renders sensibly on any background without consulting the palette.
func Dim() lipgloss.Style {
	if Get().NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Faint(true)
}

// Accent returns the style for interactive highlights (filter match, cursor).
func Accent() lipgloss.Style { return styleFg(Get().palette.Accent) }

// Success returns the style for success messages.
func Success() lipgloss.Style { return styleFg(Get().palette.Success) }

// Warning returns the style for warning messages.
func Warning() lipgloss.Style { return styleFg(Get().palette.Warning) }

// Danger returns the style for error/danger messages.
func Danger() lipgloss.Style { return styleFg(Get().palette.Danger) }

// Info returns the style for info messages.
func Info() lipgloss.Style { return styleFg(Get().palette.Info) }

// Title returns a bold variant of the Accent style, for section headings.
func Title() lipgloss.Style {
	t := Get()
	if t.NoColor {
		return lipgloss.NewStyle().Bold(true)
	}
	return lipgloss.NewStyle().Bold(true).Foreground(t.palette.Accent)
}

// SelectedRow returns the combined fg+bg+bold style for a selected list row.
func SelectedRow() lipgloss.Style {
	t := Get()
	if t.NoColor {
		return lipgloss.NewStyle().Bold(true)
	}
	return lipgloss.NewStyle().
		Foreground(t.palette.Selected).
		Background(t.palette.SelectedBg).
		Bold(true)
}

// Badge returns the combined fg+bg style for a badge with built-in padding.
func Badge() lipgloss.Style {
	t := Get()
	if t.NoColor {
		return lipgloss.NewStyle().Padding(0, 1)
	}
	return lipgloss.NewStyle().
		Foreground(t.palette.BadgeFg).
		Background(t.palette.BadgeBg).
		Padding(0, 1)
}

// Severity returns the style for an audit severity level. Accepted levels
// (case-insensitive): "critical", "high", "medium", "low", "info". Unknown
// values return the Dim style.
func Severity(level string) lipgloss.Style {
	t := Get()
	var c lipgloss.Color
	switch strings.ToUpper(level) {
	case "CRITICAL":
		c = t.palette.SeverityCritical
		if t.NoColor {
			return lipgloss.NewStyle().Bold(true)
		}
		return lipgloss.NewStyle().Foreground(c).Bold(true)
	case "HIGH":
		c = t.palette.SeverityHigh
	case "MEDIUM":
		c = t.palette.SeverityMedium
	case "LOW":
		c = t.palette.SeverityLow
	case "INFO":
		c = t.palette.SeverityInfo
	default:
		return Dim()
	}
	return styleFg(c)
}

// SeverityStyle is an alias for Severity, kept for migration convenience
// from the legacy tcSevStyle function.
func SeverityStyle(level string) lipgloss.Style { return Severity(level) }

// RiskLabelStyle returns the style for a risk label: clean|low|medium|high|critical.
// Used by formatRiskBadgeLipgloss in the legacy tui_colors.go.
func RiskLabelStyle(label string) lipgloss.Style {
	switch strings.ToLower(label) {
	case "clean":
		return Success()
	case "low":
		return Severity("low")
	case "medium":
		return Severity("medium")
	case "high":
		return Severity("high")
	case "critical":
		return Severity("critical")
	default:
		return Dim()
	}
}

// FormatRiskBadge returns a colored risk badge string like " [high]".
// Migrated from formatRiskBadgeLipgloss in legacy tui_colors.go.
func FormatRiskBadge(label string) string {
	if label == "" {
		return ""
	}
	return " " + RiskLabelStyle(label).Render("["+label+"]")
}

// styleFg is an internal helper that honors NoColor and returns a style
// with only a foreground color set.
func styleFg(c lipgloss.Color) lipgloss.Style {
	if Get().NoColor {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(c)
}
