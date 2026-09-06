package cmd

import "testing"

func TestValidateProjectName(t *testing.T) {
	valid := []string{"my-project", "blog", "app1", "Hello.World", "foo_bar"}
	for _, name := range valid {
		if err := validateProjectName(name); err != nil {
			t.Errorf("validateProjectName(%q) error = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",          // empty is reported by --name check, but must not panic
		"  ",        // whitespace only
		" lead",     // leading whitespace
		"trail ",    // trailing whitespace
		"a/b",       // path separator
		`a\b`,       // Windows separator
		".",         // current dir
		"..",        // parent dir
		"../evil",   // traversal
		"a/../evil", // traversal via segment
	}
	for _, name := range invalid {
		if err := validateProjectName(name); err == nil {
			t.Errorf("validateProjectName(%q) error = nil, want error", name)
		}
	}
}
