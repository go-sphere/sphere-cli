package create

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-sphere/sphere-cli/internal/renamer"
	"github.com/go-sphere/sphere-cli/internal/zip"
)

type TemplateLayout struct {
	Name   string `json:"name,omitempty"`
	Source string `json:"source,omitempty"`
	Ref    string `json:"ref,omitempty"`
	URI    string `json:"uri,omitempty"`
	Mod    string `json:"mod,omitempty"`
	Path   string `json:"path,omitempty"`
}

type LayoutLock struct {
	SchemaVersion  int    `json:"schema_version"`
	Name           string `json:"name"`
	Repository     string `json:"repository"`
	Ref            string `json:"ref"`
	UpstreamModule string `json:"upstream_module"`
	BaseRevision   string `json:"base_revision"`
}

type LayoutItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

var templateLayouts = map[string]*TemplateLayout{
	"": {
		Name:   "standard",
		Source: "https://github.com/go-sphere/sphere-layout.git",
		Ref:    "master",
		Mod:    "github.com/go-sphere/sphere-layout",
	},
	"standard": {
		Name:   "standard",
		Source: "https://github.com/go-sphere/sphere-layout.git",
		Ref:    "master",
		Mod:    "github.com/go-sphere/sphere-layout",
	},
	"bun": {
		Name:   "bun",
		Source: "https://github.com/go-sphere/sphere-bun-layout.git",
		Ref:    "master",
		Mod:    "github.com/go-sphere/sphere-bun-layout",
	},
	"simple": {
		Name:   "simple",
		Source: "https://github.com/go-sphere/sphere-simple-layout.git",
		Ref:    "master",
		Mod:    "github.com/go-sphere/sphere-simple-layout",
	},
	"telegram": {
		Name:   "telegram",
		Source: "https://github.com/go-sphere/sphere-telegram-layout.git",
		Ref:    "master",
		Mod:    "github.com/go-sphere/sphere-telegram-layout",
	},
}

func Project(name, mod string, layout *TemplateLayout) error {
	if err := validateLayout(layout); err != nil {
		return fmt.Errorf("invalid layout: %w", err)
	}
	targetDir, err := filepath.Abs(filepath.Join(".", name))
	if err != nil {
		return err
	}
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("target directory already exists: %s", targetDir)
	}

	layoutDir, cleanup, revision, err := materializeLayout(layout)
	if err != nil {
		return err
	}
	defer cleanup()

	err = renameGoModule(layout.Mod, mod, layoutDir)
	if err != nil {
		return err
	}

	if revision != "" {
		if err := writeLayoutLock(layoutDir, layout, revision); err != nil {
			return err
		}
	}

	if err := initGitRepo(layoutDir); err != nil {
		return err
	}

	err = moveTempDirToTarget(layoutDir, targetDir)
	if err != nil {
		return err
	}

	return nil
}

func materializeLayout(layout *TemplateLayout) (string, func(), string, error) {
	if layout.Source != "" {
		tempDir, err := os.MkdirTemp("", "sphere-layout-")
		if err != nil {
			return "", func() {}, "", err
		}
		cleanup := func() { _ = os.RemoveAll(tempDir) }
		layoutDir := filepath.Join(tempDir, "layout")
		if _, err := execCommand(tempDir, "git", "clone", "--depth", "1", "--single-branch", "--branch", layout.Ref, layout.Source, layoutDir); err != nil {
			cleanup()
			return "", func() {}, "", err
		}
		revision, err := execCommand(layoutDir, "git", "rev-parse", "HEAD")
		if err != nil {
			cleanup()
			return "", func() {}, "", err
		}
		if err := os.RemoveAll(filepath.Join(layoutDir, ".git")); err != nil {
			cleanup()
			return "", func() {}, "", err
		}
		return layoutDir, cleanup, strings.TrimSpace(revision), nil
	}

	tempDir, err := zip.DownloadAndUnzip(layout.URI)
	if err != nil {
		return "", func() {}, "", err
	}
	return filepath.Join(tempDir, layout.Path), func() { _ = os.RemoveAll(tempDir) }, "", nil
}

func writeLayoutLock(layoutDir string, layout *TemplateLayout, revision string) error {
	lock := LayoutLock{
		SchemaVersion:  1,
		Name:           layout.Name,
		Repository:     layout.Source,
		Ref:            layout.Ref,
		UpstreamModule: layout.Mod,
		BaseRevision:   revision,
	}
	raw, err := json.MarshalIndent(lock, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	lockDir := filepath.Join(layoutDir, ".sphere")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(lockDir, "layout.lock.json"), raw, 0o644)
}

func Layout(nameOrUri string) (*TemplateLayout, error) {
	if layout, ok := templateLayouts[nameOrUri]; ok {
		return layout, nil
	}
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(nameOrUri)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch layout configuration: " + resp.Status)
	}
	var layout TemplateLayout
	err = json.NewDecoder(resp.Body).Decode(&layout)
	if err != nil {
		return nil, err
	}
	if err := validateLayout(&layout); err != nil {
		return nil, errors.New("invalid layout configuration")
	}
	return &layout, nil
}

func validateLayout(layout *TemplateLayout) error {
	if layout == nil || layout.Mod == "" {
		return errors.New("missing module")
	}
	if layout.Source != "" {
		if layout.Name == "" || layout.Ref == "" {
			return errors.New("git layouts require name and ref")
		}
		return nil
	}
	if layout.URI == "" || layout.Path == "" {
		return errors.New("zip layouts require uri and path")
	}
	return nil
}

func LayoutList() ([]*LayoutItem, error) {
	client := http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get("https://go-sphere.github.io/layout/list.json")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to fetch layout list: " + resp.Status)
	}
	var layouts []*LayoutItem
	err = json.NewDecoder(resp.Body).Decode(&layouts)
	if err != nil {
		return nil, err
	}
	return layouts, nil
}

// moveTempDirToTarget moves source onto target. Layouts are materialized under
// the system temporary directory, which may live on a different filesystem than
// the target (e.g. TMPDIR on a separate mount); os.Rename then fails with
// EXDEV. Falling back to a recursive copy keeps project creation working
// regardless of where TMPDIR points.
func moveTempDirToTarget(source, target string) error {
	err := os.Rename(source, target)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	if err := copyDirContents(source, target); err != nil {
		return fmt.Errorf("move layout across devices: %w", err)
	}
	return nil
}

func copyDirContents(source, target string) error {
	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		dest := filepath.Join(target, rel)
		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, dest)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dest, data, info.Mode().Perm())
	})
}

func initGitRepo(target string) error {
	if err := ensureGitIdentity(); err != nil {
		return err
	}
	return execCommands(target,
		[]string{"git", "init"},
		[]string{"git", "add", "."},
		[]string{"git", "commit", "-m", "feat: Initial commit"},
	)
}

// ensureGitIdentity verifies that a commit identity is available before any
// commit is attempted, so users without a configured git identity get a clear
// message instead of an obscure failure from `git commit`.
func ensureGitIdentity() error {
	if _, err := execCommand("", "git", "var", "GIT_COMMITTER_IDENT"); err != nil {
		return errors.New("git commit identity is not configured: set git user.name and user.email, or export GIT_AUTHOR_NAME/GIT_AUTHOR_EMAIL and GIT_COMMITTER_NAME/GIT_COMMITTER_EMAIL")
	}
	return nil
}

func renameGoModule(oldModName, newModName, target string) error {
	log.Printf("rename module: %s -> %s", oldModName, newModName)
	if err := renamer.RenameProjectModule(oldModName, newModName, target, []string{
		"buf.gen.yaml",
		"buf.binding.yaml",
	}, true); err != nil {
		return err
	}
	err := execCommands(target,
		[]string{"go", "mod", "edit", "-module", newModName},
		[]string{"make", "init"},
		[]string{"go", "mod", "tidy"},
		[]string{"go", "fmt", "./..."},
	)
	if err != nil {
		return err
	}
	return nil
}

func execCommand(dir string, name string, arg ...string) (string, error) {
	log.Println(name, strings.Join(arg, " "))
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	var stdout strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), err
	}
	return stdout.String(), nil
}

func execCommands(dir string, commands ...[]string) error {
	for _, cmd := range commands {
		_, err := execCommand(dir, cmd[0], cmd[1:]...)
		if err != nil {
			return err
		}
	}
	return nil
}
