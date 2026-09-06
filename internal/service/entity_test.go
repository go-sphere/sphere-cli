package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaEntityTypes(t *testing.T) {
	dir := writeSchemaDir(t, "KeyValueStore", "User")
	got := SchemaEntityTypes(dir)
	if len(got) != 2 || got[0] != "KeyValueStore" || got[1] != "User" {
		t.Fatalf("SchemaEntityTypes() = %v, want sorted [KeyValueStore User]", got)
	}
}

func TestSchemaEntityTypes_MissingDir(t *testing.T) {
	if got := SchemaEntityTypes(filepath.Join(t.TempDir(), "nope")); got != nil {
		t.Fatalf("SchemaEntityTypes() on missing dir = %v, want nil", got)
	}
}

func TestSchemaEntityTypes_SkipsNonSchemaTypes(t *testing.T) {
	dir := t.TempDir()
	content := `package schema

import "entgo.io/ent"

type User struct {
	ent.Schema
}

type helper struct {
	Name string
}
`
	if err := os.WriteFile(filepath.Join(dir, "schema.go"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := SchemaEntityTypes(dir)
	if len(got) != 1 || got[0] != "User" {
		t.Fatalf("SchemaEntityTypes() = %v, want [User] only", got)
	}
}

func TestCanonicalEntityKey(t *testing.T) {
	for _, pair := range [][2]string{
		{"KeyValueStore", "keyvaluestore"},
		{"key_value_store", "keyvaluestore"},
		{"key-value-store", "keyvaluestore"},
		{"User", "user"},
	} {
		if canonicalEntityKey(pair[0]) != canonicalEntityKey(pair[1]) {
			t.Errorf("canonicalEntityKey(%q) != canonicalEntityKey(%q)", pair[0], pair[1])
		}
	}
}

func TestBuildEntityNames(t *testing.T) {
	e := buildEntity("KeyValueStore")
	if e.TypeName != "KeyValueStore" || e.FileName != "keyvaluestore" ||
		e.FieldName != "key_value_store" || e.Route != "key-value-store" ||
		e.Plural != "KeyValueStores" || e.PluralField != "key_value_stores" {
		t.Fatalf("buildEntity(KeyValueStore) = %+v", e)
	}
}
