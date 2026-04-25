package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/host"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
	"github.com/specsnl/specs-cli/pkg/util/output"
	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func newTemplateDownloadCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "download <source> <tag>",
		Short: "Download a template from a remote repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawSource, tag := args[0], args[1]

			if err := validate.Tag(tag); err != nil {
				return err
			}
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			src, err := host.Parse(rawSource)
			if err != nil {
				return err
			}
			if src.IsLocal() {
				return fmt.Errorf("use 'specs template save' to register a local path")
			}

			dest := specs.TemplatePath(tag)
			if _, err := os.Stat(dest); err == nil && !force {
				return specs.ErrTemplateAlreadyExists
			}
			if err := os.RemoveAll(dest); err != nil {
				return err
			}

			output.Info("cloning %s…", src.CloneURL)
			if err := pkggit.Clone(src.CloneURL, dest, pkggit.CloneOptions{Branch: src.Branch}); err != nil {
				return err
			}
			if err := writeMetadata(dest, tag, src.CloneURL); err != nil {
				return err
			}

			output.Info("template %q downloaded", tag)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing template")

	return cmd
}
