package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/go-sphere/sphere-cli/internal/renamer"
	"github.com/go-sphere/sphere-cli/internal/service"
)

// ServiceKind selects which generator the wizard drives.
type ServiceKind int

const (
	// ServiceKindProto generates the entity CRUD .proto file.
	ServiceKindProto ServiceKind = iota
	// ServiceKindGolang generates the entity service skeleton .go file.
	ServiceKindGolang
)

type serviceStep int

const (
	serviceStepEntity serviceStep = iota
	serviceStepName
	serviceStepPackage
	serviceStepModule
	serviceStepPreview
	serviceStepOutput
	serviceStepOutPath
	serviceStepOverwrite
	serviceStepDone
)

// outputChoice values picked at serviceStepOutput.
const (
	outputStdout = 0
	outputFile   = 1
)

const (
	customEntityTitle = "Enter a different name…"
)

type choiceItem struct {
	title       string
	description string
	// payload carries the entity name for schema items; empty for the custom
	// name entry.
	payload string
	custom  bool
}

func (c choiceItem) Title() string       { return c.title }
func (c choiceItem) Description() string { return c.description }
func (c choiceItem) FilterValue() string { return c.title }

// ServiceWizardOptions prefills the wizard from CLI flags and CWD detection.
type ServiceWizardOptions struct {
	Kind ServiceKind
	// CWD is the working directory used for schema detection, module
	// detection, and relative output paths.
	CWD  string
	Name string
	Pkg  string
	Mod  string
	Out  string
}

type ServiceWizard struct {
	kind  ServiceKind
	cwd   string
	step  serviceStep
	width int
	// height is only used for the preview viewport sizing.
	height int

	schemaDir  string
	entityList list.Model
	entities   []string

	nameInput    textinput.Model
	packageInput textinput.Model
	moduleInput  textinput.Model
	pathInput    textinput.Model

	preview      viewport.Model
	previewReady bool
	content      string
	genErr       string

	outputMode int
	outPath    string
	written    bool
	// printContent is echoed to stdout after the program exits.
	printContent bool

	keys    commonKeys
	help    helpModel
	program *tea.Program
}

// RunServiceWizard launches the interactive service generation flow. When the
// user picked stdout output the generated content is printed after the UI
// exits, so `sphere-cli service proto | lpr` still works with a TTY stdin.
func RunServiceWizard(opts ServiceWizardOptions) error {
	wizard := newServiceWizard(opts)
	final, err := runProgram(wizard)
	if err != nil {
		return err
	}
	model, ok := final.(*ServiceWizard)
	if !ok {
		return nil
	}
	fmt.Println()
	if model.step == serviceStepDone {
		if model.printContent {
			fmt.Print(model.content)
			return nil
		}
		fmt.Println(SuccessStyle.Render("✔ Written to " + model.outPath))
		return nil
	}
	if model.genErr != "" {
		return fmt.Errorf("%s", model.genErr)
	}
	return nil
}

func newServiceWizard(opts ServiceWizardOptions) *ServiceWizard {
	entityList := list.New(nil, newListDelegate(), 0, 0)
	entityList.Title = "Select an entity"
	entityList.SetShowStatusBar(false)
	entityList.SetFilteringEnabled(true)
	entityList.Styles.Title = TitleStyle

	packageInput := newTextInput("dash.v1")
	moduleInput := newTextInput("github.com/yourorg/yourproject")
	pathInput := newTextInput("relative/path/to/output")

	wizard := &ServiceWizard{
		kind:         opts.Kind,
		cwd:          opts.CWD,
		step:         serviceStepEntity,
		schemaDir:    service.DetectSchemaDir(opts.CWD),
		entityList:   entityList,
		nameInput:    newTextInput("key_value_store"),
		packageInput: packageInput,
		moduleInput:  moduleInput,
		pathInput:    pathInput,
		preview:      viewport.New(0, 0),
		keys:         newCommonKeys(),
		help:         newHelp(),
		outputMode:   outputStdout,
	}
	wizard.packageInput.SetValue("dash.v1")

	if mod, err := renamer.ModulePath(opts.CWD); err == nil && mod != "" {
		wizard.moduleInput.SetValue(mod)
	} else {
		wizard.moduleInput.SetValue("github.com/go-sphere/sphere-layout")
	}

	entities := service.SchemaEntityTypes(wizard.schemaDir)
	if opts.Name != "" {
		wizard.nameInput.SetValue(opts.Name)
		wizard.step = serviceStepPackage
	} else if len(entities) == 0 {
		wizard.step = serviceStepName
	} else {
		wizard.entities = entities
		items := []list.Item{
			choiceItem{title: customEntityTitle, description: "Type any service/entity name manually", custom: true},
		}
		for _, entity := range entities {
			items = append(items, choiceItem{
				title:       entity,
				description: "Ent schema type in " + schemaRelPath(wizard.schemaDir, opts.CWD),
				payload:     entity,
			})
		}
		_ = wizard.entityList.SetItems(items)
	}

	if opts.Pkg != "" {
		wizard.packageInput.SetValue(opts.Pkg)
	}
	if opts.Mod != "" {
		wizard.moduleInput.SetValue(opts.Mod)
	}
	if opts.Out != "" {
		wizard.outPath = opts.Out
		wizard.pathInput.SetValue(opts.Out)
		wizard.outputMode = outputFile
	}
	wizard.focusCurrentStep()
	return wizard
}

func schemaRelPath(schemaDir, cwd string) string {
	if rel, err := filepath.Rel(cwd, schemaDir); err == nil {
		return rel
	}
	return schemaDir
}

func (m *ServiceWizard) setProgram(program *tea.Program) { m.program = program }

func (m *ServiceWizard) Init() tea.Cmd {
	var cmds []tea.Cmd
	if m.step == serviceStepEntity {
		cmds = append(cmds, m.entityList.StartSpinner())
	} else {
		cmds = append(cmds, textinput.Blink)
	}
	return tea.Batch(cmds...)
}

// focusCurrentStep blurs every input and focuses the one belonging to the
// current step, so keystrokes always land in the visible field.
func (m *ServiceWizard) focusCurrentStep() {
	m.nameInput.Blur()
	m.packageInput.Blur()
	m.moduleInput.Blur()
	m.pathInput.Blur()
	switch m.step {
	case serviceStepName:
		m.nameInput.Focus()
	case serviceStepPackage:
		m.packageInput.Focus()
	case serviceStepModule:
		m.moduleInput.Focus()
	case serviceStepOutPath:
		m.pathInput.Focus()
	}
}

func (m *ServiceWizard) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	stepBefore := m.step
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.help.help.Width = msg.Width
		m.entityList.SetSize(msg.Width-4, min(12, max(6, msg.Height-10)))
		m.preview.Width = max(msg.Width-6, 20)
		m.preview.Height = min(20, max(8, msg.Height-14))
		return m, nil
	}

	if keyMessage, ok := message.(tea.KeyMsg); ok && key.Matches(keyMessage, m.keys.Quit) {
		return m, tea.Quit
	}

	model, cmd := m.updateByStep(message)
	if m.step != stepBefore {
		m.focusCurrentStep()
	}
	return model, cmd
}

func (m *ServiceWizard) updateByStep(message tea.Msg) (tea.Model, tea.Cmd) {
	switch m.step {
	case serviceStepEntity:
		return m.updateEntityStep(message)
	case serviceStepName:
		return m.updateNameStep(message)
	case serviceStepPackage:
		return m.updatePackageStep(message)
	case serviceStepModule:
		return m.updateModuleStep(message)
	case serviceStepPreview:
		return m.updatePreviewStep(message)
	case serviceStepOutput:
		return m.updateOutputStep(message)
	case serviceStepOutPath:
		return m.updateOutPathStep(message)
	case serviceStepOverwrite:
		return m.updateOverwriteStep(message)
	case serviceStepDone:
		return m, tea.Quit
	}
	return m, nil
}

func (m *ServiceWizard) updateEntityStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		newList, cmd := m.entityList.Update(message)
		m.entityList = newList
		return m, cmd
	}
	switch {
	case key.Matches(keyMessage, m.keys.Back):
		return m, tea.Quit
	case keyMessage.Type == tea.KeyEnter:
		chosen, ok := m.entityList.SelectedItem().(choiceItem)
		if !ok {
			return m, nil
		}
		if chosen.custom {
			m.step = serviceStepName
			return m, textinput.Blink
		}
		m.nameInput.SetValue(chosen.payload)
		m.step = serviceStepPackage
		return m, textinput.Blink
	}
	newList, cmd := m.entityList.Update(message)
	m.entityList = newList
	return m, cmd
}

func (m *ServiceWizard) updateNameStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			if len(m.entities) > 0 {
				m.step = serviceStepEntity
				return m, m.entityList.StartSpinner()
			}
			return m, tea.Quit
		case keyMessage.Type == tea.KeyEnter:
			name := strings.TrimSpace(m.nameInput.Value())
			if name == "" {
				return m, nil
			}
			m.step = serviceStepPackage
			return m, textinput.Blink
		}
	}
	newInput, cmd := m.nameInput.Update(message)
	m.nameInput = newInput
	return m, cmd
}

func (m *ServiceWizard) updatePackageStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			if len(m.entities) > 0 && m.nameInput.Value() == "" {
				m.step = serviceStepEntity
				return m, m.entityList.StartSpinner()
			}
			m.step = serviceStepName
			return m, textinput.Blink
		case keyMessage.Type == tea.KeyEnter:
			if strings.TrimSpace(m.packageInput.Value()) == "" {
				m.packageInput.SetValue("dash.v1")
			}
			m.step = serviceStepModule
			return m, textinput.Blink
		}
	}
	newInput, cmd := m.packageInput.Update(message)
	m.packageInput = newInput
	return m, cmd
}

func (m *ServiceWizard) updateModuleStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			m.step = serviceStepPackage
			return m, textinput.Blink
		case keyMessage.Type == tea.KeyEnter:
			if strings.TrimSpace(m.moduleInput.Value()) == "" {
				return m, nil
			}
			return m.enterPreview()
		}
	}
	newInput, cmd := m.moduleInput.Update(message)
	m.moduleInput = newInput
	return m, cmd
}

func (m *ServiceWizard) enterPreview() (tea.Model, tea.Cmd) {
	content, err := m.generate()
	if err != nil {
		m.genErr = err.Error()
		m.step = serviceStepPreview
		return m, nil
	}
	m.content = content
	m.preview.SetContent(content)
	m.preview.GotoTop()
	m.previewReady = true
	m.genErr = ""
	m.step = serviceStepPreview
	return m, nil
}

func (m *ServiceWizard) generate() (string, error) {
	name := strings.TrimSpace(m.nameInput.Value())
	pkg := strings.TrimSpace(m.packageInput.Value())
	if pkg == "" {
		pkg = "dash.v1"
	}
	switch m.kind {
	case ServiceKindProto:
		return service.GenServiceProto(name, pkg, m.schemaDir)
	case ServiceKindGolang:
		return service.GenServiceGolang(name, pkg, strings.TrimSpace(m.moduleInput.Value()), m.schemaDir)
	}
	return "", fmt.Errorf("unknown generator kind")
}

func (m *ServiceWizard) updatePreviewStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			m.step = serviceStepModule
			m.previewReady = false
			return m, textinput.Blink
		case keyMessage.Type == tea.KeyEnter && m.previewReady:
			m.step = serviceStepOutput
			return m, nil
		}
	}
	newPreview, cmd := m.preview.Update(message)
	m.preview = newPreview
	return m, cmd
}

func (m *ServiceWizard) updateOutputStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMessage.String() {
	case "esc":
		m.step = serviceStepPreview
		return m, nil
	case "up", "k":
		m.outputMode = outputStdout
		return m, nil
	case "down", "j":
		m.outputMode = outputFile
		return m, nil
	case "enter", " ", "right":
		if m.outputMode == outputFile {
			if m.pathInput.Value() == "" {
				m.pathInput.SetValue(m.suggestedOutPath())
			}
			m.step = serviceStepOutPath
			return m, textinput.Blink
		}
		m.printContent = true
		m.step = serviceStepDone
		return m, tea.Quit
	}
	return m, nil
}

// suggestedOutPath proposes a conventional location inside the current
// project based on the package name and the canonical entity file name.
func (m *ServiceWizard) suggestedOutPath() string {
	pkg := strings.TrimSpace(m.packageInput.Value())
	if pkg == "" {
		pkg = "dash.v1"
	}
	entity, _ := service.NormalizeEntity(strings.TrimSpace(m.nameInput.Value()), m.schemaDir)
	switch m.kind {
	case ServiceKindProto:
		return filepath.Join("proto", filepath.FromSlash(strings.ReplaceAll(pkg, ".", "/")), entity.FieldName+".proto")
	default:
		segment := pkg
		if i := strings.IndexAny(pkg, "./"); i >= 0 {
			segment = pkg[:i]
		}
		return filepath.Join("internal", "service", segment, entity.FieldName+".go")
	}
}

func (m *ServiceWizard) updateOutPathStep(message tea.Msg) (tea.Model, tea.Cmd) {
	if keyMessage, ok := message.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMessage, m.keys.Back):
			m.step = serviceStepOutput
			return m, nil
		case keyMessage.Type == tea.KeyEnter:
			path := strings.TrimSpace(m.pathInput.Value())
			if path == "" {
				return m, nil
			}
			m.outPath = path
			if _, err := os.Stat(path); err == nil {
				m.step = serviceStepOverwrite
				return m, nil
			}
			return m.writeFile()
		}
	}
	newInput, cmd := m.pathInput.Update(message)
	m.pathInput = newInput
	return m, cmd
}

func (m *ServiceWizard) updateOverwriteStep(message tea.Msg) (tea.Model, tea.Cmd) {
	keyMessage, ok := message.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMessage.String() {
	case "y", "Y", "enter":
		return m.writeFile()
	case "n", "N", "esc":
		m.step = serviceStepOutPath
		return m, textinput.Blink
	}
	return m, nil
}

func (m *ServiceWizard) writeFile() (tea.Model, tea.Cmd) {
	dir := filepath.Dir(m.outPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		m.genErr = err.Error()
		return m, nil
	}
	perm := os.FileMode(0o644)
	if strings.HasSuffix(m.outPath, ".sh") {
		perm = 0o755
	}
	if err := os.WriteFile(m.outPath, []byte(m.content), perm); err != nil {
		m.genErr = err.Error()
		return m, nil
	}
	m.written = true
	m.step = serviceStepDone
	return m, tea.Quit
}

func (m *ServiceWizard) View() string {
	var body string
	switch m.step {
	case serviceStepEntity:
		body = m.viewEntity()
	case serviceStepName:
		body = m.viewName()
	case serviceStepPackage:
		body = m.viewPackage()
	case serviceStepModule:
		body = m.viewModule()
	case serviceStepPreview:
		body = m.viewPreview()
	case serviceStepOutput:
		body = m.viewOutput()
	case serviceStepOutPath:
		body = m.viewOutPath()
	case serviceStepOverwrite:
		body = m.viewOverwrite()
	case serviceStepDone:
		body = m.viewDone()
	}
	return body
}

func (m *ServiceWizard) title() string {
	if m.kind == ServiceKindProto {
		return "Generate service proto"
	}
	return "Generate service golang"
}

func (m *ServiceWizard) viewEntity() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + SubtitleStyle.Render("  (detected ent schemas)") + "\n\n")
	builder.WriteString(m.entityList.View())
	return builder.String()
}

func (m *ServiceWizard) viewName() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + "\n\n")
	builder.WriteString(FieldLabelStyle.Render("  Entity/service name:") + "\n  ")
	builder.WriteString(m.nameInput.View())
	builder.WriteString("\n\n" + SubtitleStyle.Render("  enter continue · esc back"))
	return builder.String()
}

func (m *ServiceWizard) viewPackage() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + "\n\n")
	builder.WriteString(FieldLabelStyle.Render("  Proto/go package (e.g. dash.v1):") + "\n  ")
	builder.WriteString(m.packageInput.View())
	builder.WriteString("\n\n" + SubtitleStyle.Render("  enter continue · esc back"))
	return builder.String()
}

func (m *ServiceWizard) viewModule() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + "\n\n")
	builder.WriteString(FieldLabelStyle.Render("  Go module path (used in generated imports):") + "\n  ")
	builder.WriteString(m.moduleInput.View())
	builder.WriteString("\n\n" + SubtitleStyle.Render("  enter continue · esc back"))
	return builder.String()
}

func (m *ServiceWizard) viewPreview() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + SubtitleStyle.Render("  (preview — enter continues)") + "\n")
	if m.genErr != "" {
		builder.WriteString("\n  " + ErrorStyle.Render("✘ generation failed: "+m.genErr) + "\n")
		builder.WriteString("\n" + SubtitleStyle.Render("  esc back"))
		return builder.String()
	}
	builder.WriteString(m.preview.View())
	builder.WriteString("\n" + SubtitleStyle.Render("  ↑/↓ scroll · enter continue · esc back"))
	return builder.String()
}

func (m *ServiceWizard) viewOutput() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + SubtitleStyle.Render("  (choose output)") + "\n\n")
	options := []struct {
		label  string
		detail string
	}{
		{"Print to stdout", "pipe or redirect from the terminal"},
		{"Write to file", "save into the project tree"},
	}
	for i, option := range options {
		cursor, style := "  ", lipgloss.NewStyle()
		if i == m.outputMode {
			cursor, style = "> ", style.Bold(true).Foreground(lipgloss.Color(accentColor))
		}
		builder.WriteString("  " + cursor + style.Render(option.label) + SubtitleStyle.Render("   "+option.detail) + "\n")
	}
	builder.WriteString("\n" + SubtitleStyle.Render("  up/down choose · enter select · esc back"))
	return builder.String()
}

func (m *ServiceWizard) viewOutPath() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + "\n\n")
	builder.WriteString(FieldLabelStyle.Render("  Output file path (relative to project root):") + "\n  ")
	builder.WriteString(m.pathInput.View())
	builder.WriteString("\n\n" + SubtitleStyle.Render("  enter write · esc back"))
	return builder.String()
}

func (m *ServiceWizard) viewOverwrite() string {
	var builder strings.Builder
	builder.WriteString(TitleStyle.Render(m.title()) + "\n\n")
	builder.WriteString("  " + WarningStyle.Render("⚠ "+m.outPath+" already exists.") + "\n\n")
	builder.WriteString(SubtitleStyle.Render("  y overwrite · n/esc back"))
	return builder.String()
}

func (m *ServiceWizard) viewDone() string {
	if m.printContent {
		return m.title() + "\n" + SubtitleStyle.Render("Sent to stdout.")
	}
	return m.title() + "\n" + SuccessStyle.Render("✔ Written to "+m.outPath)
}
