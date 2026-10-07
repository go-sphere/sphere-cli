package create

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

// BuiltInLayouts returns the layouts compiled into the CLI. The interactive
// wizard falls back to this list when the remote layout list is unreachable.
func BuiltInLayouts() []*LayoutItem {
	names := make([]string, 0, len(templateLayouts))
	for name := range templateLayouts {
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	items := make([]*LayoutItem, 0, len(names))
	for _, name := range names {
		items = append(items, &LayoutItem{Name: name, Description: "Built-in layout"})
	}
	return items
}

// Options describes a project creation run. Name and Module are required,
// Layout may be nil to select the default layout, and the boolean switches
// control the optional post-scaffold steps.
type Options struct {
	Name     string
	Module   string
	Layout   *TemplateLayout
	InitGit  bool
	InitDeps bool
}

// ProgressEvent reports what the creation pipeline is currently doing. The
// interactive UI renders it as an overall progress bar; the CLI prints it as
// plain log lines.
type ProgressEvent struct {
	// Step is the zero-based index of the current pipeline step.
	Step int
	// TotalSteps is the number of steps in the pipeline.
	TotalSteps int
	// Label is a human-readable description of the current step.
	Label string
	// Percent is the progress inside the current step, 0..1. Negative values
	// mean the step has no measurable progress (spinner only).
	Percent float64
	// Detail is the most recent command or status line of the current step.
	Detail string
	// Done marks the whole pipeline as finished.
	Done bool
}

// Reporter receives progress events; it must be safe for concurrent use.
type Reporter func(ProgressEvent)

const (
	stepDownload = iota
	stepRenameModule
	stepWriteLock
	stepInitGit
	stepInstallDeps
	stepMoveOutput
	stepCount
)

// Project creates a project with default options: git init and dependency
// installation enabled. It is kept for scripted usage and tests.
func Project(name, mod string, layout *TemplateLayout) error {
	return Create(Options{
		Name:     name,
		Module:   mod,
		Layout:   layout,
		InitGit:  true,
		InitDeps: true,
	}, nil)
}

// Create runs the full project creation pipeline, reporting progress through
// the optional reporter.
func Create(opts Options, report Reporter) error {
	if err := validateLayout(opts.Layout); err != nil {
		return fmt.Errorf("invalid layout: %w", err)
	}
	targetDir, err := filepath.Abs(filepath.Join(".", opts.Name))
	if err != nil {
		return err
	}
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("target directory already exists: %s", targetDir)
	}

	emit := func(step int, label string, percent float64, detail string) {
		if report == nil {
			return
		}
		report(ProgressEvent{Step: step, TotalSteps: stepCount, Label: label, Percent: percent, Detail: detail})
	}

	layoutDir, cleanup, revision, err := materializeLayout(opts.Layout, func(percent float64, detail string) {
		emit(stepDownload, "Downloading template", percent, detail)
	})
	if err != nil {
		return err
	}
	defer cleanup()
	emit(stepDownload, "Downloading template", 1, "")

	emit(stepRenameModule, "Renaming Go module", -1, opts.Module)
	if err := renameGoModule(opts.Layout.Mod, opts.Module, layoutDir); err != nil {
		return err
	}
	emit(stepRenameModule, "Renaming Go module", 1, "")

	if revision != "" {
		emit(stepWriteLock, "Recording template revision", -1, revision[:min(12, len(revision))])
		if err := writeLayoutLock(layoutDir, opts.Layout, revision); err != nil {
			return err
		}
	}
	emit(stepWriteLock, "Recording template revision", 1, "")

	if opts.InitGit {
		emit(stepInitGit, "Initializing git repository", -1, "git init")
		if err := ensureGitIdentity(); err != nil {
			return err
		}
		if err := execCommands(layoutDir, nil, []string{"git", "init"}); err != nil {
			return err
		}
		emit(stepInitGit, "Initializing git repository", 1, "")
	}

	if opts.InitDeps {
		emit(stepInstallDeps, "Installing dependencies", -1, "make init")
		if err := installDependencies(layoutDir); err != nil {
			return err
		}
		emit(stepInstallDeps, "Installing dependencies", 1, "")
	}

	// Commit last so files produced by dependency installation are included in
	// the initial commit.
	if opts.InitGit {
		emit(stepInitGit, "Creating initial commit", 0.5, "git add .")
		if err := execCommands(layoutDir, nil,
			[]string{"git", "add", "."},
			[]string{"git", "commit", "-m", "feat: Initial commit"},
		); err != nil {
			return err
		}
		emit(stepInitGit, "Creating initial commit", 1, "")
	}

	emit(stepMoveOutput, "Writing project files", -1, targetDir)
	if err := moveTempDirToTarget(layoutDir, targetDir); err != nil {
		return err
	}
	emit(stepMoveOutput, "Writing project files", 1, "")

	if report != nil {
		report(ProgressEvent{Step: stepCount, TotalSteps: stepCount, Done: true})
	}
	return nil
}

func materializeLayout(layout *TemplateLayout, onProgress func(percent float64, detail string)) (string, func(), string, error) {
	if layout.Source != "" {
		tempDir, err := os.MkdirTemp("", "sphere-layout-")
		if err != nil {
			return "", func() {}, "", err
		}
		cleanup := func() { _ = os.RemoveAll(tempDir) }
		layoutDir := filepath.Join(tempDir, "layout")
		if _, err := cloneRepository(layout.Source, layout.Ref, tempDir, layoutDir, onProgress); err != nil {
			cleanup()
			return "", func() {}, "", err
		}
		revision, err := execCommand(layoutDir, nil, "git", "rev-parse", "HEAD")
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

// cloneRepository clones source at ref into layoutDir, streaming git progress
// to onProgress as a 0..1 percentage.
func cloneRepository(source, ref, dir, layoutDir string, onProgress func(percent float64, detail string)) (string, error) {
	args := []string{"clone", "--depth", "1", "--single-branch", "--branch", ref}
	if onProgress != nil {
		args = append(args, "--progress")
	}
	args = append(args, "--", source, layoutDir)

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	stdout := &strings.Builder{}
	cmd.Stdout = stdout
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	if onProgress != nil {
		reader := bufio.NewReader(stderrPipe)
		for {
			line, err := reader.ReadString('\n')
			for _, part := range strings.Split(strings.TrimRight(line, "\r\n"), "\r") {
				if phase, percent, ok := ParseGitProgress(part); ok {
					onProgress(percent, phase)
				}
			}
			if err != nil {
				break
			}
		}
	} else {
		_, _ = io.Copy(io.Discard, stderrPipe)
	}
	if err := cmd.Wait(); err != nil {
		return stdout.String(), err
	}
	return stdout.String(), nil
}

// ParseGitProgress extracts a phase name and normalized 0..1 progress from a
// git clone progress line such as "Receiving objects:  45% (123/273)".
// Cloning weighs 2%, receiving 83%, resolving deltas 10%, and updating files
// the remaining 5%.
func ParseGitProgress(line string) (string, float64, bool) {
	phases := []struct {
		name  string
		lo    float64
		span  float64
		scale float64
	}{
		{name: "Cloning", lo: 0.00, span: 0.02, scale: 1},
		{name: "Receiving objects", lo: 0.02, span: 0.83, scale: 1},
		{name: "Resolving deltas", lo: 0.85, span: 0.10, scale: 1},
		{name: "Checking objects", lo: 0.85, span: 0.10, scale: 1},
		{name: "Updating files", lo: 0.95, span: 0.05, scale: 1},
		{name: "Checking out files", lo: 0.95, span: 0.05, scale: 1},
	}
	for _, phase := range phases {
		rest, ok := cutPrefix(line, phase.name+":")
		if !ok {
			continue
		}
		percent, ok := leadingPercent(rest)
		if !ok {
			return phase.name, phase.lo, true
		}
		return phase.name, phase.lo + phase.span*(percent/100), true
	}
	if strings.HasPrefix(line, "remote:") {
		return "Contacting remote", 0.01, true
	}
	return "", 0, false
}

func cutPrefix(s, prefix string) (string, bool) {
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	return strings.TrimPrefix(s, prefix), true
}

func leadingPercent(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	percentIndex := strings.IndexByte(s, '%')
	if percentIndex < 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(s[:percentIndex]), 64)
	if err != nil {
		return 0, false
	}
	return value, true
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

// ValidateProjectName rejects names that would create the project outside the
// current directory or silently produce a different directory than the name
// the user typed (path separators, "." / ".." traversal, stray whitespace).
func ValidateProjectName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("project name must not be empty or whitespace")
	}
	if name != strings.TrimSpace(name) {
		return errors.New("project name must not start or end with whitespace")
	}
	if strings.ContainsAny(name, `/\`) {
		return errors.New("project name must be a single directory name without path separators")
	}
	if name == "." || name == ".." {
		return errors.New("project name must not be '.' or '..'")
	}
	return nil
}

func validateLayout(layout *TemplateLayout) error {
	if layout == nil || layout.Mod == "" {
		return errors.New("missing module")
	}
	if layout.Source != "" {
		if layout.Name == "" || layout.Ref == "" {
			return errors.New("git layouts require name and ref")
		}
		// A leading "-" would be parsed by git as an option, not a repository.
		if strings.HasPrefix(layout.Source, "-") {
			return errors.New("git layout source must not start with '-'")
		}
		return nil
	}
	if layout.URI == "" || layout.Path == "" {
		return errors.New("zip layouts require uri and path")
	}
	// Path is joined onto the extraction directory; it must stay inside it.
	if !filepath.IsLocal(layout.Path) {
		return errors.New("zip layout path must be a relative path inside the archive")
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

// LayoutListWithFallback returns the remote layout list, falling back to the
// built-in layouts when the remote list is unreachable. The second result
// reports whether the remote list was used.
func LayoutListWithFallback() ([]*LayoutItem, bool, error) {
	layouts, err := LayoutList()
	if err == nil && len(layouts) > 0 {
		return layouts, true, nil
	}
	return BuiltInLayouts(), false, nil
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

// ensureGitIdentity verifies that a commit identity is available before any
// commit is attempted, so users without a configured git identity get a clear
// message instead of an obscure failure from `git commit`.
func ensureGitIdentity() error {
	if _, err := execCommand("", nil, "git", "var", "GIT_COMMITTER_IDENT"); err != nil {
		return errors.New("git commit identity is not configured: set git user.name and user.email, or export GIT_AUTHOR_NAME/GIT_AUTHOR_EMAIL and GIT_COMMITTER_NAME/GIT_COMMITTER_EMAIL")
	}
	return nil
}

func renameGoModule(oldModName, newModName, target string) error {
	if err := renamer.RenameProjectModule(oldModName, newModName, target, []string{
		"buf.gen.yaml",
		"buf.binding.yaml",
	}, true); err != nil {
		return err
	}
	_, err := execCommand(target, nil, "go", "mod", "edit", "-module", newModName)
	return err
}

func installDependencies(target string) error {
	return execCommands(target, nil,
		[]string{"make", "init"},
		[]string{"go", "mod", "tidy"},
		[]string{"go", "fmt", "./..."},
	)
}

// execCommand runs a command quietly, capturing stdout for the caller and
// stderr for error reporting. Optional onOutput receives progress lines.
func execCommand(dir string, onDetail func(string), name string, arg ...string) (string, error) {
	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	var stdout strings.Builder
	var stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message != "" {
			return stdout.String(), fmt.Errorf("%s %s: %w\n%s", name, strings.Join(arg, " "), err, message)
		}
		return stdout.String(), fmt.Errorf("%s %s: %w", name, strings.Join(arg, " "), err)
	}
	if onDetail != nil {
		if line := lastNonEmptyLine(stdout.String()); line != "" {
			onDetail(line)
		}
	}
	return stdout.String(), nil
}

func lastNonEmptyLine(text string) string {
	text = strings.TrimRight(text, "\n\r \t")
	if index := strings.LastIndexAny(text, "\n"); index >= 0 {
		return text[index+1:]
	}
	return text
}

func execCommands(dir string, onDetail func(string), commands ...[]string) error {
	for _, cmd := range commands {
		if _, err := execCommand(dir, onDetail, cmd[0], cmd[1:]...); err != nil {
			return err
		}
	}
	return nil
}
