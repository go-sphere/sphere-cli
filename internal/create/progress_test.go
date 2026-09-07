package create

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestParseGitProgress(t *testing.T) {
	tests := []struct {
		line        string
		wantPhase   string
		wantPercent float64
		wantOK      bool
	}{
		{"Cloning into 'layout'...", "", 0, false},
		{"remote: Enumerating objects: 123, done.", "Contacting remote", 0.01, true},
		{"Receiving objects:   0% (1/273)", "Receiving objects", 0.02, true},
		{"Receiving objects:  45% (123/273)", "Receiving objects", 0.02 + 0.83*0.45, true},
		{"Receiving objects: 100% (273/273), done.", "Receiving objects", 0.85, true},
		{"Resolving deltas: 100% (10/10), done.", "Resolving deltas", 0.95, true},
		{"Updating files:  80% (8/10)", "Updating files", 0.95 + 0.05*0.8, true},
		{"Updating files: 100% (10/10), done.", "Updating files", 1, true},
		{"Branch 'master' set up to track remote branch.", "", 0, false},
	}
	for _, tt := range tests {
		phase, percent, ok := ParseGitProgress(tt.line)
		if ok != tt.wantOK {
			t.Errorf("ParseGitProgress(%q) ok = %v, want %v", tt.line, ok, tt.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if phase != tt.wantPhase {
			t.Errorf("ParseGitProgress(%q) phase = %q, want %q", tt.line, phase, tt.wantPhase)
		}
		if !almostEqual(percent, tt.wantPercent) {
			t.Errorf("ParseGitProgress(%q) percent = %v, want %v", tt.line, percent, tt.wantPercent)
		}
	}
}

func TestLayoutListWithFallbackReturnsCatalog(t *testing.T) {
	layouts, _, err := LayoutListWithFallback()
	if err != nil {
		t.Fatalf("LayoutListWithFallback() error = %v", err)
	}
	if len(layouts) == 0 {
		t.Fatal("LayoutListWithFallback() returned an empty catalog")
	}
}

func TestValidateProjectName(t *testing.T) {
	for _, name := range []string{"my-project", "blog", "app1", "foo_bar"} {
		if err := ValidateProjectName(name); err != nil {
			t.Errorf("ValidateProjectName(%q) error = %v, want nil", name, err)
		}
	}
	for _, name := range []string{"", "  ", " lead", "trail ", "a/b", `a\b`, ".", "..", "../evil"} {
		if err := ValidateProjectName(name); err == nil {
			t.Errorf("ValidateProjectName(%q) error = nil, want error", name)
		}
	}
}
