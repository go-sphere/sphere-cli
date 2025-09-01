package create

import (
	"flag"
	"os"
	"testing"
)

var createTest = flag.Bool("create_test", false, "run create tests that create files and directories")

func TestProject(t *testing.T) {
	if !*createTest {
		t.Skip("Skipping create tests, run with -create_test to enable")
	}
	err := Project("example", "example", &defaultTemplateLayout)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll("example")
}

func TestLayout(t *testing.T) {
	if !*createTest {
		t.Skip("Skipping create tests, run with -create_test to enable")
	}
	layout, err := Layout("https://go-sphere.github.io/layout/simple.json")
	if err != nil {
		t.Fatal(err)
	}
	err = Project("simple", "simple", layout)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.RemoveAll("simple")
}
