package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateProjectName(t *testing.T) {
	valid := []string{"my-project", "blog", "app1", "Hello.World", "foo_bar"}
	for _, name := range valid {
		if err := validateProjectName(name); err != nil {
			t.Errorf("validateProjectName(%q) error = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",          // empty is reported by --name check, but must not panic
		"  ",        // whitespace only
		" lead",     // leading whitespace
		"trail ",    // trailing whitespace
		"a/b",       // path separator
		`a\b`,       // Windows separator
		".",         // current dir
		"..",        // parent dir
		"../evil",   // traversal
		"a/../evil", // traversal via segment
	}
	for _, name := range invalid {
		if err := validateProjectName(name); err == nil {
			t.Errorf("validateProjectName(%q) error = nil, want error", name)
		}
	}
}

func TestSchemaDirForCWD(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })

	t.Run("empty dir", func(t *testing.T) {
		if err := os.Chdir(t.TempDir()); err != nil {
			t.Fatal(err)
		}
		if got := schemaDirForCWD(); got != "" {
			t.Fatalf("schemaDirForCWD() in empty dir = %q, want empty", got)
		}
	})

	t.Run("sphere layout", func(t *testing.T) {
		root := t.TempDir()
		schemaDir := filepath.Join(root, "internal", "pkg", "database", "schema")
		if err := os.MkdirAll(schemaDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(root); err != nil {
			t.Fatal(err)
		}
		got := schemaDirForCWD()
		want, err := filepath.Abs(schemaDir)
		if err != nil {
			t.Fatal(err)
		}
		// macOS maps /var -> /private/var; normalise both sides so the
		// comparison is not affected by symlinked prefixes.
		if resolvedGot, err := filepath.EvalSymlinks(got); err == nil {
			got = resolvedGot
		}
		if resolvedWant, err := filepath.EvalSymlinks(want); err == nil {
			want = resolvedWant
		}
		if got != want {
			t.Fatalf("schemaDirForCWD() = %q, want %q", got, want)
		}
	})
}
