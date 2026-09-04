package create

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestLayoutBuiltIn(t *testing.T) {
	tests := []struct {
		name string
		want TemplateLayout
	}{
		{
			name: "",
			want: TemplateLayout{
				URI:  "https://github.com/go-sphere/sphere-layout/archive/refs/heads/master.zip",
				Mod:  "github.com/go-sphere/sphere-layout",
				Path: "sphere-layout-master",
			},
		},
		{
			name: "bun",
			want: TemplateLayout{
				URI:  "https://github.com/go-sphere/sphere-bun-layout/archive/refs/heads/master.zip",
				Mod:  "github.com/go-sphere/sphere-bun-layout",
				Path: "sphere-bun-layout-master",
			},
		},
		{
			name: "simple",
			want: TemplateLayout{
				URI:  "https://github.com/go-sphere/sphere-simple-layout/archive/refs/heads/master.zip",
				Mod:  "github.com/go-sphere/sphere-simple-layout",
				Path: "sphere-simple-layout-master",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Layout(tt.name)
			if err != nil {
				t.Fatalf("Layout(%q) error = %v", tt.name, err)
			}
			if !reflect.DeepEqual(*got, tt.want) {
				t.Errorf("Layout(%q) = %#v, want %#v", tt.name, *got, tt.want)
			}
		})
	}
}

func TestLayoutRemote(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		want    *TemplateLayout
		wantErr string
	}{
		{
			name:   "valid",
			status: http.StatusOK,
			body:   `{"uri":"https://example.com/layout.zip","mod":"example.com/layout","path":"layout-main"}`,
			want:   &TemplateLayout{URI: "https://example.com/layout.zip", Mod: "example.com/layout", Path: "layout-main"},
		},
		{
			name:    "HTTP error",
			status:  http.StatusBadGateway,
			wantErr: "failed to fetch layout configuration: 502 Bad Gateway",
		},
		{
			name:    "missing required field",
			status:  http.StatusOK,
			body:    `{"uri":"https://example.com/layout.zip","mod":"example.com/layout"}`,
			wantErr: "invalid layout configuration",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			t.Cleanup(server.Close)

			got, err := Layout(server.URL)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Layout() error = nil, want %q", tt.wantErr)
				}
				if gotErr := err.Error(); gotErr != tt.wantErr {
					t.Errorf("Layout() error = %q, want %q", gotErr, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Layout() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Layout() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestLayoutRemoteRejectsMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"uri":`))
	}))
	t.Cleanup(server.Close)

	if _, err := Layout(server.URL); err == nil {
		t.Fatal("Layout() error = nil, want JSON decoding error")
	}
}
