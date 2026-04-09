package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is the current binary version.
// Set at build time via: -ldflags "-X github.com/specsnl/specs-cli/pkg/cmd.Version=1.0.0"
var Version = "dev"

func newVersionCmd() *cobra.Command {
	var dontPrettify bool

	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the specs version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dontPrettify {
				fmt.Fprintln(cmd.OutOrStdout(), Version)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "specs version %s\n", Version)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dontPrettify, "dont-prettify", false, "Print plain text without styling")
	return cmd
}
