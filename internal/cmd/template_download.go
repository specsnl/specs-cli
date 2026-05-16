package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/internal/host"
	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
	"github.com/specsnl/specs-cli/internal/util/validate"
)

func newTemplateDownloadCmd(app *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "download <source> <name>",
		Short: "Download a template from a remote repository",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawSource, name := args[0], args[1]

			if err := validate.Name(name); err != nil {
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
				return specs.ErrLocalSource
			}

			dest := specs.TemplatePath(name)
			if _, err := os.Stat(dest); err == nil && !force {
				return fmt.Errorf("%w: %s — use --force to overwrite", specs.ErrTemplateAlreadyExists, name)
			}
			if err := os.RemoveAll(dest); err != nil {
				return err
			}

			app.Output.Info("cloning %s…", src.CloneURL)
			// git layer logs clone start/complete
			if err := pkggit.Clone(src.CloneURL, dest, pkggit.CloneOptions{Branch: src.Branch}); err != nil {
				return err
			}

			// When no branch was specified, resolve the actual checked-out branch so that
			// future update/upgrade checks can compare against the correct remote ref.
			branch := src.Branch
			if branch == "" {
				if b, err := pkggit.CurrentBranch(dest); err == nil {
					branch = b
				}
			}

			// git layer logs describe result or failure
			desc, _ := pkggit.Describe(dest)
			if err := pkgtemplate.SaveMetadata(dest, name, src.CloneURL, branch, desc.Commit, desc.Version, time.Now().UTC()); err != nil {
				return err
			}

			app.Output.Info("template %q downloaded", name)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing template")

	return cmd
}
