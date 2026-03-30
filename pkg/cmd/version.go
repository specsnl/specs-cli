package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current binary version.
// Set at build time via: -ldflags "-X github.com/specsnl/specs-cli/pkg/cmd.Version=1.0.0"
var Version = "dev"

var dontPrettifyVersion bool

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the specs version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintf(cmd.OutOrStdout(), "specs version %s\n", Version)
		return nil
	},
}

func init() {
	versionCmd.Flags().BoolVar(&dontPrettifyVersion, "dont-prettify", false,
		"Print plain text without styling")
}
