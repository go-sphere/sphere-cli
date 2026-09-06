package create

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLayoutBuiltIn(t *testing.T) {
	tests := []struct {
		name string
		want TemplateLayout
	}{
		{
			name: "",
			want: TemplateLayout{
				Name:   "standard",
				Source: "https://github.com/go-sphere/sphere-layout.git",
				Ref:    "master",
				Mod:    "github.com/go-sphere/sphere-layout",
			},
		},
		{
			name: "standard",
			want: TemplateLayout{
				Name:   "standard",
				Source: "https://github.com/go-sphere/sphere-layout.git",
				Ref:    "master",
				Mod:    "github.com/go-sphere/sphere-layout",
			},
		},
		{
			name: "bun",
			want: TemplateLayout{
				Name:   "bun",
				Source: "https://github.com/go-sphere/sphere-bun-layout.git",
				Ref:    "master",
				Mod:    "github.com/go-sphere/sphere-bun-layout",
			},
		},
		{
			name: "simple",
			want: TemplateLayout{
				Name:   "simple",
				Source: "https://github.com/go-sphere/sphere-simple-layout.git",
				Ref:    "master",
				Mod:    "github.com/go-sphere/sphere-simple-layout",
			},
		},
		{
			name: "telegram",
			want: TemplateLayout{
				Name:   "telegram",
				Source: "https://github.com/go-sphere/sphere-telegram-layout.git",
				Ref:    "master",
				Mod:    "github.com/go-sphere/sphere-telegram-layout",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Layout(tt.name)
			if err != nil {
				t.Fatalf("Layout(%q) error = %v", tt.name, err)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("Layout(%q) = %#v, want %#v", tt.name, *got, tt.want)
			}
		})
	}
}

func TestLayoutRemote(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    *TemplateLayout
		wantErr string
	}{
		{
			name:   "valid Git layout",
			status: http.StatusOK,
			body:   `{"name":"custom","source":"https://example.com/layout.git","ref":"main","mod":"example.com/layout"}`,
			want:   &TemplateLayout{Name: "custom", Source: "https://example.com/layout.git", Ref: "main", Mod: "example.com/layout"},
		},
		{
			name:   "valid",
			status: http.StatusOK,
			body:   `{"uri":"https://example.com/layout.zip","mod":"example.com/layout","path":"layout-main"}`,
			want:   &TemplateLayout{URI: "https://example.com/layout.zip", Mod: "example.com/layout", Path: "layout-main"},
		},
		{
			name:    "HTTP error",
			status:  http.StatusBadGateway,
			wantErr: "failed to fetch layout configuration: 502 Bad Gateway",
		},
		{
			name:    "missing required field",
			status:  http.StatusOK,
			body:    `{"uri":"https://example.com/layout.zip","mod":"example.com/layout"}`,
			wantErr: "invalid layout configuration",
		},
		{
			name:    "Git layout missing ref",
			status:  http.StatusOK,
			body:    `{"name":"custom","source":"https://example.com/layout.git","mod":"example.com/layout"}`,
			wantErr: "invalid layout configuration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			got, err := Layout(server.URL)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Layout() error = nil, want %q", tt.wantErr)
				}
				if gotErr := err.Error(); gotErr != tt.wantErr {
					t.Errorf("Layout() error = %q, want %q", gotErr, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Layout() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestProjectFromOfficialGitLayoutsWritesExactLockAndCleanCommit(t *testing.T) {
	for _, layoutName := range []string{"standard", "bun", "simple", "telegram"} {
		t.Run(layoutName, func(t *testing.T) {
			layout, err := Layout(layoutName)
			if err != nil {
				t.Fatalf("Layout(%q) error = %v", layoutName, err)
			}
			source, revision := createGitLayoutFixture(t, layout.Mod, layout.Name)
			localLayout := *layout
			localLayout.Source = source
			assertGitProjectCreation(t, &localLayout, revision)
		})
	}
}

func createGitLayoutFixture(t *testing.T, module, name string) (string, string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(filepath.Join(source, ".sphere"), 0o755); err != nil {
		t.Fatalf("create source: %v", err)
	}
	writeFixtureFile(t, filepath.Join(source, "go.mod"), "module "+module+"\n\ngo 1.26.0\n")
	writeFixtureFile(t, filepath.Join(source, "main.go"), "package main\n\nfunc main() {}\n")
	writeFixtureFile(t, filepath.Join(source, "buf.gen.yaml"), "module: "+module+"\n")
	writeFixtureFile(t, filepath.Join(source, "buf.binding.yaml"), "module: "+module+"\n")
	writeFixtureFile(t, filepath.Join(source, "Makefile"), "init:\n\t@true\n")
	writeFixtureFile(t, filepath.Join(source, ".sphere", "layout.json"), `{ "schema_version": 1, "name": "`+name+`" }`)

	runGit(t, source, "init", "-b", "master")
	runGit(t, source, "add", ".")
	runGit(t, source, "-c", "user.name=Sphere Test", "-c", "user.email=sphere@example.com", "commit", "-m", "fixture")
	return source, strings.TrimSpace(runGit(t, source, "rev-parse", "HEAD"))
}

func assertGitProjectCreation(t *testing.T, layout *TemplateLayout, revision string) {
	t.Helper()
	setGitIdentity(t)
	workspace := t.TempDir()
	t.Chdir(workspace)
	if err := Project("generated", "example.com/project", layout); err != nil {
		t.Fatalf("Project() error = %v", err)
	}

	projectDir := filepath.Join(workspace, "generated")
	raw, err := os.ReadFile(filepath.Join(projectDir, ".sphere", "layout.lock.json"))
	if err != nil {
		t.Fatalf("read layout lock: %v", err)
	}
	var lock LayoutLock
	if err := json.Unmarshal(raw, &lock); err != nil {
		t.Fatalf("decode layout lock: %v", err)
	}
	if lock.SchemaVersion != 1 || lock.Name != layout.Name || lock.BaseRevision != revision || lock.Repository != layout.Source || lock.Ref != layout.Ref || lock.UpstreamModule != layout.Mod {
		t.Fatalf("unexpected layout lock: %+v", lock)
	}
	moduleFile, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read project go.mod: %v", err)
	}
	if !strings.HasPrefix(string(moduleFile), "module example.com/project\n") {
		t.Fatalf("module was not renamed: %s", moduleFile)
	}
	if status := strings.TrimSpace(runGit(t, projectDir, "status", "--porcelain")); status != "" {
		t.Fatalf("created project is dirty: %s", status)
	}
}

func TestProjectFromLegacyZipRemainsCompatibleWithoutLock(t *testing.T) {
	var archive bytes.Buffer
	zipWriter := zip.NewWriter(&archive)
	files := map[string]string{
		"layout-main/go.mod":           "module example.com/layout\n\ngo 1.26.0\n",
		"layout-main/main.go":          "package main\n\nfunc main() {}\n",
		"layout-main/buf.gen.yaml":     "module: example.com/layout\n",
		"layout-main/buf.binding.yaml": "module: example.com/layout\n",
		"layout-main/Makefile":         "init:\n\t@true\n",
	}
	for path, content := range files {
		file, err := zipWriter.Create(path)
		if err != nil {
			t.Fatalf("create ZIP entry: %v", err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatalf("write ZIP entry: %v", err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("close ZIP: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(archive.Bytes())
	}))
	t.Cleanup(server.Close)

	setGitIdentity(t)
	workspace := t.TempDir()
	t.Chdir(workspace)
	layout := &TemplateLayout{
		URI:  server.URL,
		Mod:  "example.com/layout",
		Path: "layout-main",
	}
	if err := Project("generated", "example.com/project", layout); err != nil {
		t.Fatalf("Project() error = %v", err)
	}
	projectDir := filepath.Join(workspace, "generated")
	if _, err := os.Stat(filepath.Join(projectDir, ".sphere", "layout.lock.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy ZIP unexpectedly produced a synchronization lock: %v", err)
	}
	moduleFile, err := os.ReadFile(filepath.Join(projectDir, "go.mod"))
	if err != nil {
		t.Fatalf("read project go.mod: %v", err)
	}
	if !strings.HasPrefix(string(moduleFile), "module example.com/project\n") {
		t.Fatalf("module was not renamed: %s", moduleFile)
	}
	if status := strings.TrimSpace(runGit(t, projectDir, "status", "--porcelain")); status != "" {
		t.Fatalf("created project is dirty: %s", status)
	}
}

func setGitIdentity(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_NAME", "Sphere Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "sphere@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Sphere Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "sphere@example.com")
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func TestLayoutRemoteRejectsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"uri":`))
	}))
	t.Cleanup(server.Close)

	if _, err := Layout(server.URL); err == nil {
		t.Fatal("Layout() error = nil, want JSON decoding error")
	}
}

func TestProjectFailsWhenTargetDirectoryExists(t *testing.T) {
	setGitIdentity(t)
	source, _ := createGitLayoutFixture(t, "example.com/layout", "standard")
	workspace := t.TempDir()
	t.Chdir(workspace)

	layout := &TemplateLayout{Name: "standard", Source: source, Ref: "master", Mod: "example.com/layout"}
	if err := Project("app", "example.com/app", layout); err != nil {
		t.Fatalf("first Project() error = %v", err)
	}
	err := Project("app", "example.com/app", layout)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second Project() error = %v, want 'already exists'", err)
	}
}

func TestEnsureGitIdentityRequiresConfiguredIdentity(t *testing.T) {
	// Blank out every identity source: env vars, user config, and system config.
	t.Setenv("GIT_AUTHOR_NAME", "")
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_COMMITTER_NAME", "")
	t.Setenv("GIT_COMMITTER_EMAIL", "")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	if err := ensureGitIdentity(); err == nil {
		t.Fatal("ensureGitIdentity() error = nil, want missing-identity error")
	} else if !strings.Contains(err.Error(), "git commit identity") {
		t.Fatalf("ensureGitIdentity() error = %v, want actionable identity message", err)
	}
}

func TestProjectCustomLayoutWithoutBufFilesSucceeds(t *testing.T) {
	// A custom layout is not required to ship buf.gen.yaml / buf.binding.yaml;
	// missing related files must not abort project creation.
	setGitIdentity(t)
	source := filepath.Join(t.TempDir(), "source")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatalf("mkdir source: %v", err)
	}
	writeFixtureFile(t, filepath.Join(source, "go.mod"), "module example.com/custom\n\ngo 1.26.0\n")
	writeFixtureFile(t, filepath.Join(source, "main.go"), "package main\n\nfunc main() {}\n")
	writeFixtureFile(t, filepath.Join(source, "Makefile"), "init:\n\t@true\n")
	runGit(t, source, "init", "-b", "main")
	runGit(t, source, "add", ".")
	runGit(t, source, "-c", "user.name=Sphere Test", "-c", "user.email=sphere@example.com", "commit", "-m", "fixture")

	workspace := t.TempDir()
	t.Chdir(workspace)
	layout := &TemplateLayout{Name: "custom", Source: source, Ref: "main", Mod: "example.com/custom"}
	if err := Project("generated", "example.com/project", layout); err != nil {
		t.Fatalf("Project() without buf files error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "generated", "go.mod")); err != nil {
		t.Fatalf("generated project missing: %v", err)
	}
}

func TestProjectReportsLayoutValidationDetail(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	err := Project("app", "example.com/app", &TemplateLayout{Mod: "example.com/x"})
	if err == nil || !strings.Contains(err.Error(), "zip layouts require uri and path") {
		t.Fatalf("Project() error = %v, want detailed validation error", err)
	}
}

func TestCopyDirContentsPreservesFilesDirsAndSymlinks(t *testing.T) {
	source := t.TempDir()
	if err := os.MkdirAll(filepath.Join(source, "sub", "deep"), 0o755); err != nil {
		t.Fatalf("mkdir fixture: %v", err)
	}
	writeFixtureFile(t, filepath.Join(source, "a.txt"), "hello")
	writeFixtureFile(t, filepath.Join(source, "sub", "b.txt"), "world")
	writeFixtureFile(t, filepath.Join(source, "sub", "deep", "c.txt"), "!")
	target := filepath.Join(t.TempDir(), "out")
	if err := copyDirContents(source, target); err != nil {
		t.Fatalf("copyDirContents() error = %v", err)
	}
	for _, rel := range []string{"a.txt", "sub/b.txt", "sub/deep/c.txt"} {
		data, err := os.ReadFile(filepath.Join(target, rel))
		if err != nil {
			t.Errorf("read copied %s: %v", rel, err)
			continue
		}
		if len(data) == 0 {
			t.Errorf("copied %s is empty", rel)
		}
	}
}
