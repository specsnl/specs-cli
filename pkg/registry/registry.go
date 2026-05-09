package registry

import (
	"fmt"
	"os"

	"github.com/specsnl/specs-cli/pkg/specs"
	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
	pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

// Entry represents a registered template with its metadata and cached remote status.
type Entry struct {
	Name     string
	Root     string
	Metadata *pkgtemplate.Metadata
	Status   *pkgtemplate.TemplateStatus
}

// UpgradeResult describes the outcome of an Upgrade call.
type UpgradeResult struct {
	// IsLocal is true when the template has no tracked remote branch and was skipped.
	IsLocal bool
	// AlreadyUpToDate is true when the remote confirms the template is current.
	AlreadyUpToDate bool
	// Repository is the remote URL that was cloned.
	Repository string
	// TargetRef is the branch or tag that was checked out.
	TargetRef string
}

// Load returns the registry Entry for name, loading its metadata and cached status.
// Returns nil when the template does not exist.
func Load(name string) (*Entry, error) {
	root := specs.TemplatePath(name)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}
	meta, _ := pkgtemplate.LoadMetadata(root)
	var status *pkgtemplate.TemplateStatus
	if meta != nil && meta.Repository != "" && meta.Branch != "" {
		status, _ = pkgtemplate.LoadStatus(root)
	}
	return &Entry{Name: name, Root: root, Metadata: meta, Status: status}, nil
}

// Upgrade fetches the latest version of the named template from its remote and
// replaces the local copy. It preserves the original Created timestamp in metadata
// and removes the stale __status.json so the next list/update refreshes it.
//
// Returns ErrTemplateNotFound when name is not registered.
// Returns a result with IsLocal=true when the template has no remote branch.
// Returns a result with AlreadyUpToDate=true when no newer version is available.
func Upgrade(name string) (UpgradeResult, error) {
	root := specs.TemplatePath(name)
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return UpgradeResult{}, fmt.Errorf("%w: %s", specs.ErrTemplateNotFound, name)
	}

	meta, _ := pkgtemplate.LoadMetadata(root)
	if meta == nil || meta.Repository == "" || meta.Branch == "" {
		return UpgradeResult{IsLocal: true}, nil
	}

	targetRef := meta.Branch
	result, _ := pkggit.CheckRemote(root, meta.Repository, meta.Branch)
	if err := result.Err(); err != nil {
		return UpgradeResult{}, err
	}
	if result.IsUpToDate && result.LatestVersion == "" {
		return UpgradeResult{AlreadyUpToDate: true}, nil
	}

	newBranch := meta.Branch
	if result.LatestVersion != "" {
		targetRef = result.LatestVersion
		newBranch = result.LatestVersion
	}

	if err := os.RemoveAll(root); err != nil {
		return UpgradeResult{}, err
	}
	if err := pkggit.Clone(meta.Repository, root, pkggit.CloneOptions{Branch: targetRef}); err != nil {
		return UpgradeResult{}, err
	}

	desc, _ := pkggit.Describe(root)
	if err := pkgtemplate.SaveMetadata(root, name, meta.Repository, newBranch, desc.Commit, desc.Version, meta.Created.Time); err != nil {
		return UpgradeResult{}, err
	}

	// Remove stale status; regenerated on next template list or update.
	_ = os.Remove(root + "/" + specs.StatusFile)

	return UpgradeResult{Repository: meta.Repository, TargetRef: targetRef}, nil
}
