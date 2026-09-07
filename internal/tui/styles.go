// Package tui contains the interactive terminal UIs built with
// charmbracelet/bubbletea. Each wizard also has a non-interactive flag-driven
// path; the TUI is only launched when both stdin and stdout are terminals and
// required flags are missing.
package tui

import "github.com/charmbracelet/lipgloss"

// The theme is a single blue/cyan accent family. Terminal color "39" is
// DeepSkyBlue1; gradients pair sky blue with emerald so the progress bar
// reads as progress rather than decoration.
const accentColor = "39"

var (
	// TitleStyle renders wizard titles.
	TitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accentColor)).MarginBottom(1)

	// SubtitleStyle renders secondary titles and step hints.
	SubtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// ErrorStyle renders error messages.
	ErrorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))

	// WarningStyle renders fallback notices.
	WarningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	// SuccessStyle renders final success banners.
	SuccessStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))

	// DetailStyle renders log/detail lines under the progress bar.
	DetailStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// FieldLabelStyle renders form field labels.
	FieldLabelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

	// ValueStyle renders confirmed values in summaries.
	ValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	// PanelStyle draws the summary/confirm panel border.
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("31")).
			Padding(1, 2)
)
