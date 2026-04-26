package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/specsnl/specs-cli/pkg/host"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
	"github.com/specsnl/specs-cli/pkg/util/osutil"
	"github.com/specsnl/specs-cli/pkg/util/output"
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
	defer os.RemoveAll(tmp)

	var templateRoot string

	if src.IsLocal() {
		if err := osutil.CopyDir(src.LocalPath, tmp); err != nil {
			return fmt.Errorf("copying local template: %w", err)
		}
		templateRoot = tmp
	} else {
		// go-git requires the destination to not exist; use a subdirectory.
		cloneDir := filepath.Join(tmp, "repo")
		output.Info("cloning %s…", src.CloneURL)
		if err := pkggit.Clone(src.CloneURL, cloneDir, pkggit.CloneOptions{Branch: src.Branch}); err != nil {
			return err
		}
		templateRoot = cloneDir
	}

	return app.executeTemplate(templateRoot, targetDir, opts)
}
