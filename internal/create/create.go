package create

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-sphere/sphere-cli/internal/renamer"
	"github.com/go-sphere/sphere-cli/internal/zip"
)

const (
	sphereModule                = "github.com/go-sphere/sphere"
	defaultProjectLayout        = "https://github.com/go-sphere/sphere-layout/archive/refs/heads/master.zip"
	defaultProjectLayoutModName = "github.com/go-sphere/sphere-layout"
	defaultLayoutPath           = "sphere-layout-master"
)

func Project(name, mod string) error {
	targetDir, err := filepath.Abs(filepath.Join(".", name))
	if err != nil {
		return err
	}

	// download and unzip the default project layout
	tempDir, err := zip.DownloadAndUnzip(defaultProjectLayout)
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()
	layoutDir := filepath.Join(tempDir, defaultLayoutPath)

	// init git repository
	err = initGitRepo(layoutDir)
	if err != nil {
		return err
	}

	// rename the Go module name
	err = renameGoModule(defaultProjectLayoutModName, mod, layoutDir)
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
