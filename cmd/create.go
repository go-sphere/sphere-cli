package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-sphere/sphere-cli/internal/create"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Sphere project",
	Long:  `Create a new Sphere project with the specified name and optional template.`,
}

var createListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available project templates",
	Long:  `List all available project templates that can be used when creating a new Sphere project.`,
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.AddCommand(createListCmd)

	{
		flag := createCmd.Flags()
		name := flag.String("name", "", "Name of the new Sphere project")
		module := flag.String("module", "", "Go module name for the project (optional)")
		layout := flag.String("layout", "", "Sphere layout name or custom template layout URI (optional)")
		createCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if *name == "" {
				return errors.New("--name is required")
			}
			if err := validateProjectName(*name); err != nil {
				return err
			}
			if *module == "" {
				module = name // Default to the project name if no module is specified
			}
			tmpl, err := create.Layout(*layout)
			if err != nil {
				if !strings.Contains(*layout, "://") {
					return fmt.Errorf("unknown layout %q: %w (use a built-in name from 'sphere-cli create list' or a full template URL)", *layout, err)
				}
				return err
			}
			return create.Project(*name, *module, tmpl)
		}
	}

	{
		createListCmd.RunE = func(cmd *cobra.Command, args []string) error {
			templates, err := create.LayoutList()
			if err != nil {
				return err
			}
			for _, item := range templates {
				cmd.Println(item.Name, ":", item.Description, " (", item.Path, ")")
			}
			return nil
		}
	}
}

// validateProjectName rejects names that would create the project outside the
// current directory or silently produce a different directory than the name
// the user typed (path separators, "." / ".." traversal, stray whitespace).
func validateProjectName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("--name must not be empty or whitespace")
	}
	if name != strings.TrimSpace(name) {
		return errors.New("--name must not start or end with whitespace")
	}
	if strings.ContainsAny(name, `/\`) {
		return errors.New("--name must be a single directory name without path separators")
	}
	if name == "." || name == ".." {
		return errors.New("--name must not be '.' or '..'")
	}
	return nil
}
