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

func TestLayout(t *testing.T) {
	layout, err := Layout("https://go-sphere.github.io/layout/simple.json")
	if err != nil {
		t.Fatal(err)
	}
	err = Project("example", "example", layout)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Remove("example")
}
