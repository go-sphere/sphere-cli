package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// version is injected at build time via
// `-ldflags "-X github.com/go-sphere/sphere-cli/cmd.version=vX.Y.Z"`.
// `make build` and the release workflow set it from the git tag.
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "sphere-cli",
	Version: version,
	Short:   "A tool for managing sphere projects",
	Long:    `Sphere CLI is a command-line tool designed to help you manage Sphere projects efficiently.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
