package create

import (
	"os"
	"testing"
)

func TestProject(t *testing.T) {
	err := Project("example", "example", &defaultTemplateLayout)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove("example")
}
