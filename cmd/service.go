package cmd

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/go-sphere/sphere-cli/internal/service"
	"github.com/spf13/cobra"
)

var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Generate service code",
	Long:  `Generate service code for Sphere projects, including service interfaces and implementations.`,
}

var serviceProtoCmd = &cobra.Command{
	Use:   "proto",
	Short: "Generate service proto code",
	Long: `Generate service proto code for Sphere projects.

The generated proto references entpb.<Entity> messages, so the entity must
already exist as an Ent schema annotated for entproto generation. Run inside
the project so --name is matched against the real schema types.`,
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
(key_value_store or key-value-store).`,
}

// schemaDirForCWD resolves the sphere schema directory relative to the current
// working directory when it exists, so the generators can match the service
// name against real ent schema types. Empty is returned outside a project (or
// when the directory is not found), in which case the generators fall back to
// inflection-based naming.
func schemaDirForCWD() string {
	for _, candidate := range []string{
		"internal/pkg/database/schema",
		"internal/ent/schema",
		"ent/schema",
		"schema",
	} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}
	return ""
}

func init() {
	rootCmd.AddCommand(serviceCmd)
	serviceCmd.AddCommand(serviceProtoCmd)
	serviceCmd.AddCommand(serviceGolangCmd)

	{
		flag := serviceProtoCmd.Flags()
		name := flag.String("name", "", "Name of the service")
		pkg := flag.String("package", "dash.v1", "Package name for the generated proto code")
		serviceProtoCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if *name == "" || *pkg == "" {
				return errors.New("--name and --package are required")
			}
			text, err := service.GenServiceProto(*name, *pkg, schemaDirForCWD())
			if err != nil {
				return err
			}
			cmd.Println(text)
			return nil
		}
	}
	{
		flag := serviceGolangCmd.Flags()
		name := flag.String("name", "", "Name of the service")
		pkg := flag.String("package", "dash.v1", "Package name for the generated Go code")
		mod := flag.String("mod", "github.com/go-sphere/sphere-layout", "Go module path for the generated code")
		serviceGolangCmd.RunE = func(cmd *cobra.Command, args []string) error {
			if *name == "" || *pkg == "" {
				return errors.New("--name and --package are required")
			}
			text, err := service.GenServiceGolang(*name, *pkg, *mod, schemaDirForCWD())
			if err != nil {
				return err
			}
			cmd.Println(text)
			return nil
		}
	}
}
