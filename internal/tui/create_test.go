package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/go-sphere/sphere-cli/internal/create"
)

func enterKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

func update(t *testing.T, model *CreateWizard, message tea.Msg) *CreateWizard {
	t.Helper()
	next, _ := model.Update(message)
	wizard, ok := next.(*CreateWizard)
	if !ok {
		t.Fatalf("Update returned %T, want *CreateWizard", next)
	}
	return wizard
}

func TestCreateWizardWalksThroughFormSteps(t *testing.T) {
	t.Chdir(t.TempDir())

	wizard := newCreateWizard(CreateWizardOptions{Name: "app", Module: "example.com/app", Layout: "standard"})
	if wizard.step != createStepTemplate || !wizard.loading {
		t.Fatalf("wizard should start on the loading template step, got step=%v loading=%v", wizard.step, wizard.loading)
	}

	// The catalog loads and preselects the --layout name.
	wizard = update(t, wizard, layoutsLoadedMsg{layouts: []*create.LayoutItem{
		{Name: "simple", Description: "d"},
		{Name: "standard", Description: "d"},
	}})
	if wizard.loading || len(wizard.layouts) != 2 {
		t.Fatalf("catalog not applied: loading=%v layouts=%d", wizard.loading, len(wizard.layouts))
	}
	if item, ok := wizard.list.SelectedItem().(layoutListItem); !ok || item.item.Name != "standard" {
		t.Fatalf("--layout standard was not preselected: %+v", wizard.list.SelectedItem())
	}

	// enter accepts the template and moves to the name step.
	wizard = update(t, wizard, enterKey())
	if wizard.step != createStepName || wizard.nameInput.Value() != "app" {
		t.Fatalf("expected name step with prefilled name, got step=%v name=%q", wizard.step, wizard.nameInput.Value())
	}

	// enter submits the name and moves to the module step, keeping the
	// --module prefill untouched.
	wizard = update(t, wizard, enterKey())
	if wizard.step != createStepModule || wizard.moduleInput.Value() != "example.com/app" {
		t.Fatalf("expected module step with preserved prefill, got step=%v module=%q", wizard.step, wizard.moduleInput.Value())
	}
	wizard = update(t, wizard, enterKey())
	if wizard.step != createStepOptions {
		t.Fatalf("expected options step, got %v", wizard.step)
	}

	// toggling both options off is reflected in the model.
	wizard = update(t, wizard, tea.KeyMsg{Type: tea.KeySpace})
	if wizard.options[optionGitIndex].enabled {
		t.Fatal("space did not toggle the git option off")
	}

	// enter moves to confirm; the wizard must NOT start creating without a
	// final enter.
	wizard = update(t, wizard, enterKey())
	if wizard.step != createStepConfirm {
		t.Fatalf("expected confirm step, got %v", wizard.step)
	}
	if view := wizard.View(); view == "" {
		t.Fatal("confirm view is empty")
	}
}

func TestCreateWizardModuleSuggestsProjectName(t *testing.T) {
	t.Chdir(t.TempDir())

	// Without a --module prefill the module step stays empty and shows the
	// project name as an editable default suggestion.
	wizard := newCreateWizard(CreateWizardOptions{Name: "app"})
	wizard = update(t, wizard, layoutsLoadedMsg{layouts: []*create.LayoutItem{{Name: "standard"}}})
	wizard = update(t, wizard, enterKey()) // template -> name
	wizard = update(t, wizard, enterKey()) // name -> module

	if wizard.step != createStepModule || wizard.moduleInput.Value() != "" {
		t.Fatalf("expected empty module step, got step=%v module=%q", wizard.step, wizard.moduleInput.Value())
	}
	if !strings.Contains(wizard.moduleInput.Placeholder, "app") {
		t.Fatalf("module placeholder should suggest the project name, got %q", wizard.moduleInput.Placeholder)
	}

	// typing a custom module replaces the suggestion wholesale.
	wizard = update(t, wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("example.com/app")})
	wizard = update(t, wizard, enterKey())
	if wizard.step != createStepOptions || wizard.effectiveModule() != "example.com/app" {
		t.Fatalf("expected options step with custom module, got step=%v module=%q", wizard.step, wizard.effectiveModule())
	}

	// an empty module falls back to the project name.
	wizard.moduleInput.SetValue("")
	if got := wizard.effectiveModule(); got != "app" {
		t.Fatalf("effectiveModule() = %q, want the project name fallback", got)
	}
}

func TestCreateWizardValidatesProjectName(t *testing.T) {
	t.Chdir(t.TempDir())

	wizard := newCreateWizard(CreateWizardOptions{Name: "../evil"})
	wizard.loading = false
	wizard = update(t, wizard, layoutsLoadedMsg{layouts: []*create.LayoutItem{{Name: "standard"}}})
	wizard = update(t, wizard, enterKey()) // template -> name
	wizard = update(t, wizard, enterKey()) // submit invalid name

	if wizard.step != createStepName || wizard.nameErr == "" {
		t.Fatalf("expected the name step with a validation error, got step=%v err=%q", wizard.step, wizard.nameErr)
	}
}

func TestCreateWizardRejectsExistingTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(filepath.Join(dir, "taken"), 0o755); err != nil {
		t.Fatal(err)
	}

	wizard := newCreateWizard(CreateWizardOptions{Name: "taken"})
	wizard.loading = false
	wizard = update(t, wizard, layoutsLoadedMsg{layouts: []*create.LayoutItem{{Name: "standard"}}})
	wizard = update(t, wizard, enterKey()) // template -> name
	wizard = update(t, wizard, enterKey()) // submit; directory already exists

	if wizard.step != createStepName || wizard.nameErr == "" {
		t.Fatalf("expected an existing-directory error, got step=%v err=%q", wizard.step, wizard.nameErr)
	}
}

func TestCreateWizardProgressAccumulatesAcrossSteps(t *testing.T) {
	wizard := newCreateWizard(CreateWizardOptions{Name: "app"})
	wizard = update(t, wizard, tea.WindowSizeMsg{Width: 100, Height: 30})

	total := float64(6)
	cases := []struct {
		event create.ProgressEvent
		want  float64
	}{
		{create.ProgressEvent{Step: 0, TotalSteps: 6, Label: "Downloading template", Percent: 0.5}, 0.5 / total},
		{create.ProgressEvent{Step: 1, TotalSteps: 6, Label: "Renaming Go module", Percent: -1}, 1.0 / total},
		{create.ProgressEvent{Step: 5, TotalSteps: 6, Label: "Writing project files", Percent: 1}, (5.0 + 1) / total},
	}
	for _, tt := range cases {
		wizard = update(t, wizard, progressMsg{event: tt.event})
		if got := wizard.progress.Percent(); got < tt.want-0.01 || got > tt.want+0.01 {
			t.Errorf("progress after step %d = %v, want ~%v", tt.event.Step, got, tt.want)
		}
	}
}

func TestCreateWizardCustomLayoutSkipsPicker(t *testing.T) {
	wizard := newCreateWizard(CreateWizardOptions{Layout: "https://example.com/layout.json"})
	if wizard.step != createStepName || wizard.customLayoutURI == "" {
		t.Fatalf("custom layout URI should skip the picker, got step=%v uri=%q", wizard.step, wizard.customLayoutURI)
	}
	if got := wizard.resolvedLayoutName(); got != "https://example.com/layout.json" {
		t.Fatalf("resolvedLayoutName() = %q, want the custom URI", got)
	}
}
