package tui

import (
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IsInteractive reports whether the CLI is attached to a terminal on both
// ends. Wizards require a TTY; scripted runs fall back to flags.
func IsInteractive() bool {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	out, err := os.Stdout.Stat()
	if err != nil || out.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	return true
}

// newTextInput builds a text input with the shared prompt style.
func newTextInput(placeholder string) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.PromptStyle = FieldLabelStyle
	input.TextStyle = ValueStyle
	input.CharLimit = 256
	return input
}

// newSpinner builds the shared dots spinner.
func newSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(accentColor))
	return s
}

// newProgress builds the shared progress bar with a sky-blue to emerald
// gradient, sized for typical terminals.
func newProgress() progress.Model {
	return progress.New(progress.WithGradient("#38BDF8", "#34D399"))
}

// commonKeys is the key map shared by every wizard screen.
type commonKeys struct {
	Back  key.Binding
	Quit  key.Binding
	Force key.Binding
}

func newCommonKeys() commonKeys {
	return commonKeys{
		Back:  key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Quit:  key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
		Force: key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "force")),
	}
}

// helpModel is embedded by wizards that render the bubbles help component.
type helpModel struct {
	help help.Model
}

func newHelp() helpModel {
	return helpModel{help: help.New()}
}

// newListDelegate builds the shared list item delegate. The bubbles default
// styles hardcode a purple selected-item palette, so both colors are
// overridden with the theme accent.
func newListDelegate() list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		BorderForeground(lipgloss.Color(accentColor)).
		Foreground(lipgloss.Color(accentColor))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		BorderForeground(lipgloss.Color(accentColor)).
		Foreground(lipgloss.Color(accentColor))
	return delegate
}

// programAware is implemented by wizards that stream background progress into
// the UI and therefore need their tea.Program reference before Run starts.
type programAware interface {
	setProgram(*tea.Program)
}

// runProgram starts a wizard inline (no alternate screen), so the wizard
// shares the terminal and its output stays in the scrollback. Final views are
// rendered to stdout after the program exits.
func runProgram(model tea.Model) (tea.Model, error) {
	program := tea.NewProgram(model)
	if aware, ok := model.(programAware); ok {
		aware.setProgram(program)
	}
	final, err := program.Run()
	return final, err
}
