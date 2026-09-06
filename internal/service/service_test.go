package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenServiceGolang(t *testing.T) {
	out, err := GenServiceGolang("user", "dash.v1", "github.com/go-sphere/sphere-layout", "")
	if err != nil {
		t.Fatalf("GenServiceGolang() error = %v", err)
	}
	for _, want := range []string{
		`dashv1 "github.com/go-sphere/sphere-layout/api/dash/v1"`,
		"var _ dashv1.UserServiceHTTPServer = (*Service)(nil)",
		"func (s *Service) CreateUser(ctx context.Context",
		"func (s *Service) ListUsers(ctx context.Context",
		`ent/user"`,
		"entbind.IgnoreField(user.FieldID)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GenServiceGolang() output missing %q", want)
		}
	}
	if !strings.HasPrefix(out, "package dash\n") {
		t.Errorf("GenServiceGolang() unexpected header:\n%s", out[:40])
	}
}

func TestGenServiceGolangCamelizesAndPluralizes(t *testing.T) {
	out, err := GenServiceGolang("category", "shop.v1", "example.com/x", "")
	if err != nil {
		t.Fatalf("GenServiceGolang() error = %v", err)
	}
	for _, want := range []string{
		"func (s *Service) CreateCategory(",
		"func (s *Service) ListCategories(",
		"func (s *Service) UpdateCategory(",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GenServiceGolang() output missing %q", want)
		}
	}
}

func TestGenServiceProto(t *testing.T) {
	out, err := GenServiceProto("user", "dash.v1", "")
	if err != nil {
		t.Fatalf("GenServiceProto() error = %v", err)
	}
	for _, want := range []string{
		"package dash.v1;",
		"service UserService {",
		"rpc ListUsers(",
		`get: "/api/user/list"`,
		`post: "/api/user/create"`,
		`get: "/api/user/detail/{id}"`,
		`delete: "/api/user/delete/{id}"`,
		"message CreateUserRequest {",
		"entpb.User user = 1;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GenServiceProto() output missing %q", want)
		}
	}
}

func TestGenServiceGolangSnakeCaseMultiWordOutsideProject(t *testing.T) {
	// Without a schema directory the name is normalised purely by inflection;
	// a separated multi-word name must produce the correct PascalCase entity
	// and the ent sub-package import ent uses (lower-cased, no separators).
	out, err := GenServiceGolang("key_value_store", "dash.v1", "github.com/go-sphere/sphere-layout", "")
	if err != nil {
		t.Fatalf("GenServiceGolang() error = %v", err)
	}
	for _, want := range []string{
		"func (s *Service) CreateKeyValueStore(",
		"func (s *Service) ListKeyValueStores(",
		"entbind.CreateKeyValueStore(",
		`ent/keyvaluestore"`,
		"entbind.IgnoreField(keyvaluestore.FieldID)",
		"var _ dashv1.KeyValueStoreServiceHTTPServer = (*Service)(nil)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GenServiceGolang() output missing %q", want)
		}
	}
}

func TestGenServiceProtoSnakeCaseMultiWord(t *testing.T) {
	out, err := GenServiceProto("key_value_store", "dash.v1", "")
	if err != nil {
		t.Fatalf("GenServiceProto() error = %v", err)
	}
	for _, want := range []string{
		"service KeyValueStoreService {",
		"rpc ListKeyValueStores(",
		`get: "/api/key-value-store/list"`,
		"repeated entpb.KeyValueStore key_value_stores = 1;",
		"entpb.KeyValueStore key_value_store = 1;",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GenServiceProto() output missing %q", want)
		}
	}
}

// writeSchemaDir creates a temporary sphere-style schema directory containing
// the given type declarations that embed ent.Schema.
func writeSchemaDir(t *testing.T, types ...string) string {
	t.Helper()
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("package schema\n\nimport \"entgo.io/ent\"\n\n")
	for _, typ := range types {
		b.WriteString("type " + typ + " struct {\n\tent.Schema\n}\n\n")
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.go"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestNormalizeEntity_MatchesSchemaType(t *testing.T) {
	schemaDir := writeSchemaDir(t, "KeyValueStore", "AdminSession", "User")

	cases := []struct {
		name string
		want string
	}{
		{"keyvaluestore", "KeyValueStore"},
		{"key_value_store", "KeyValueStore"},
		{"KeyValueStore", "KeyValueStore"},
		{"adminsession", "AdminSession"},
		{"admin_session", "AdminSession"},
		{"users", "User"}, // plural name resolves via singular form
		{"unknown_thing", ""},
	}
	for _, c := range cases {
		got, ok := NormalizeEntity(c.name, schemaDir)
		if c.want == "" {
			if ok {
				t.Errorf("NormalizeEntity(%q) unexpectedly resolved to %q", c.name, got.TypeName)
			}
			continue
		}
		if !ok {
			t.Errorf("NormalizeEntity(%q) did not resolve", c.name)
			continue
		}
		if got.TypeName != c.want {
			t.Errorf("NormalizeEntity(%q).TypeName = %q, want %q", c.name, got.TypeName, c.want)
		}
	}
}

func TestGenServiceGolang_UndividedNameResolvesViaSchema(t *testing.T) {
	schemaDir := writeSchemaDir(t, "KeyValueStore")
	// The undivided lowercase name would inflect to the wrong "Keyvaluestore";
	// with a schema directory present it must resolve to the real type.
	out, err := GenServiceGolang("keyvaluestore", "dash.v1", "github.com/go-sphere/sphere-layout", schemaDir)
	if err != nil {
		t.Fatalf("GenServiceGolang() error = %v", err)
	}
	for _, want := range []string{
		"func (s *Service) CreateKeyValueStore(",
		"var _ dashv1.KeyValueStoreServiceHTTPServer = (*Service)(nil)",
		`ent/keyvaluestore"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("GenServiceGolang() output missing %q", want)
		}
	}
	if strings.Contains(out, "Keyvaluestore") {
		t.Errorf("GenServiceGolang() produced wrong undivided casing:\n%s", out)
	}
}
