package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/go-sphere/sphere-cli/internal/create"
)

type createStep int

const (
	createStepTemplate createStep = iota
	createStepName
	createStepModule
	createStepOptions
	createStepConfirm
	createStepRunning
	createStepDone
	createStepError
)

const (
	createTotalFormSteps = "4"
	optionGitIndex       = 0
	optionDepsIndex      = 1
)

// layoutsLoadedMsg carries the remote (or built-in) template catalog.
type layoutsLoadedMsg struct {
	layouts  []*create.LayoutItem
	fallback bool
}

// progressMsg streams pipeline progress into the running screen.
type progressMsg struct {
	event create.ProgressEvent
}

// createFinishedMsg terminates the pipeline.
type createFinishedMsg struct {
	err error
}

// layoutListItem adapts create.LayoutItem to the bubbles list component.
type layoutListItem struct {
	item *create.LayoutItem
}

func (l layoutListItem) Title() string       { return l.item.Name }
func (l layoutListItem) Description() string { return l.item.Description }
func (l layoutListItem) FilterValue() string { return l.item.Name }

type toggleOption struct {
	label   string
	detail  string
	enabled bool
}

// CreateWizardOptions prefills the wizard from CLI flags.
type CreateWizardOptions struct {
	Name   string
	Module string
	Layout string
	NoGit  bool
	NoDeps bool
}

type CreateWizard struct {
	width, height int
	step          createStep

	// Template catalog.
	spinner  spinner.Model
	loading  bool
	fallback bool
	list     list.Model
	layouts  []*create.LayoutItem
	// chosenLayoutName is a catalog name; customLayoutURI is a full JSON
	// layout URI passed via --layout, which skips the picker entirely.
	chosenLayoutName string
	customLayoutURI  string

	nameInput     textinput.Model
	moduleInput   textinput.Model
	moduleTouched bool
	nameErr       string
	moduleErr     string

	options        []toggleOption
	selectedOption int

	progress  progress.Model
	event     create.ProgressEvent
	recent    []string
	createCmd tea.Cmd

	errMessage string
	targetDir  string

	keys    commonKeys
	help    helpModel
	program *tea.Program
}

// RunCreateWizard launches the interactive project creation flow. It returns
// an error only when the UI itself fails; pipeline failures surface on the
// wizard's error screen and are re-reported by the final summary.
func RunCreateWizard(opts CreateWizardOptions) error {
	wizard := newCreateWizard(opts)
	final, err := runProgram(wizard)
	if err != nil {
		return err
	}
	model, ok := final.(*CreateWizard)
	if !ok {
		return nil
	}
	fmt.Println()
	if model.step == createStepDone {
		fmt.Println(SuccessStyle.Render("✔ Project created: " + model.targetDir))
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  cd " + model.nameInput.Value())
		fmt.Println("  make help   # list available targets")
		return nil
	}
	if model.step == createStepError && model.errMessage != "" {
		return fmt.Errorf("%s", model.errMessage)
	}
	return nil
}

func newCreateWizard(opts CreateWizardOptions) *CreateWizard {
	delegate := newListDelegate()

	templateList := list.New(nil, delegate, 0, 0)
	templateList.Title = "Select a project template"
	templateList.SetShowStatusBar(false)
	templateList.SetFilteringEnabled(true)
	templateList.Styles.Title = TitleStyle

	nameInput := newTextInput("my-project")
	nameInput.Focus()
	moduleInput := newTextInput("github.com/yourorg/my-project (default: project name)")

	wizard := &CreateWizard{
		step:        createStepTemplate,
		spinner:     newSpinner(),
		loading:     true,
		list:        templateList,
		nameInput:   nameInput,
		moduleInput: moduleInput,
		options: []toggleOption{
			{label: "Initialize git repository", detail: "git init + initial commit", enabled: true},
			{label: "Install dependencies", detail: "make init + go mod tidy", enabled: true},
		},
		progress: newProgress(),
		keys:     newCommonKeys(),
		help:     newHelp(),
	}

	if opts.Name != "" {
		wizard.nameInput.SetValue(opts.Name)
	}
	if opts.Module != "" {
		wizard.moduleInput.SetValue(opts.Module)
		wizard.moduleTouched = true
	}
	if opts.Layout != "" && strings.Contains(opts.Layout, "://") {
		wizard.customLayoutURI = opts.Layout
		wizard.step = createStepName
	} else if opts.Layout != "" {
		wizard.chosenLayoutName = opts.Layout
	}
	if opts.NoGit {
		wizard.options[optionGitIndex].enabled = false
	}
	if opts.NoDeps {
		wizard.options[optionDepsIndex].enabled = false
	}
	return wizard
}

func (m *CreateWizard) setProgram(program *tea.Program) { m.program = program }

func (m *CreateWizard) Init() tea.Cmd {
	cmds := []tea.Cmd{m.spinner.Tick, loadLayoutsCmd}
	if m.step == createStepName {
		cmds = append(cmds, textinput.Blink)
	}
	return tea.Batch(cmds...)
}

func loadLayoutsCmd() tea.Msg {
	layouts, fallback, _ := create.LayoutListWithFallback()
	return layoutsLoadedMsg{layouts: layouts, fallback: fallback}
}

func (m *CreateWizard) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.help.Width = msg.Width
		m.progress.Width = min(msg.Width-8, 56)
		listHeight := min(12, max(6, msg.Height-10))
		m.list.SetSize(msg.Width-4, listHeight)
		return m, nil

	case spinner.TickMsg:
		// Keep the spinner animating: it backs both the catalog loader and the
		// indeterminate phases of the pipeline.
		newSpinner, cmd := m.spinner.Update(msg)
		m.spinner = newSpinner
		return m, cmd

	case layoutsLoadedMsg:
		return m.applyLayouts(msg)

	case progress.FrameMsg:
		newProgress, cmd := m.progress.Update(msg)
		m.progress = newProgress.(progress.Model)
		return m, cmd

	case progressMsg:
		return m.applyProgress(msg.event)

	case createFinishedMsg:
		if msg.err != nil {
			m.errMessage = msg.err.Error()
			m.step = createStepError
			return m, nil
		}
		m.step = createStepDone
		return m, tea.Quit
	}
	return m.updateByStep(message)
}

func (m *CreateWizard) applyLayouts(msg layoutsLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	m.layouts = msg.layouts
	m.fallback = msg.fallback
	items := make([]list.Item, 0, len(msg.layouts))
	selected := 0
	for i, item := range msg.layouts {
		items = append(items, layoutListItem{item: item})
		if item.Name == m.chosenLayoutName {
			selected = i
		}
	}
	if len(items) == 0 {
		m.step = createStepName
		return m, textinput.Blink
	}
	cmd := m.list.SetItems(items)
	m.list.Select(selected)
	m.chosenLayoutName = ""
	return m, cmd
}

func (m *CreateWizard) applyProgress(event create.ProgressEvent) (tea.Model, tea.Cmd) {
	m.event = event
	if event.Detail != "" {
		line := event.Detail
		if event.Label != "" && !strings.HasPrefix(event.Detail, event.Label) {
			line = event.Label + ": " + line
		}
		m.recent = append(m.recent, line)
		if len(m.recent) > 6 {
			m.recent = m.recent[len(m.recent)-6:]
		}
	}
	if event.Done {
		return m, nil
	}
	overall := float64(event.Step) / float64(event.TotalSteps)
	if event.Percent > 0 {
		overall += min(event.Percent, 1) / float64(event.TotalSteps)
	}
	return m, m.progress.SetPercent(overall)
}

func (m *CreateWizard) updateByStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok && key.Matches(keyMessage, m.keys.Quit) {
		return m, tea.Quit
	}

	switch m.step {
	case createStepTemplate:
		return m.updateTemplateStep(message)
	case createStepName:
		return m.updateNameStep(message)
	case createStepModule:
		return m.updateModuleStep(message)
	case createStepOptions:
		return m.updateOptionsStep(message)
	case createStepConfirm:
		return m.updateConfirmStep(message)
	case createStepRunning, createStepDone:
		return m, nil
	case createStepError:
		// Any key dismisses the error screen and exits the wizard.
		if _, ok := message.(tea.KeyMsg); ok {
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m *CreateWizard) updateTemplateStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		newList, cmd := m.list.Update(message)
		m.list = newList
		return m, cmd
	}
	switch {
	case key.Matches(keyMessage, m.keys.Back):
		return m, tea.Quit
	case keyMessage.Type == tea.KeyEnter:
		chosen, ok := m.list.SelectedItem().(layoutListItem)
		if !ok {
			return m, nil
		}
		m.chosenLayoutName = chosen.item.Name
		m.step = createStepName
		m.nameInput.Focus()
		return m, textinput.Blink
	}
	newList, cmd := m.list.Update(message)
	m.list = newList
	return m, cmd
}

func (m *CreateWizard) updateNameStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			if m.customLayoutURI != "" {
				return m, tea.Quit
			}
			m.step = createStepTemplate
			return m, nil
		case keyMessage.Type == tea.KeyEnter:
			return m.submitName()
		}
	}
	newInput, cmd := m.nameInput.Update(message)
	m.nameInput = newInput
	return m, cmd
}

func (m *CreateWizard) submitName() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.nameInput.Value())
	m.nameErr = ""
	if err := create.ValidateProjectName(name); err != nil {
		m.nameErr = err.Error()
		return m, nil
	}
	if _, err := os.Stat(filepath.Join(".", name)); err == nil {
		m.nameErr = fmt.Sprintf("directory %q already exists in the current folder", name)
		return m, nil
	}
	m.nameInput.SetValue(name)
	// The module step shows the project name as an editable suggestion: the
	// field stays empty so typed input replaces it wholesale, and pressing
	// enter accepts the placeholder as the default. A --module prefill is
	// kept untouched.
	if !m.moduleTouched {
		m.moduleInput.SetValue("")
		m.moduleInput.Placeholder = name + "  (enter accepts this default)"
	}
	m.nameInput.Blur()
	m.moduleInput.Focus()
	m.step = createStepModule
	return m, textinput.Blink
}

// effectiveModule returns the module path the wizard will use: the typed value
// or the project name default.
func (m *CreateWizard) effectiveModule() string {
	if module := strings.TrimSpace(m.moduleInput.Value()); module != "" {
		return module
	}
	return m.nameInput.Value()
}

func (m *CreateWizard) updateModuleStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			m.step = createStepName
			m.moduleInput.Blur()
			m.nameInput.Focus()
			return m, textinput.Blink
		case keyMessage.Type == tea.KeyEnter:
			return m.submitModule()
		}
	}
	newInput, cmd := m.moduleInput.Update(message)
	m.moduleInput = newInput
	return m, cmd
}

func (m *CreateWizard) submitModule() (tea.Model, tea.Cmd) {
	module := strings.TrimSpace(m.moduleInput.Value())
	m.moduleErr = ""
	if module == "" {
		module = m.nameInput.Value()
		m.moduleInput.SetValue(module)
	}
	if strings.ContainsAny(module, " \t") || strings.Contains(module, "://") {
		m.moduleErr = "module path must not contain spaces"
		return m, nil
	}
	m.step = createStepOptions
	return m, nil
}

func (m *CreateWizard) updateOptionsStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMessage.String() {
	case "esc":
		m.step = createStepModule
		m.moduleInput.Focus()
		return m, textinput.Blink
	case "up", "k", "down", "j":
		if keyMessage.String() == "up" || keyMessage.String() == "k" {
			m.selectedOption = max(m.selectedOption-1, 0)
		} else {
			m.selectedOption = min(m.selectedOption+1, len(m.options)-1)
		}
		return m, nil
	case " ", "left", "right":
		m.options[m.selectedOption].enabled = !m.options[m.selectedOption].enabled
		return m, nil
	case "enter":
		m.step = createStepConfirm
		return m, nil
	}
	return m, nil
}

func (m *CreateWizard) updateConfirmStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case keyMessage.Type == tea.KeyEnter:
		return m.startCreate()
	case key.Matches(keyMessage, m.keys.Back):
		m.step = createStepOptions
		return m, nil
	}
	return m, nil
}

func (m *CreateWizard) startCreate() (tea.Model, tea.Cmd) {
	module := m.effectiveModule()
	targetDir, err := filepath.Abs(m.nameInput.Value())
	if err == nil {
		m.targetDir = targetDir
	}
	initGit := m.options[optionGitIndex].enabled
	initDeps := m.options[optionDepsIndex].enabled

	program := m.program
	m.step = createStepRunning
	m.createCmd = func() tea.Msg {
		layout, err := create.Layout(m.resolvedLayoutName())
		if err != nil {
			return createFinishedMsg{err: err}
		}
		err = create.Create(create.Options{
			Name:     m.nameInput.Value(),
			Module:   module,
			Layout:   layout,
			InitGit:  initGit,
			InitDeps: initDeps,
		}, func(event create.ProgressEvent) {
			if program != nil {
				program.Send(progressMsg{event: event})
			}
		})
		return createFinishedMsg{err: err}
	}
	return m, m.createCmd
}

// resolvedLayoutName returns the template the user picked in the catalog, or
// the custom --layout URI when the picker was skipped.
func (m *CreateWizard) resolvedLayoutName() string {
	if m.customLayoutURI != "" {
		return m.customLayoutURI
	}
	if item, ok := m.list.SelectedItem().(layoutListItem); ok {
		return item.item.Name
	}
	return m.chosenLayoutName
}

func (m *CreateWizard) View() string {
	var body string
	switch m.step {
	case createStepTemplate:
		body = m.viewTemplate()
	case createStepName:
		body = m.viewName()
	case createStepModule:
		body = m.viewModule()
	case createStepOptions:
		body = m.viewOptions()
	case createStepConfirm:
		body = m.viewConfirm()
	case createStepRunning:
		body = m.viewRunning()
	case createStepDone:
		body = m.viewDone()
	case createStepError:
		body = m.viewError()
	}
	return body
}

func (m *CreateWizard) header(hint string) string {
	return TitleStyle.Render("Create a new Sphere project") + SubtitleStyle.Render(hint) + "\n"
}

func (m *CreateWizard) viewTemplate() string {
	var builder strings.Builder
	builder.WriteString(m.header("  (step 1 of " + createTotalFormSteps + " — press enter to continue)"))
	builder.WriteString("\n")
	if m.loading {
		builder.WriteString("  " + m.spinner.View() + " Fetching template catalog…")
		return builder.String()
	}
	if m.fallback {
		builder.WriteString(WarningStyle.Render("  Remote template catalog unavailable, showing built-in layouts.") + "\n\n")
	}
	builder.WriteString(m.list.View())
	return builder.String()
}

func (m *CreateWizard) viewName() string {
	var builder strings.Builder
	builder.WriteString(m.header("  (step 2 of " + createTotalFormSteps + ")"))
	builder.WriteString("\n")
	builder.WriteString(FieldLabelStyle.Render("  Project name (directory):") + "\n  ")
	builder.WriteString(m.nameInput.View())
	builder.WriteString("\n")
	if m.nameErr != "" {
		builder.WriteString("\n  " + ErrorStyle.Render("✘ "+m.nameErr) + "\n")
	}
	builder.WriteString("\n" + SubtitleStyle.Render("  enter continue · esc back"))
	return builder.String()
}

func (m *CreateWizard) viewModule() string {
	var builder strings.Builder
	builder.WriteString(m.header("  (step 3 of " + createTotalFormSteps + ")"))
	builder.WriteString("\n")
	builder.WriteString(FieldLabelStyle.Render("  Go module path:") + "\n  ")
	builder.WriteString(m.moduleInput.View())
	builder.WriteString("\n")
	if m.moduleErr != "" {
		builder.WriteString("\n  " + ErrorStyle.Render("✘ "+m.moduleErr) + "\n")
	}
	builder.WriteString("\n" + SubtitleStyle.Render("  enter continue · esc back"))
	return builder.String()
}

func (m *CreateWizard) viewOptions() string {
	var builder strings.Builder
	builder.WriteString(m.header("  (step 4 of " + createTotalFormSteps + " — space toggles, enter continues)"))
	builder.WriteString("\n")
	for i, option := range m.options {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.selectedOption {
			cursor = "> "
			style = style.Bold(true).Foreground(lipgloss.Color(accentColor))
		}
		checkbox := "[ ]"
		if option.enabled {
			checkbox = "[x]"
		}
		builder.WriteString("  " + cursor + style.Render(checkbox+" "+option.label))
		builder.WriteString(SubtitleStyle.Render("   "+option.detail) + "\n")
	}
	builder.WriteString("\n" + SubtitleStyle.Render("  up/down move · space toggle · enter continue · esc back"))
	return builder.String()
}

func (m *CreateWizard) viewConfirm() string {
	git := "no"
	if m.options[optionGitIndex].enabled {
		git = "yes"
	}
	deps := "no"
	if m.options[optionDepsIndex].enabled {
		deps = "yes"
	}
	panel := PanelStyle.Render(strings.Join([]string{
		FieldLabelStyle.Render("Template ") + ValueStyle.Render(m.templateLabel()),
		FieldLabelStyle.Render("Project  ") + ValueStyle.Render(m.nameInput.Value()),
		FieldLabelStyle.Render("Module   ") + ValueStyle.Render(m.effectiveModule()),
		FieldLabelStyle.Render("Git init ") + ValueStyle.Render(git),
		FieldLabelStyle.Render("Deps     ") + ValueStyle.Render(deps),
	}, "\n"))
	var builder strings.Builder
	builder.WriteString(m.header(""))
	builder.WriteString("  " + panel + "\n\n")
	builder.WriteString(SubtitleStyle.Render("  enter start creation · esc back"))
	return builder.String()
}

func (m *CreateWizard) templateLabel() string {
	return m.resolvedLayoutName()
}

func (m *CreateWizard) viewRunning() string {
	var builder strings.Builder
	builder.WriteString(m.header(""))
	builder.WriteString("\n  " + m.progress.View() + "\n\n")
	if m.event.Label != "" {
		builder.WriteString("  " + FieldLabelStyle.Render(m.event.Label))
		if m.event.Percent >= 0 {
			builder.WriteString(SubtitleStyle.Render(fmt.Sprintf("  (%d%%)", int(min(m.event.Percent, 1)*100))))
		}
		builder.WriteString("\n")
	} else {
		builder.WriteString("  " + m.spinner.View() + " Working…\n")
	}
	builder.WriteString("\n")
	for _, line := range m.recent {
		builder.WriteString("  " + DetailStyle.Render(truncate(line, max(m.width-6, 20))) + "\n")
	}
	return builder.String()
}

func (m *CreateWizard) viewDone() string {
	return m.header("") + "\n  " + SuccessStyle.Render("✔ Project created at "+m.targetDir) + "\n"
}

func (m *CreateWizard) viewError() string {
	var builder strings.Builder
	builder.WriteString(m.header(""))
	builder.WriteString("\n  " + ErrorStyle.Render("✘ Creation failed") + "\n\n")
	builder.WriteString("  " + DetailStyle.Render(truncate(m.errMessage, max(m.width-6, 20))) + "\n")
	return builder.String()
}

func truncate(text string, width int) string {
	if width <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= width {
		return text
	}
	return string(runes[:width-1]) + "…"
}
