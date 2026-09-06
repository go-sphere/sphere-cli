package renamer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameModuleImportBoundary(t *testing.T) {
	source := `package p

import (
	"github.com/a/foo"
	"github.com/a/foo/sub"
	"github.com/a/foobar/x"
	"net/http"
)

func F() { _ = http.StatusOK }
`
	path := filepath.Join(t.TempDir(), "p.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	if err := RenameModule("github.com/a/foo", "example.com/n", path); err != nil {
		t.Fatalf("RenameModule() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	content := string(got)
	for _, want := range []string{
		`"example.com/n"`,
		`"example.com/n/sub"`,
		`"github.com/a/foobar/x"`,
		`"net/http"`,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("result missing %s:\n%s", want, content)
		}
	}
	for _, forbidden := range []string{
		`"github.com/a/foo"`,  // module root must be rewritten
		`example.com/nbar/x"`, // sibling module must not be mangled
		`example.com/n/x"`,    // foobar import corrupted
	} {
		if strings.Contains(content, forbidden) {
			t.Errorf("result contains forbidden %s:\n%s", forbidden, content)
		}
	}
}

func TestRenameProjectModuleRewritesModuleAndRelatedFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":          "module github.com/a/foo\n\ngo 1.26.0\n",
		"main.go":         "package main\n\nimport _ \"github.com/a/foo/internal/x\"\n\nfunc main() {}\n",
		"buf.gen.yaml":    "module: github.com/a/foo\n",
		"missing.ignored": "",
	}
	for path, content := range files {
		if content == "" {
			continue
		}
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	err := RenameProjectModule("github.com/a/foo", "example.com/n", dir, []string{"buf.gen.yaml", "buf.binding.yaml"}, true)
	if err != nil {
		t.Fatalf("RenameProjectModule(ignoreMissing=true) error = %v", err)
	}
	mod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if !strings.HasPrefix(string(mod), "module example.com/n\n") {
		t.Errorf("go.mod not renamed:\n%s", mod)
	}
	buf, err := os.ReadFile(filepath.Join(dir, "buf.gen.yaml"))
	if err != nil {
		t.Fatalf("read buf.gen.yaml: %v", err)
	}
	if !strings.Contains(string(buf), "example.com/n") {
		t.Errorf("buf.gen.yaml not rewritten:\n%s", buf)
	}

	// Same layout with a missing related file must fail when not ignored.
	err = RenameProjectModule("github.com/a/foo", "example.com/n", dir, []string{"buf.gen.yaml", "buf.binding.yaml"}, false)
	if err == nil {
		t.Fatal("RenameProjectModule(ignoreMissing=false) error = nil, want error")
	}
}

func TestRenameProjectModuleGoModMismatch(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/other\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	err := RenameDirModule("github.com/a/foo", "example.com/n", dir)
	if err == nil || !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("RenameDirModule() error = %v, want go.mod mismatch", err)
	}
}
