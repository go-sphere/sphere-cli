package create

import "testing"

func TestProject(t *testing.T) {
	err := Project("example", "example")
	if err != nil {
		t.Fatal(err)
	}
}
