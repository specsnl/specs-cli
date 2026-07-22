package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/specsnl/specs-cli/internal/host"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
	"github.com/specsnl/specs-cli/internal/util/osutil"
)

func newUseCmd(app *App) *cobra.Command {
	var opts executeOpts

	cmd := &cobra.Command{
		Use:   "use <source> <target-dir>",
		Short: "Fetch and execute a template in one step (no registry entry created)",
		Long: `Download a template from a remote repository (or copy a local path) and
execute it directly into <target-dir>. No entry is added to the local registry.

Source formats:
  github:user/repo            GitHub shorthand (default branch)
  github:user/repo:branch     GitHub shorthand with specific branch
  https://github.com/user/repo  Full HTTPS URL
  ./path  ../path  /path      Local path
  file:./path                 Local path with explicit prefix`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUse(app, args[0], args[1], opts)
		},
	}

	cmd.Flags().StringVar(&opts.valuesFile, "values", "", "JSON file of pre-filled values")
	cmd.Flags().StringArrayVar(&opts.argPairs, "arg", nil, "Key=Value pair (repeatable)")
	cmd.Flags().BoolVar(&opts.useDefaults, "use-defaults", false, "Skip prompts; use schema defaults")
	cmd.Flags().BoolVar(&opts.noHooks, "no-hooks", false, "Skip pre/post-use hooks")
	cmd.Flags().BoolVar(&opts.allowHooks, "allow-hooks", false, "Allow hooks even when --safe-mode is set")
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "Skip interactive confirmation for remote hook execution")
	cmd.Flags().BoolVar(&opts.continueOnError, "continue-on-error", false, "Warn and copy files verbatim on render errors instead of aborting")

	return cmd
}

func runUse(app *App, rawSource, targetDir string, opts executeOpts) error {
	src, err := host.Parse(rawSource)
	if err != nil {
		return err
	}

	// Parent temp dir — always cleaned up, even on error.
	tmp, err := os.MkdirTemp("", "specs-use-src-*")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			app.Output.Warn("failed to remove temp dir %s: %v", tmp, err)
		}
	}()

	var templateRoot string

	if src.IsLocal() {
		if err := osutil.CopyDir(src.LocalPath, tmp); err != nil {
			return fmt.Errorf("copying local template: %w", err)
		}

		templateRoot = tmp
	} else {
		app.Output.Info("cloning %s…", src.CloneURL)
		// git layer logs clone start/complete
		cloneDir, err := pkggit.CloneInto(tmp, "repo", src.CloneURL, pkggit.CloneOptions{Branch: src.Branch})
		if err != nil {
			return err
		}

		templateRoot = cloneDir
		opts.remote = true
	}

	return app.executeTemplate(templateRoot, targetDir, opts)
}
