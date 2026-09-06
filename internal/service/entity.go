package service

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-openapi/inflect"
)

func newRules() *inflect.Ruleset {
	return inflect.NewDefaultRuleset()
}

// Entity is the canonicalised entity identity shared by the proto and Go
// generators. All names are derived from a single PascalCase TypeName so the
// output stays consistent regardless of the separator style of the --name flag.
type Entity struct {
	// TypeName is the PascalCase entity/type name (e.g. KeyValueStore).
	TypeName string
	// FileName is the lower-cased name used for ent sub-package imports
	// (e.g. keyvaluestore).
	FileName string
	// FieldName is the snake_case proto/JSON field name of the entity
	// (e.g. key_value_store).
	FieldName string
	// Route is the kebab-case route segment (e.g. key-value-store).
	Route string
	// Plural is the inflected plural of TypeName (e.g. KeyValueStores).
	Plural string
	// PluralField is the snake_case plural field name used in list responses
	// (e.g. key_value_stores).
	PluralField string
}

// buildEntity derives every name from a resolved PascalCase type name.
func buildEntity(typeName string) Entity {
	rules := newRules()
	return Entity{
		TypeName: typeName,
		// ent generates one sub-package per type named by the lower-cased type
		// name with no separators (KeyValueStore -> keyvaluestore).
		FileName:    strings.ToLower(typeName),
		FieldName:   rules.Underscore(typeName),
		Route:       rules.Dasherize(typeName),
		Plural:      rules.Pluralize(typeName),
		PluralField: rules.Underscore(rules.Pluralize(typeName)),
	}
}

// NormalizeEntity resolves a CLI-supplied service name to an Entity.
//
// When schemaDir names an existing sphere schema directory, the name is matched
// case- and separator-insensitively against the real ent schema types there
// (also trying the singular form), so "--name keyvaluestore" resolves to the
// KeyValueStore schema instead of producing the un-compilable "Keyvaluestore".
// The second return value reports whether such a schema-backed match happened.
//
// When no schema match is possible (outside a project, or the name does not
// correspond to a schema), the name is PascalCased through inflection.
// Separator-rich inputs (snake_case, kebab-case) split reliably; a single
// undivided lowercase word is treated as one word (so "keyvaluestore" would
// produce "Keyvaluestore" — pass the schema-separated form, or run inside the
// project, to disambiguate).
func NormalizeEntity(name, schemaDir string) (Entity, bool) {
	if resolved, ok := ResolveSchemaEntity(name, schemaDir); ok {
		return buildEntity(resolved), true
	}
	return buildEntity(newRules().Camelize(name)), false
}

// canonicalEntityKey reduces an entity name to a case/separator-insensitive
// form so that "KeyValueStore", "key_value_store", "key-value-store" and
// "keyvaluestore" all compare equal.
func canonicalEntityKey(name string) string {
	var b []byte
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z':
			b = append(b, c)
		case c >= 'A' && c <= 'Z':
			b = append(b, c+'a'-'A')
		case c >= '0' && c <= '9':
			b = append(b, c)
		}
	}
	return string(b)
}

// singularOf returns the singular form of name when inflect determines one;
// otherwise name is returned unchanged.
func singularOf(name string) string {
	if out := newRules().Singularize(name); out != "" {
		return out
	}
	return name
}

// SchemaEntityTypes scans a sphere project's schema directory and returns the
// sorted list of ent schema type names declared there (types embedding
// ent.Schema). It returns an empty slice when the directory does not exist or
// cannot be parsed, so callers outside a project degrade gracefully.
func SchemaEntityTypes(schemaDir string) []string {
	entries, err := os.ReadDir(schemaDir)
	if err != nil {
		return nil
	}
	var types []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" {
			continue
		}
		types = append(types, schemaTypesInFile(filepath.Join(schemaDir, entry.Name()))...)
	}
	sort.Strings(types)
	return types
}

func schemaTypesInFile(path string) []string {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil
	}
	var types []string
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if embedsEntSchema(st) {
				types = append(types, ts.Name.Name)
			}
		}
	}
	return types
}

// embedsEntSchema reports whether the struct embeds ent.Schema directly.
func embedsEntSchema(st *ast.StructType) bool {
	for _, f := range st.Fields.List {
		if len(f.Names) != 0 || f.Type == nil {
			continue
		}
		sel, ok := f.Type.(*ast.SelectorExpr)
		if !ok {
			continue
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "ent" || sel.Sel.Name != "Schema" {
			continue
		}
		return true
	}
	return false
}

// ResolveSchemaEntity maps a CLI-supplied service name to the canonical ent
// schema type name present in schemaDir. The name is matched case- and
// separator-insensitively against both the raw name and its singular form. It
// returns ok=false when no schema type matches.
func ResolveSchemaEntity(name, schemaDir string) (string, bool) {
	types := SchemaEntityTypes(schemaDir)
	if len(types) == 0 {
		return "", false
	}
	byKey := make(map[string]string, len(types))
	for _, t := range types {
		byKey[canonicalEntityKey(t)] = t
	}
	candidates := []string{name, singularOf(name)}
	for _, candidate := range candidates {
		if t, ok := byKey[canonicalEntityKey(candidate)]; ok {
			return t, true
		}
	}
	return "", false
}
