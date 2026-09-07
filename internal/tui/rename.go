package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/go-sphere/sphere-cli/internal/renamer"
)

type renameStep int

const (
	renameStepTarget renameStep = iota
	renameStepModule
	renameStepConfirm
	renameStepRunning
	renameStepDone
	renameStepError
)

// renameFinishedMsg terminates the rename pipeline.
type renameFinishedMsg struct {
	err error
}

// RenameWizardOptions prefills the wizard from CLI flags.
type RenameWizardOptions struct {
	Old    string
	New    string
	Target string
}

type RenameWizard struct {
	step   renameStep
	width  int
	height int

	targetInput textinput.Model
	oldInput    textinput.Model
	newInput    textinput.Model
	targetErr   string
	moduleErr   string
	// oldFocused routes module-step keystrokes to the current-module input.
	oldFocused bool
	// detected is true when the old module was read from go.mod instead of
	// typed by the user.
	detected   bool
	detectNote string

	spinner    spinner.Model
	progress   progress.Model
	errMessage string

	keys    commonKeys
	help    helpModel
	program *tea.Program
}

// RunRenameWizard launches the interactive module rename flow.
func RunRenameWizard(opts RenameWizardOptions) error {
	wizard := newRenameWizard(opts)
	final, err := runProgram(wizard)
	if err != nil {
		return err
	}
	model, ok := final.(*RenameWizard)
	if !ok {
		return nil
	}
	fmt.Println()
	if model.step == renameStepDone {
		fmt.Println(SuccessStyle.Render(fmt.Sprintf("✔ Renamed %s -> %s in %s",
			model.oldInput.Value(), model.newInput.Value(), model.targetInput.Value())))
		return nil
	}
	if model.step == renameStepError && model.errMessage != "" {
		return fmt.Errorf("%s", model.errMessage)
	}
	return nil
}

func newRenameWizard(opts RenameWizardOptions) *RenameWizard {
	// The target stays empty so typed input replaces it wholesale; pressing
	// enter accepts "." as the default.
	targetInput := newTextInput(". (enter accepts this default)")
	if opts.Target != "" {
		targetInput.SetValue(opts.Target)
	}
	oldInput := newTextInput("github.com/old/module")
	newInput := newTextInput("github.com/new/module")
	if opts.Old != "" {
		oldInput.SetValue(opts.Old)
	}
	if opts.New != "" {
		newInput.SetValue(opts.New)
	}

	wizard := &RenameWizard{
		step:        renameStepTarget,
		targetInput: targetInput,
		oldInput:    oldInput,
		newInput:    newInput,
		spinner:     newSpinner(),
		progress:    newProgress(),
		keys:        newCommonKeys(),
		help:        newHelp(),
	}

	// Pre-detect the current module so the module step shows it prefilled.
	if opts.Old == "" {
		if module, err := renamer.ModulePath(targetInput.Value()); err == nil {
			wizard.oldInput.SetValue(module)
			wizard.detected = true
		}
	} else {
		wizard.step = renameStepModule
		if module, err := renamer.ModulePath(targetInput.Value()); err == nil {
			wizard.detectNote = fmt.Sprintf("go.mod currently declares %q", module)
		}
		wizard.newInput.Focus()
	}
	if wizard.step == renameStepTarget {
		wizard.targetInput.Focus()
	}
	return wizard
}

func (m *RenameWizard) setProgram(program *tea.Program) { m.program = program }

func (m *RenameWizard) Init() tea.Cmd {
	if m.step == renameStepModule {
		return tea.Batch(textinput.Blink, m.spinner.Tick)
	}
	return m.spinner.Tick
}

func (m *RenameWizard) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.help.Width = msg.Width
		return m, nil

	case renameFinishedMsg:
		if msg.err != nil {
			m.errMessage = msg.err.Error()
			m.step = renameStepError
			return m, nil
		}
		m.step = renameStepDone
		return m, tea.Quit
	}

	if keyMessage, ok := message.(tea.KeyMsg); ok && key.Matches(keyMessage, m.keys.Quit) {
		return m, tea.Quit
	}

	switch m.step {
	case renameStepTarget:
		return m.updateTargetStep(message)
	case renameStepModule:
		return m.updateModuleStep(message)
	case renameStepConfirm:
		return m.updateConfirmStep(message)
	case renameStepRunning, renameStepDone, renameStepError:
		return m, nil
	}
	return m, nil
}

func (m *RenameWizard) updateTargetStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			return m, tea.Quit
		case keyMessage.Type == tea.KeyEnter:
			return m.submitTarget()
		}
	}
	newInput, cmd := m.targetInput.Update(message)
	m.targetInput = newInput
	return m, cmd
}

func (m *RenameWizard) submitTarget() (tea.Model, tea.Cmd) {
	target := strings.TrimSpace(m.targetInput.Value())
	m.targetErr = ""
	if target == "" {
		target = "."
		m.targetInput.SetValue(".")
	}
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		m.targetErr = fmt.Sprintf("%q is not a directory", target)
		return m, nil
	}
	if _, err := os.Stat(filepath.Join(target, "go.mod")); err != nil {
		m.targetErr = fmt.Sprintf("no go.mod found in %q", target)
		return m, nil
	}
	m.targetInput.SetValue(target)
	if module, err := renamer.ModulePath(target); err == nil && m.oldInput.Value() == "" {
		m.oldInput.SetValue(module)
		m.detected = true
	}
	m.step = renameStepModule
	if m.oldInput.Value() == "" {
		m.oldFocused = true
		_ = m.oldInput.Focus()
		return m, textinput.Blink
	}
	m.oldFocused = false
	_ = m.newInput.Focus()
	return m, textinput.Blink
}

func (m *RenameWizard) updateModuleStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case keyMessage.Type == tea.KeyTab:
			m.oldFocused = !m.oldFocused
			m.oldInput.Blur()
			m.newInput.Blur()
			if m.oldFocused {
				m.oldInput.Focus()
			} else {
				m.newInput.Focus()
			}
			return m, textinput.Blink
		case key.Matches(keyMessage, m.keys.Back):
			m.step = renameStepTarget
			m.oldInput.Blur()
			m.newInput.Blur()
			m.targetInput.Focus()
			return m, nil
		case keyMessage.Type == tea.KeyEnter:
			return m.submitModule()
		}
	}
	var cmd tea.Cmd
	var old, new textinput.Model
	if m.oldFocused {
		old, cmd = m.oldInput.Update(message)
		m.oldInput = old
	} else {
		new, cmd = m.newInput.Update(message)
		m.newInput = new
	}
	return m, cmd
}

func (m *RenameWizard) submitModule() (tea.Model, tea.Cmd) {
	oldModule := strings.TrimSpace(m.oldInput.Value())
	newModule := strings.TrimSpace(m.newInput.Value())
	m.moduleErr = ""
	if oldModule == "" || newModule == "" {
		m.moduleErr = "old and new module paths are required"
		return m, nil
	}
	if oldModule == newModule {
		m.moduleErr = "old and new module paths must differ"
		return m, nil
	}
	m.oldInput.SetValue(oldModule)
	m.newInput.SetValue(newModule)
	m.step = renameStepConfirm
	return m, nil
}

func (m *RenameWizard) updateConfirmStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case keyMessage.Type == tea.KeyEnter:
		oldModule := m.oldInput.Value()
		newModule := m.newInput.Value()
		target := m.targetInput.Value()
		m.step = renameStepRunning
		return m, func() tea.Msg {
			err := renamer.RenameProjectModule(oldModule, newModule, target, []string{
				"buf.gen.yaml",
				"buf.binding.yaml",
			}, true)
			return renameFinishedMsg{err: err}
		}
	case key.Matches(keyMessage, m.keys.Back):
		m.step = renameStepModule
		if m.oldFocused {
			m.oldInput.Focus()
		} else {
			m.newInput.Focus()
		}
		return m, textinput.Blink
	}
	return m, nil
}

func (m *RenameWizard) View() string {
	var body string
	switch m.step {
	case renameStepTarget:
		body = m.viewTarget()
	case renameStepModule:
		body = m.viewModule()
	case renameStepConfirm:
		body = m.viewConfirm()
	case renameStepRunning:
		body = m.viewRunning()
	case renameStepDone:
		body = m.viewDone()
	case renameStepError:
		body = m.viewError()
	}
	return body
}

func (m *RenameWizard) viewTarget() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render("Rename Go module") + "\n\n")
	builder.WriteString(FieldLabelStyle.Render("  Target directory:") + "\n  ")
	builder.WriteString(m.targetInput.View())
	builder.WriteString("\n")
	if m.targetErr != "" {
		builder.WriteString("\n  " + ErrorStyle.Render("✘ "+m.targetErr) + "\n")
	}
	builder.WriteString("\n" + SubtitleStyle.Render("  enter continue · esc quit"))
	return builder.String()
}

func (m *RenameWizard) viewModule() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render("Rename Go module") + "\n\n")
	builder.WriteString(FieldLabelStyle.Render("  Current module:") + "\n  ")
	builder.WriteString(m.oldInput.View())
	if m.detected {
		builder.WriteString(SubtitleStyle.Render("   (read from go.mod)"))
	}
	if m.detectNote != "" {
		builder.WriteString("\n  " + DetailStyle.Render(m.detectNote))
	}
	builder.WriteString("\n\n")
	builder.WriteString(FieldLabelStyle.Render("  New module:") + "\n  ")
	builder.WriteString(m.newInput.View())
	builder.WriteString("\n")
	if m.moduleErr != "" {
		builder.WriteString("\n  " + ErrorStyle.Render("✘ "+m.moduleErr) + "\n")
	}
	builder.WriteString("\n" + SubtitleStyle.Render("  enter continue · esc back"))
	return builder.String()
}

func (m *RenameWizard) viewConfirm() string {
	panel := PanelStyle.Render(strings.Join([]string{
		FieldLabelStyle.Render("Target ") + ValueStyle.Render(m.targetInput.Value()),
		FieldLabelStyle.Render("Old    ") + ValueStyle.Render(m.oldInput.Value()),
		FieldLabelStyle.Render("New    ") + ValueStyle.Render(m.newInput.Value()),
	}, "\n"))
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render("Rename Go module") + "\n\n")
	builder.WriteString("  " + panel + "\n\n")
	builder.WriteString(WarningStyle.Render("  This rewrites imports across the project.") + "\n\n")
	builder.WriteString(SubtitleStyle.Render("  enter start rename · esc back"))
	return builder.String()
}

func (m *RenameWizard) viewRunning() string {
	return TitleStyle.Render("Rename Go module") + "\n\n" +
		"  " + m.spinner.View() + " " + SubtitleStyle.Render("Rewriting module references…") + "\n"
}

func (m *RenameWizard) viewDone() string {
	return TitleStyle.Render("Rename Go module") + "\n\n" +
		"  " + SuccessStyle.Render("✔ Module renamed") + "\n"
}

func (m *RenameWizard) viewError() string {
	return TitleStyle.Render("Rename Go module") + "\n\n" +
		"  " + ErrorStyle.Render("✘ Rename failed") + "\n\n" +
		"  " + DetailStyle.Render(truncate(m.errMessage, max(m.width-6, 20))) + "\n"
}
