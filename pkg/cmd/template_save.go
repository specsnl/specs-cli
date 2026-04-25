package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	"github.com/specsnl/specs-cli/pkg/util/osutil"
	"github.com/specsnl/specs-cli/pkg/util/output"
	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func newTemplateSaveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "save <path> <tag>",
		Short: "Save a local directory as a template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcPath, tag := args[0], args[1]

			if err := validate.Tag(tag); err != nil {
				return err
			}
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			dest := specs.TemplatePath(tag)
			if _, err := os.Stat(dest); err == nil && !force {
				return specs.ErrTemplateAlreadyExists
			}

			if err := os.RemoveAll(dest); err != nil {
				return err
			}
			if err := osutil.CopyDir(srcPath, dest); err != nil {
				return err
			}
			if err := writeMetadata(dest, tag, srcPath); err != nil {
				return err
			}

			output.Info("template %q saved", tag)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing template")

	return cmd
}
