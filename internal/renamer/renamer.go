package renamer

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func RenameDirModule(oldModule, newModule string, dir string) error {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			return RenameModule(oldModule, newModule, path)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return renameGoModuleFile(oldModule, newModule, filepath.Join(dir, "go.mod"))
}

func RenameProjectModule(oldModule, newModule, dir string, relatedFiles []string, ignoreMissingFiles bool) error {
	if err := RenameDirModule(oldModule, newModule, dir); err != nil {
		return err
	}
	for _, file := range relatedFiles {
		filePath := filepath.Join(dir, file)
		if err := replaceFileContent(oldModule, newModule, filePath); err != nil {
			if ignoreMissingFiles && errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func RenameModule(oldModule, newModule string, path string) error {
	files := token.NewFileSet()
	node, err := parser.ParseFile(files, path, nil, parser.ParseComments)
	if err != nil {
		log.Printf("parse file error: %v", err)
		return err
	}
	ast.Inspect(node, func(n ast.Node) bool {
		importSpec, ok := n.(*ast.ImportSpec)
		if ok {
			goPath := strings.Trim(importSpec.Path.Value, `"`)
			if strings.HasPrefix(goPath, oldModule) {
				newPath := strings.Replace(goPath, oldModule, newModule, 1)
				importSpec.Path.Value = `"` + newPath + `"`
			}
		}
		return true
	})
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		log.Printf("open file error: %v", err)
		return err
	}
	defer func() {
		_ = file.Close()
	}()
	err = printer.Fprint(file, files, node)
	if err != nil {
		log.Printf("write file error: %v", err)
		return err
	}
	return nil
}

func renameGoModuleFile(oldModule, newModule, modPath string) error {
	content, err := os.ReadFile(modPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("go.mod not found in target: %s", modPath)
		}
		return err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "module ") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return fmt.Errorf("invalid module directive in %s", modPath)
		}
		currentModule := strings.Trim(fields[1], `"`)
		if oldModule != "" && currentModule != oldModule {
			return fmt.Errorf("go.mod module mismatch: expected %q, got %q", oldModule, currentModule)
		}
		lines[i] = "module " + newModule
		found = true
		break
	}

	if !found {
		return fmt.Errorf("module directive not found in %s", modPath)
	}
	return os.WriteFile(modPath, []byte(strings.Join(lines, "\n")), 0o644)
}

func replaceFileContent(old, new, filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	replaced := strings.ReplaceAll(string(content), old, new)
	return os.WriteFile(filePath, []byte(replaced), 0o644)
}
