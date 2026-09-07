package service

import (
	_ "embed"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type serviceConfig struct {
	ServiceName     string
	ServiceFileName string
	PluralName      string
	PackagePath     string
	ServicePackage  string
	BizPackagePath  string
}

//go:embed service.tmpl
var serviceTemplate string

// DetectSchemaDir resolves the sphere schema directory relative to dir when it
// exists, so generators can match service names against real ent schema types.
// Empty is returned outside a project (or when no known schema directory is
// found), in which case the generators fall back to inflection-based naming.
func DetectSchemaDir(dir string) string {
	for _, candidate := range []string{
		"internal/pkg/database/schema",
		"internal/ent/schema",
		"ent/schema",
		"schema",
	} {
		joined := filepath.Join(dir, candidate)
		if info, err := os.Stat(joined); err == nil && info.IsDir() {
			if abs, err := filepath.Abs(joined); err == nil {
				return abs
			}
			return joined
		}
	}
	return ""
}

// GenServiceGolang generates a service implementation skeleton for the entity
// named by `name`. When schemaDir points at an existing sphere schema
// directory, the name is matched against the real schema types first (see
// NormalizeEntity); otherwise inflection is used.
func GenServiceGolang(name, pkg, mod, schemaDir string) (string, error) {
	entity, _ := NormalizeEntity(name, schemaDir)

	conf := serviceConfig{
		ServiceName:     entity.TypeName,
		ServiceFileName: entity.FileName,
		PluralName:      entity.Plural,
		PackagePath:     strings.Join(strings.Split(pkg, "."), "/"),
		ServicePackage:  strings.ReplaceAll(pkg, ".", ""),
		BizPackagePath:  mod,
	}

	tmpl, err := template.New("service").Parse(serviceTemplate)
	if err != nil {
		return "", err
	}

	var file strings.Builder
	if err := tmpl.Execute(&file, conf); err != nil {
		return "", err
	}

	source, err := format.Source([]byte(file.String()))
	if err != nil {
		return "", err
	}
	return string(source), nil
}
