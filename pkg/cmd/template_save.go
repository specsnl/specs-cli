package cmd

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
	"github.com/specsnl/specs-cli/pkg/util/osutil"
	"github.com/specsnl/specs-cli/pkg/util/output"
	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func newTemplateSaveCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "save <path> <name>",
		Short: "Save a local directory as a template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcPath, name := args[0], args[1]

			if err := validate.Name(name); err != nil {
				return err
			}
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			dest := specs.TemplatePath(name)
			if _, err := os.Stat(dest); err == nil && !force {
				return specs.ErrTemplateAlreadyExists
			}

			if err := os.RemoveAll(dest); err != nil {
				return err
			}
			if err := osutil.CopyDir(srcPath, dest); err != nil {
				return err
			}
			desc, _ := pkggit.Describe(srcPath)
			if err := writeMetadata(dest, name, srcPath, "", desc.Commit, desc.Version); err != nil {
				return err
			}

			output.Info("template %q saved", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing template")

	return cmd
}
