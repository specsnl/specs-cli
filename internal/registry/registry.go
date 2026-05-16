package registry

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/specsnl/specs-cli/internal/specs"
	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
	pkggit "github.com/specsnl/specs-cli/internal/util/git"
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
	meta, err := pkgtemplate.LoadMetadata(root)
	if err != nil {
		slog.Debug("failed to parse template metadata", "template", name, "error", err)
	}
	var status *pkgtemplate.TemplateStatus
	if meta != nil && meta.Repository != "" && meta.Branch != "" {
		status, err = pkgtemplate.LoadStatus(root)
		if err != nil {
			slog.Debug("failed to load template status", "template", name, "error", err)
		}
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

	slog.Debug("upgrade start", "template", name)

	meta, err := pkgtemplate.LoadMetadata(root)
	if err != nil {
		slog.Debug("failed to parse template metadata", "template", name, "error", err)
	}
	if meta == nil || meta.Repository == "" || strings.HasPrefix(meta.Repository, "local:") {
		return UpgradeResult{IsLocal: true}, nil
	}

	branch := meta.Branch
	if branch == "" {
		b, err := pkggit.CurrentBranch(root)
		if err != nil {
			slog.Debug("could not resolve branch from local HEAD, treating as local", "template", name, "error", err)
			return UpgradeResult{IsLocal: true}, nil
		}
		branch = b
	}

	targetRef := branch
	result := pkggit.CheckRemote(root, meta.Repository, branch)
	if err := result.Err(); err != nil {
		return UpgradeResult{}, err
	}
	if result.IsUpToDate && result.LatestVersion == "" {
		return UpgradeResult{AlreadyUpToDate: true}, nil
	}

	slog.Debug("upgrade target",
		"template", name,
		"repo", meta.Repository,
		"branch", meta.Branch,
		"target_ref", targetRef,
		"latest_version", result.LatestVersion,
	)

	newBranch := branch
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

	desc, _ := pkggit.Describe(root) // errors are logged by the git layer
	if err := pkgtemplate.SaveMetadata(root, name, meta.Repository, newBranch, desc.Commit, desc.Version, meta.Created.Time); err != nil {
		return UpgradeResult{}, err
	}

	// Remove stale status; regenerated on next template list or update.
	if err := os.Remove(root + "/" + specs.StatusFile); err != nil && !os.IsNotExist(err) {
		slog.Debug("removing stale status file failed", "template", name, "error", err)
	}

	slog.Debug("upgrade complete", "template", name, "target_ref", targetRef)
	return UpgradeResult{Repository: meta.Repository, TargetRef: targetRef}, nil
}
