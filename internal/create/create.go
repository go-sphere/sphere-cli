package create

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-sphere/sphere-cli/internal/renamer"
	"github.com/go-sphere/sphere-cli/internal/zip"
)

type TemplateLayout struct {
	URI  string `json:"uri,omitempty"`
	Mod  string `json:"mod,omitempty"`
	Path string `json:"path,omitempty"`
}

type LayoutItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

var templateLayouts = map[string]*TemplateLayout{
	"": {
		URI:  "https://github.com/go-sphere/sphere-layout/archive/refs/heads/master.zip",
		Mod:  "github.com/go-sphere/sphere-layout",
		Path: "sphere-layout-master",
	},
	"bun": {
		URI:  "https://github.com/go-sphere/sphere-bun-layout/archive/refs/heads/master.zip",
		Mod:  "github.com/go-sphere/sphere-bun-layout",
		Path: "sphere-bun-layout-master",
	},
	"simple": {
		URI:  "https://github.com/go-sphere/sphere-simple-layout/archive/refs/heads/master.zip",
		Mod:  "github.com/go-sphere/sphere-simple-layout",
		Path: "sphere-simple-layout-master",
	},
}

func Project(name, mod string, layout *TemplateLayout) error {
	if layout == nil {
		return errors.New("invalid layout")
	}
	targetDir, err := filepath.Abs(filepath.Join(".", name))
	if err != nil {
		return err
	}

	// download and unzip the default project layout
	tempDir, err := zip.DownloadAndUnzip(layout.URI)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	layoutDir := filepath.Join(tempDir, layout.Path)

	// init git repository
	err = initGitRepo(layoutDir)
	if err != nil {
		return err
	}

	// rename the Go module name
	err = renameGoModule(layout.Mod, mod, layoutDir)
	if err != nil {
		return err
	}

	// Move the layout to the target directory
	err = moveTempDirToTarget(layoutDir, targetDir)
	if err != nil {
		return err
	}

	return nil
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
	if layout.URI == "" || layout.Mod == "" || layout.Path == "" {
		return nil, errors.New("invalid layout configuration")
	}
	return &layout, nil
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

func moveTempDirToTarget(source, target string) error {
	err := os.Rename(source, target)
	if err != nil {
		return err
	}
	return nil
}

func initGitRepo(target string) error {
	return execCommands(target,
		[]string{"git", "init"},
		[]string{"git", "add", "."},
		[]string{"git", "commit", "-m", "feat: Initial commit"},
	)
}

func renameGoModule(oldModName, newModName, target string) error {
	log.Printf("rename module: %s -> %s", oldModName, newModName)
	err := renamer.RenameDirModule(oldModName, newModName, target)
	if err != nil {
		return err
	}
	files := []string{
		"buf.gen.yaml",
		"buf.binding.yaml",
	}
	for _, file := range files {
		e := replaceFileContent(oldModName, newModName, filepath.Join(target, file))
		if e != nil {
			return e
		}
	}
	err = execCommands(target,
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
	return stdout.String(), cmd.Run()
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

func replaceFileContent(old, new, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	replacer := strings.NewReplacer(old, new)
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	_, err = replacer.WriteString(file, string(content))
	return err
}
