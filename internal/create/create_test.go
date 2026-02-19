package create

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-sphere/sphere-cli/internal/renamer"
)

var createTest = flag.Bool("create_test", false, "run create tests that create files and directories")

func TestProject(t *testing.T) {
	if !*createTest {
		t.Skip("Skipping create tests, run with -create_test to enable")
	}
	err := Project("example", "example", templateLayouts[""])
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll("example")
}

func TestLayout(t *testing.T) {
	if !*createTest {
		t.Skip("Skipping create tests, run with -create_test to enable")
	}
	layout, err := Layout("https://go-sphere.github.io/layout/simple.json")
	if err != nil {
		t.Fatal(err)
	}
	err = Project("simple", "simple", layout)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll("simple")
}

func TestSimpleLayoutCreateAndRenameBuild(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping integration test in CI")
	}
	if !*createTest {
		t.Skip("Skipping integration test, run with -create_test to enable")
	}

	workspace := t.TempDir()
	prevDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workspace); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(prevDir)
	})

	projectName := "simple-e2e"
	oldModule := "github.com/example/simple-e2e"
	newModule := "github.com/example/simple-e2e-renamed"

	if err := Project(projectName, oldModule, templateLayouts["simple"]); err != nil {
		t.Fatal(err)
	}

	projectDir := filepath.Join(workspace, projectName)
	if err := renamer.RenameProjectModule(oldModule, newModule, projectDir, []string{
		"buf.gen.yaml",
		"buf.binding.yaml",
	}, true); err != nil {
		t.Fatal(err)
	}

	if err := execCommands(projectDir,
		[]string{"make", "init"},
		[]string{"make", "build"},
	); err != nil {
		t.Fatal(err)
	}
}
