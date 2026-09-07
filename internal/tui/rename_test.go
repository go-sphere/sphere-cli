package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func updateRename(t *testing.T, wizard *RenameWizard, message tea.Msg) *RenameWizard {
	t.Helper()
	next, _ := wizard.Update(message)
	model, ok := next.(*RenameWizard)
	if !ok {
		t.Fatalf("Update returned %T, want *RenameWizard", next)
	}
	return model
}

func newRenameProject(t *testing.T) string {
	t.Helper()
	project := t.TempDir()
	main := "package main\n\nimport \"github.com/old/app/internal/pkg\"\n\nfunc main() { pkg.Hello() }\n"
	pkgDir := filepath.Join(project, "internal", "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(project, "go.mod"):       "module github.com/old/app\n\ngo 1.26.0\n",
		filepath.Join(project, "main.go"):      main,
		filepath.Join(pkgDir, "pkg.go"):        "package pkg\n\nfunc Hello() {}\n",
		filepath.Join(project, "buf.gen.yaml"): "module: github.com/old/app\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return project
}

func TestRenameWizardDetectsModuleFromGoMod(t *testing.T) {
	project := newRenameProject(t)
	t.Chdir(project)

	wizard := newRenameWizard(RenameWizardOptions{})
	if !wizard.detected || wizard.oldInput.Value() != "github.com/old/app" {
		t.Fatalf("module not detected: detected=%v old=%q", wizard.detected, wizard.oldInput.Value())
	}

	// enter on the target step moves to the module step (new module focused).
	wizard = updateRename(t, wizard, enterKey())
	if wizard.step != renameStepModule {
		t.Fatalf("expected module step, got %v", wizard.step)
	}

	wizard.newInput.SetValue("github.com/new/app")
	wizard = updateRename(t, wizard, enterKey())
	if wizard.step != renameStepConfirm {
		t.Fatalf("expected confirm step, got %v", wizard.step)
	}

	// enter on confirm starts the rename; run the returned command to get the
	// completion message.
	_, cmd := wizard.Update(enterKey())
	if cmd == nil {
		t.Fatal("confirm enter did not schedule the rename")
	}
	result, ok := cmd().(renameFinishedMsg)
	if !ok {
		t.Fatalf("rename command returned %T", cmd())
	}
	if result.err != nil {
		t.Fatalf("rename failed: %v", result.err)
	}

	modData, err := os.ReadFile(filepath.Join(project, "go.mod"))
	if err != nil || !strings.HasPrefix(string(modData), "module github.com/new/app") {
		t.Fatalf("go.mod not renamed: %v %s", err, modData)
	}
	mainData, err := os.ReadFile(filepath.Join(project, "main.go"))
	if err != nil || !strings.Contains(string(mainData), `"github.com/new/app/internal/pkg"`) {
		t.Fatalf("imports not rewritten: %v %s", err, mainData)
	}
	bufData, err := os.ReadFile(filepath.Join(project, "buf.gen.yaml"))
	if err != nil || !strings.Contains(string(bufData), "github.com/new/app") {
		t.Fatalf("related file not rewritten: %v %s", err, bufData)
	}
}

func TestRenameWizardRejectsSameModule(t *testing.T) {
	project := newRenameProject(t)
	t.Chdir(project)

	wizard := newRenameWizard(RenameWizardOptions{})
	wizard = updateRename(t, wizard, enterKey())
	wizard.newInput.SetValue("github.com/old/app")
	wizard = updateRename(t, wizard, enterKey())
	if wizard.step != renameStepModule || wizard.moduleErr == "" {
		t.Fatalf("expected a same-module error, step=%v err=%q", wizard.step, wizard.moduleErr)
	}
}

func TestRenameWizardTargetValidation(t *testing.T) {
	t.Chdir(t.TempDir())

	wizard := newRenameWizard(RenameWizardOptions{})
	wizard.targetInput.SetValue("does-not-exist")
	wizard = updateRename(t, wizard, enterKey())
	if wizard.step != renameStepTarget || wizard.targetErr == "" {
		t.Fatalf("expected a target error, step=%v err=%q", wizard.step, wizard.targetErr)
	}
}
