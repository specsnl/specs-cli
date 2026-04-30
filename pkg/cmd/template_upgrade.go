package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/specsnl/specs-cli/pkg/specs"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
	"github.com/specsnl/specs-cli/pkg/util/output"
)

func newTemplateUpgradeCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "upgrade [name]",
		Short: "Upgrade registered templates to the latest version",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("cannot use --all with a template name")
			}
			if !all && len(args) == 0 {
				return fmt.Errorf("provide a template name or use --all")
			}

			if err := specs.EnsureRegistry(); err != nil {
				return err
			}

			var names []string
			if all {
				entries, err := os.ReadDir(specs.TemplateDir())
				if err != nil {
					return err
				}
				for _, e := range entries {
					if e.IsDir() {
						names = append(names, e.Name())
					}
				}
			} else {
				names = []string{args[0]}
			}

			for _, name := range names {
				if err := upgradeTemplate(name); err != nil {
					if all {
						output.Warn("template %q: %v", name, err)
						continue
					}
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Upgrade all remote templates")

	return cmd
}

func upgradeTemplate(name string) error {
	root := specs.TemplatePath(name)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return fmt.Errorf("template %q is not registered", name)
	}
	meta, err := loadMetadataForListing(root)
	if err != nil {
		return err
	}
	if meta == nil || meta.Repository == "" || meta.Branch == "" {
		output.Info("template %q is a local template — skipping (no remote branch)", name)
		return nil
	}

	// Determine the target ref.
	targetRef := meta.Branch
	result, _ := pkggit.CheckRemote(root, meta.Repository, meta.Branch)
	if result.ErrorKind != pkggit.CheckErrorNone {
		return fmt.Errorf("could not check remote: %s", result.ErrorKind)
	}
	if result.IsUpToDate && result.LatestVersion == "" {
		output.Info("template %q is already up-to-date", name)
		return nil
	}
	newBranch := meta.Branch
	if result.LatestVersion != "" {
		targetRef = result.LatestVersion
		newBranch = result.LatestVersion
	}

	// Remove existing template and re-clone.
	if err := os.RemoveAll(root); err != nil {
		return err
	}

	output.Info("cloning %s@%s…", meta.Repository, targetRef)
	if err := pkggit.Clone(meta.Repository, root, pkggit.CloneOptions{Branch: targetRef}); err != nil {
		return err
	}

	desc, _ := pkggit.Describe(root)
	if err := writeMetadata(root, name, meta.Repository, newBranch, desc.Commit, desc.Version); err != nil {
		return err
	}

	// Remove stale status; it will be regenerated on next template list.
	_ = os.Remove(root + "/" + specs.StatusFile)

	output.Info("template %q upgraded", name)
	return nil
}
