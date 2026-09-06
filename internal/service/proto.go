package service

import (
	_ "embed"
	"strings"
	"text/template"
)

type protoConfig struct {
	PackageName string
	// ServiceName is the PascalCase entity name (KeyValueStore).
	ServiceName string
	// EntityField is the snake_case singular field name (key_value_store).
	EntityField string
	// EntityPluralField is the snake_case plural list field name
	// (key_value_stores).
	EntityPluralField string
	// RouteName is the kebab-case route segment (key-value-store).
	RouteName string
}

//go:embed proto.tmpl
var protoTemplate string

// GenServiceProto generates a CRUD service proto for the entity named by
// `name`. When schemaDir points at an existing sphere schema directory, the
// name is matched against the real schema types first (see NormalizeEntity);
// otherwise inflection is used.
func GenServiceProto(name, pkg, schemaDir string) (string, error) {
	entity, _ := NormalizeEntity(name, schemaDir)

	conf := protoConfig{
		PackageName:       pkg,
		ServiceName:       entity.TypeName,
		EntityField:       entity.FieldName,
		EntityPluralField: entity.PluralField,
		RouteName:         entity.Route,
	}

	tmpl, err := template.New("proto").Funcs(template.FuncMap{
		"plural": func(s string) string { return newRules().Pluralize(s) },
	}).Parse(protoTemplate)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	if err := tmpl.Execute(&sb, conf); err != nil {
		return "", err
	}

	return sb.String(), nil
}
