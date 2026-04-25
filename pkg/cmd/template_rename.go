package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	"github.com/specsnl/specs-cli/pkg/util/output"
	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func newTemplateRenameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rename <old-tag> <new-tag>",
		Short: "Rename a registered template",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			oldTag, newTag := args[0], args[1]

			if err := validate.Tag(newTag); err != nil {
				return err
			}
			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			src := specs.TemplatePath(oldTag)
			if _, err := os.Stat(src); os.IsNotExist(err) {
				return fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, oldTag)
			}

			dst := specs.TemplatePath(newTag)
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("tag %q already exists — delete it first", newTag)
			}

			if err := os.Rename(src, dst); err != nil {
				return err
			}

			output.Info("template %q renamed to %q", oldTag, newTag)
			return nil
		},
	}
}
