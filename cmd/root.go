package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "sphere-cli",
	Version: "v0.0.2",
	Short:   "A tool for managing sphere projects",
	Long:    `Sphere CLI is a command-line tool designed to help you manage Sphere projects efficiently.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
