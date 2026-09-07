package cmd

import (
	"errors"

	"github.com/go-sphere/sphere-cli/internal/renamer"
	"github.com/go-sphere/sphere-cli/internal/tui"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename",
	Short: "Rename Go module in a directory (interactive when run in a terminal)",
	Long: `Rename the Go module in the specified directory from old to new name.

Without --old/--new in a terminal the interactive wizard runs instead: it
reads the current module from go.mod, asks for the new one, and confirms
before rewriting imports across the project.`,
}

func init() {
	rootCmd.AddCommand(renameCmd)

	flag := renameCmd.Flags()
	oldMod := flag.String("old", "", "Old Go module name (detected from go.mod when omitted)")
	newMod := flag.String("new", "", "New Go module name")
	target := flag.String("target", ".", "Target directory to rename the module in")

	renameCmd.RunE = func(cmd *cobra.Command, args []string) error {
		if *newMod == "" {
			if !tui.IsInteractive() {
				return errors.New("--old and --new are required (or run interactively in a terminal)")
			}
			// Only prefill the target when --target was passed explicitly;
			// the flag's "." default would otherwise wedge itself into the
			// wizard's editable field.
			targetPrefill := ""
			if cmd.Flags().Changed("target") {
				targetPrefill = *target
			}
			return tui.RunRenameWizard(tui.RenameWizardOptions{
				Old:    *oldMod,
				New:    "",
				Target: targetPrefill,
			})
		}
		if *oldMod == "" {
			// Keep the non-interactive path convenient: detect the old module
			// from go.mod instead of failing.
			detected, err := renamer.ModulePath(*target)
			if err != nil {
				return errors.New("--old and --new are required")
			}
			*oldMod = detected
		}
		if *oldMod == *newMod {
			return errors.New("--old and --new must be different")
		}
		return renamer.RenameProjectModule(*oldMod, *newMod, *target, []string{
			"buf.gen.yaml",
			"buf.binding.yaml",
		}, true)
	}
}
