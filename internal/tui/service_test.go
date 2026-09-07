package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const schemaFixture = `package schema

import (
	"entgo.io/ent"
)

type KeyValueStore struct {
	ent.Schema
}

func (KeyValueStore) Fields() []ent.Field { return nil }
`

func newProtoWizardInProject(t *testing.T) *ServiceWizard {
	t.Helper()
	project := t.TempDir()
	schemaDir := filepath.Join(project, "internal", "pkg", "database", "schema")
	if err := os.MkdirAll(schemaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(schemaDir, "key_value_store.go"), []byte(schemaFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(project)
	return newServiceWizard(ServiceWizardOptions{Kind: ServiceKindProto, CWD: project})
}

func updateService(t *testing.T, wizard *ServiceWizard, message tea.Msg) *ServiceWizard {
	t.Helper()
	next, _ := wizard.Update(message)
	model, ok := next.(*ServiceWizard)
	if !ok {
		t.Fatalf("Update returned %T, want *ServiceWizard", next)
	}
	return model
}

func TestServiceWizardListsSchemaEntities(t *testing.T) {
	wizard := newProtoWizardInProject(t)
	if wizard.schemaDir == "" {
		t.Fatal("schema directory was not detected")
	}
	items := wizard.entityList.Items()
	if len(items) != 2 {
		t.Fatalf("expected custom entry plus 1 entity, got %d", len(items))
	}
	entity, ok := items[1].(choiceItem)
	if !ok || entity.payload != "KeyValueStore" {
		t.Fatalf("expected the KeyValueStore schema entity, got %+v", items[1])
	}

	// Selecting the schema entity advances to the package step.
	wizard.entityList.Select(1)
	wizard = updateService(t, wizard, enterKey())
	if wizard.step != serviceStepPackage || wizard.nameInput.Value() != "KeyValueStore" {
		t.Fatalf("entity selection failed: step=%v name=%q", wizard.step, wizard.nameInput.Value())
	}
}

func TestServiceWizardOutputToFileWithSuggestedPath(t *testing.T) {
	wizard := newProtoWizardInProject(t)
	wizard.nameInput.SetValue("KeyValueStore")

	// package -> module -> preview
	wizard.step = serviceStepPackage
	wizard = updateService(t, wizard, enterKey())
	wizard = updateService(t, wizard, enterKey())
	if wizard.step != serviceStepPreview || wizard.content == "" {
		t.Fatalf("expected a generated preview, step=%v len=%d", wizard.step, len(wizard.content))
	}

	// preview -> output choice
	wizard = updateService(t, wizard, enterKey())
	if wizard.step != serviceStepOutput {
		t.Fatalf("expected output step, got %v", wizard.step)
	}

	// choose "write to file"
	wizard.outputMode = outputFile
	wizard = updateService(t, wizard, enterKey())
	if wizard.step != serviceStepOutPath {
		t.Fatalf("expected out-path step, got %v", wizard.step)
	}
	if want := filepath.Join("proto", "dash", "v1", "key_value_store.proto"); wizard.pathInput.Value() != want {
		t.Fatalf("suggested path = %q, want %q", wizard.pathInput.Value(), want)
	}

	// writing creates the file on disk, including parent directories.
	wizard = updateService(t, wizard, enterKey())
	if wizard.step != serviceStepDone || !wizard.written {
		t.Fatalf("expected a written file, step=%v written=%v", wizard.step, wizard.written)
	}
	data, err := os.ReadFile(wizard.outPath)
	if err != nil || !strings.Contains(string(data), "service KeyValueStoreService") {
		t.Fatalf("written file missing expected content: %v", err)
	}
}

func TestServiceWizardStdoutOutput(t *testing.T) {
	wizard := newProtoWizardInProject(t)
	wizard.nameInput.SetValue("KeyValueStore")
	wizard.step = serviceStepPackage

	wizard = updateService(t, wizard, enterKey()) // package -> module
	wizard = updateService(t, wizard, enterKey()) // module -> preview
	wizard = updateService(t, wizard, enterKey()) // preview -> output

	// stdout stays the default selection; enter finishes with printContent.
	wizard = updateService(t, wizard, enterKey())
	if wizard.step != serviceStepDone || !wizard.printContent {
		t.Fatalf("expected stdout finish, step=%v print=%v", wizard.step, wizard.printContent)
	}
	if !strings.Contains(wizard.content, "service KeyValueStoreService") {
		t.Fatal("generated proto content is missing the service definition")
	}
}

func TestServiceWizardOverwriteConfirmation(t *testing.T) {
	wizard := newProtoWizardInProject(t)
	wizard.nameInput.SetValue("KeyValueStore")
	wizard.step = serviceStepPackage

	wizard = updateService(t, wizard, enterKey())
	wizard = updateService(t, wizard, enterKey())
	wizard = updateService(t, wizard, enterKey())
	wizard.outputMode = outputFile
	wizard = updateService(t, wizard, enterKey())

	target := wizard.suggestedOutPath()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	wizard = updateService(t, wizard, enterKey()) // out path -> overwrite prompt
	if wizard.step != serviceStepOverwrite {
		t.Fatalf("expected overwrite prompt, got %v", wizard.step)
	}
	wizard = updateService(t, wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if wizard.step != serviceStepDone || !wizard.written {
		t.Fatalf("overwrite failed: step=%v written=%v", wizard.step, wizard.written)
	}
	if data, err := os.ReadFile(target); err != nil || string(data) == "old" {
		t.Fatalf("file was not overwritten: %v %q", err, data)
	}
}

func TestServiceWizardGolangSuggestedPath(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	wizard := newServiceWizard(ServiceWizardOptions{
		Kind: ServiceKindGolang,
		CWD:  project,
		Name: "key_value_store",
		Pkg:  "dash.v1",
	})
	if wizard.step != serviceStepPackage {
		t.Fatalf("prefilled name should skip to the package step, got %v", wizard.step)
	}
	if got := wizard.suggestedOutPath(); got != filepath.Join("internal", "service", "dash", "key_value_store.go") {
		t.Fatalf("suggestedOutPath() = %q", got)
	}
}
