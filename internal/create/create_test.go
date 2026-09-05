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
