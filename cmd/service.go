package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-sphere/sphere-cli/internal/service"
	"github.com/go-sphere/sphere-cli/internal/tui"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Generate service code (interactive when run in a terminal)",
	Long: `Generate service code for Sphere projects, including service interfaces and implementations.

Run without --name in a terminal to launch the interactive wizard: pick an
entity from the detected ent schemas (or type a custom name), set the package
and module, preview the generated code, and choose between stdout and file
output with a suggested path.`,
}

var serviceProtoCmd = &cobra.Command{
	Use:   "proto",
	Short: "Generate service proto code",
	Long: `Generate service proto code for Sphere projects.

The generated proto references entpb.<Entity> messages, so the entity must
already exist as an Ent schema annotated for entproto generation. Run inside
the project so --name is matched against the real schema types.

Without --name in a terminal the interactive wizard runs instead.`,
}

var serviceGolangCmd = &cobra.Command{
	Use:   "golang",
	Short: "Generate service Golang code",
	Long: `Generate service Golang code for Sphere projects.

The generated skeleton calls project-generated APIs (entbind.Create<Entity>,
ent.<Entity>.Create, s.render.<Entity>), so the entity must already exist as an
Ent schema and the project must have run 'make gen/proto'. Run inside the
project so an undivided --name like "keyvaluestore" resolves to the
KeyValueStore schema; outside a project, pass multi-word names separated
(key_value_store or key-value-store).

Without --name in a terminal the interactive wizard runs instead.`,
}

// schemaDirForCWD resolves the sphere schema directory relative to the current
// working directory when it exists, so the generators can match the service
// name against real ent schema types.
func schemaDirForCWD() string {
	return service.DetectSchemaDir(".")
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceProtoCmd)
	serviceCmd.AddCommand(serviceGolangCmd)

	{
		flag := serviceProtoCmd.Flags()
		name := flag.String("name", "", "Name of the service")
		pkg := flag.String("package", "dash.v1", "Package name for the generated proto code")
		out := flag.String("out", "", "Write the generated code to this file instead of stdout")
		serviceProtoCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if *name == "" {
				// Only prefill the wizard when --package was passed explicitly;
				// a flag default must not wedge itself into an editable field.
				pkgPrefill := ""
				if cmd.Flags().Changed("package") {
					pkgPrefill = *pkg
				}
				return runServiceWizard(serviceProtoCmd, cmd, "", pkgPrefill, "", *out)
			}
			if *pkg == "" {
				return errors.New("--package is required")
			}
			text, err := service.GenServiceProto(*name, *pkg, schemaDirForCWD())
			if err != nil {
				return err
			}
			return emitGenerated(cmd, text, *out)
		}
	}
	{
		flag := serviceGolangCmd.Flags()
		name := flag.String("name", "", "Name of the service")
		pkg := flag.String("package", "dash.v1", "Package name for the generated Go code")
		mod := flag.String("mod", "github.com/go-sphere/sphere-layout", "Go module path for the generated code")
		out := flag.String("out", "", "Write the generated code to this file instead of stdout")
		serviceGolangCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if *name == "" {
				// Only prefill the wizard with explicitly passed flags; defaults
				// must not wedge themselves into editable fields.
				pkgPrefill := ""
				if cmd.Flags().Changed("package") {
					pkgPrefill = *pkg
				}
				modPrefill := ""
				if cmd.Flags().Changed("mod") {
					modPrefill = *mod
				}
				return runServiceWizard(serviceGolangCmd, cmd, "", pkgPrefill, modPrefill, *out)
			}
			if *pkg == "" {
				return errors.New("--package is required")
			}
			text, err := service.GenServiceGolang(*name, *pkg, *mod, schemaDirForCWD())
			if err != nil {
				return err
			}
			return emitGenerated(cmd, text, *out)
		}
	}
}

// runServiceWizard launches the interactive generator, or explains the flags
// requirement when the session is not attached to a terminal.
func runServiceWizard(target *cobra.Command, print *cobra.Command, name, pkg, mod, out string) error {
	if !tui.IsInteractive() {
		return errors.New("--name is required (or run interactively in a terminal)")
	}
	var kind tui.ServiceKind
	if target == serviceGolangCmd {
		kind = tui.ServiceKindGolang
	}
	return tui.RunServiceWizard(tui.ServiceWizardOptions{
		Kind: kind,
		CWD:  ".",
		Name: name,
		Pkg:  pkg,
		Mod:  mod,
		Out:  out,
	})
}

// emitGenerated either prints the generated content or writes it to the file
// named by --out.
func emitGenerated(cmd *cobra.Command, text, out string) error {
	if out == "" {
		cmd.Println(text)
		return nil
	}
	if dir := filepath.Dir(out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("refusing to overwrite existing file %s (remove it first)", out)
	}
	if err := os.WriteFile(out, []byte(text), 0o644); err != nil {
		return err
	}
	cmd.Printf("wrote %s\n", out)
	return nil
}
