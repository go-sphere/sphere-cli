package renamer

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ModulePath reads the module directive from the go.mod file in dir. It
// returns an error when dir has no readable go.mod or the directive is
// missing, so callers can fall back to asking the user.
func ModulePath(dir string) (string, error) {
	content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "module ") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			return "", fmt.Errorf("invalid module directive in %s", filepath.Join(dir, "go.mod"))
		}
		return strings.Trim(fields[1], `"`), nil
	}
	return "", errors.New("module directive not found in go.mod")
}

func RenameDirModule(oldModule, newModule string, dir string) error {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Symlinks are skipped: a layout from an untrusted source could point
		// one outside the project, and rewriting it would edit that target.
		if d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
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
		return fmt.Errorf("parse %s: %w", path, err)
	}
	ast.Inspect(node, func(n ast.Node) bool {
		importSpec, ok := n.(*ast.ImportSpec)
		if ok {
			goPath := strings.Trim(importSpec.Path.Value, `"`)
			// Only rewrite imports inside oldModule: the module itself or its
			// subpackages. A plain prefix match would also rewrite a sibling
			// module such as github.com/a/foobar when renaming github.com/a/foo.
			if goPath == oldModule || strings.HasPrefix(goPath, oldModule+"/") {
				newPath := newModule + strings.TrimPrefix(goPath, oldModule)
				importSpec.Path.Value = `"` + newPath + `"`
			}
		}
		return true
	})
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer func() {
		_ = file.Close()
	}()
	err = printer.Fprint(file, files, node)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func renameGoModuleFile(oldModule, newModule, modPath string) error {
	if isSymlink(modPath) {
		return fmt.Errorf("go.mod must not be a symlink: %s", modPath)
	}
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

// replaceFileContent replaces old with new in filePath. Symlinks are left
// untouched for the same reason RenameDirModule skips them.
func replaceFileContent(old, new, filePath string) error {
	if isSymlink(filePath) {
		return nil
	}
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	replaced := strings.ReplaceAll(string(content), old, new)
	return os.WriteFile(filePath, []byte(replaced), 0o644)
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&fs.ModeSymlink != 0
}
