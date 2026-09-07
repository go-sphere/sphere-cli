package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-sphere/sphere-cli/internal/create"
	"github.com/go-sphere/sphere-cli/internal/tui"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new Sphere project (interactive when run in a terminal)",
	Long: `Create a new Sphere project with the specified name and optional template.

Run without flags in a terminal to launch the interactive wizard: pick a
template, enter the project name and Go module path, choose the optional
steps, and watch a live progress bar while the template downloads and the
project initializes.

All flags still work for scripted (non-interactive) use.`,
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
		noGit := flag.Bool("no-git", false, "Skip git repository initialization")
		noDeps := flag.Bool("no-deps", false, "Skip dependency installation (make init + go mod tidy)")
		createCmd.RunE = func(cmd *cobra.Command, args []string) error {
			// Interactive path: no --name and a real terminal.
			if *name == "" {
				if !tui.IsInteractive() {
					return errors.New("--name is required (or run interactively in a terminal)")
				}
				return tui.RunCreateWizard(tui.CreateWizardOptions{
					Name:   "",
					Module: *module,
					Layout: *layout,
					NoGit:  *noGit,
					NoDeps: *noDeps,
				})
			}
			if err := validateProjectName(*name); err != nil {
				return err
			}
			moduleName := *module
			if moduleName == "" {
				moduleName = *name // Default to the project name if no module is specified
			}
			tmpl, err := create.Layout(*layout)
			if err != nil {
				if !strings.Contains(*layout, "://") {
					return fmt.Errorf("unknown layout %q: %w (use a built-in name from 'sphere-cli create list' or a full template URL)", *layout, err)
				}
				return err
			}
			return create.Create(create.Options{
				Name:     *name,
				Module:   moduleName,
				Layout:   tmpl,
				InitGit:  !*noGit,
				InitDeps: !*noDeps,
			}, stepReporter(cmd))
		}
	}

	{
		createListCmd.RunE = func(cmd *cobra.Command, args []string) error {
			templates, usedRemote, err := create.LayoutListWithFallback()
			if err != nil {
				return err
			}
			if !usedRemote {
				cmd.Println("Remote catalog unavailable; built-in layouts:")
				cmd.Println()
			}
			for _, item := range templates {
				cmd.Printf("  %-10s %s\n", item.Name, item.Description)
			}
			return nil
		}
	}
}

// stepReporter renders pipeline progress as plain log lines for scripted runs.
func stepReporter(cmd *cobra.Command) create.Reporter {
	lastStep := -1
	return func(event create.ProgressEvent) {
		if event.Done {
			return
		}
		if event.Step != lastStep {
			lastStep = event.Step
			cmd.Printf("[%d/%d] %s\n", event.Step+1, event.TotalSteps, event.Label)
		}
	}
}

// validateProjectName rejects names that would create the project outside the
// current directory or silently produce a different directory than the name
// the user typed (path separators, "." / ".." traversal, stray whitespace).
func validateProjectName(name string) error {
	return create.ValidateProjectName(name)
}
