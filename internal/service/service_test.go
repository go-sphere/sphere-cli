package service

import (
	"strings"
	"testing"
)

func TestGenServiceGolang(t *testing.T) {
	out, err := GenServiceGolang("user", "dash.v1", "github.com/go-sphere/sphere-layout")
	if err != nil {
		t.Fatalf("GenServiceGolang() error = %v", err)
	}
	for _, want := range []string{
		`dashv1 "github.com/go-sphere/sphere-layout/api/dash/v1"`,
		"var _ dashv1.UserServiceHTTPServer = (*Service)(nil)",
		"func (s *Service) CreateUser(ctx context.Context",
		"func (s *Service) ListUsers(ctx context.Context",
		`ent/user"`,
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
	out, err := GenServiceGolang("category", "shop.v1", "example.com/x")
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
	out, err := GenServiceProto("user", "dash.v1")
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
