package git

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	gogitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// CloneOptions controls how a repository is cloned.
type CloneOptions struct {
	// Branch is the branch (or tag) to check out. Empty means the remote's default branch.
	Branch string
	// Depth limits clone depth for a shallow clone. 1 is the fastest option when only the
	// latest commit is needed. 0 means a full clone.
	Depth int
}

// CloneInto clones url into a new subdirectory named name inside parent.
// parent must already exist; name must not — go-git requires a non-existent destination.
// Returns the path of the created clone directory.
func CloneInto(parent, name, url string, opts CloneOptions) (string, error) {
	dir := filepath.Join(parent, name)
	if err := Clone(url, dir, opts); err != nil {
		return "", err
	}

	return dir, nil
}

// Clone clones the repository at url into dir using a shallow clone (Depth 1 by default).
// dir must not already exist — go-git creates it.
// SSH URLs (git@host:path or ssh://host/path) are detected automatically and authenticated
// via SSH agent or standard key files in ~/.ssh.
func Clone(url, dir string, opts CloneOptions) error {
	slog.Debug("git clone start", "repo", url, "dest", dir, "branch", opts.Branch)

	cloneOpts := &gogit.CloneOptions{
		URL:      url,
		Depth:    opts.Depth,
		Progress: nil, // callers that want progress attach a writer before calling
	}

	if cloneOpts.Depth == 0 {
		cloneOpts.Depth = 1 // default: shallow clone for speed
	}

	if isSSHURL(url) {
		auth, err := sshAuth(url)
		if err != nil {
			return err
		}

		cloneOpts.Auth = auth
	}

	if opts.Branch != "" {
		cloneOpts.SingleBranch = true
		if err := cloneWithRef(url, dir, cloneOpts, opts.Branch); err != nil {
			return err
		}
	} else {
		_, err := gogit.PlainClone(dir, false, cloneOpts)
		if err != nil {
			return fmt.Errorf("cloning %s: %w", url, err)
		}
	}

	slog.Debug("git clone complete", "repo", url, "dest", dir, "branch", opts.Branch)

	return nil
}

// cloneWithRef tries the given ref as a Git tag first, then as a branch.
// This lets callers pass version tags ("0.1.0", "v1.2.3") or branch names
// ("main") without needing to know which kind of ref it is.
func cloneWithRef(url, dir string, cloneOpts *gogit.CloneOptions, ref string) error {
	cloneOpts.ReferenceName = plumbing.NewTagReferenceName(ref)

	_, err := gogit.PlainClone(dir, false, cloneOpts)
	if err == nil {
		return nil
	}

	if !strings.Contains(err.Error(), "couldn't find remote ref") {
		return fmt.Errorf("cloning %s: %w", url, err)
	}

	// Tag ref not found — retry as a branch. The dir must be emptied first,
	// otherwise the retry clone fails on a non-empty target.
	if rmErr := os.RemoveAll(dir); rmErr != nil {
		return fmt.Errorf("cleaning up %s before branch retry: %w", dir, rmErr)
	}

	cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(ref)

	_, err = gogit.PlainClone(dir, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("cloning %s: %w", url, err)
	}

	return nil
}

// DescribeResult holds version information about the state of a git repository.
type DescribeResult struct {
	// Commit is the full 40-character SHA-1 hash of HEAD.
	Commit string
	// Version is similar to `git describe --tags --dirty`: the nearest ancestor tag,
	// optionally followed by "-<n>-g<short-hash>" when HEAD is not directly on a tag,
	// and "-dirty" when the working tree has uncommitted changes.
	// Falls back to the short hash when no tags are reachable.
	Version string
}

// Describe returns version information for the repository at dir.
// Returns an error only when dir is not a git repository or HEAD cannot be read.
func Describe(dir string) (DescribeResult, error) {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		slog.Debug("git describe failed", "dest", dir, "error", err)
		return DescribeResult{}, fmt.Errorf("opening repository at %s: %w", dir, err)
	}

	head, err := repo.Head()
	if err != nil {
		slog.Debug("git describe failed", "dest", dir, "error", err)
		return DescribeResult{}, fmt.Errorf("reading HEAD: %w", err)
	}

	commit := head.Hash().String()
	shortHash := commit[:7]

	dirty := false

	if wt, err := repo.Worktree(); err == nil {
		if st, err := wt.Status(); err == nil {
			for _, s := range st {
				// Purely untracked files don't count as dirty — matches git describe --dirty.
				if s.Staging == gogit.Untracked && s.Worktree == gogit.Untracked {
					continue
				}

				dirty = true

				break
			}
		}
	}

	result := DescribeResult{
		Commit:  commit,
		Version: buildVersion(repo, head.Hash(), shortHash, dirty),
	}
	slog.Debug("git describe", "dest", dir, "commit", result.Commit, "version", result.Version)

	return result, nil
}

// buildVersion constructs a version string in git-describe style.
func buildVersion(repo *gogit.Repository, headHash plumbing.Hash, shortHash string, dirty bool) string {
	// Map each tagged commit hash to its tag name (dereference annotated tags).
	tagMap := make(map[plumbing.Hash]string)

	if tags, err := repo.Tags(); err == nil {
		_ = tags.ForEach(func(ref *plumbing.Reference) error {
			h := ref.Hash()
			if obj, err := repo.TagObject(h); err == nil {
				h = obj.Target
			}

			tagMap[h] = ref.Name().Short()

			return nil
		})
	}

	// Walk commits from HEAD to find the nearest tagged ancestor.
	foundTag, distance := "", 0

	if iter, err := repo.Log(&gogit.LogOptions{From: headHash}); err == nil {
		_ = iter.ForEach(func(c *object.Commit) error {
			if tag, ok := tagMap[c.Hash]; ok {
				foundTag = tag
				return storer.ErrStop
			}

			distance++

			return nil
		})
	}

	var v string

	switch {
	case foundTag == "":
		v = shortHash
	case distance == 0:
		v = foundTag
	default:
		v = fmt.Sprintf("%s-%d-g%s", foundTag, distance, shortHash)
	}

	if dirty {
		v += "-dirty"
	}

	return v
}

// isSSHURL reports whether url requires SSH transport.
func isSSHURL(url string) bool {
	return strings.HasPrefix(url, "ssh://") ||
		(strings.Contains(url, "@") && strings.Contains(url, ":") && !strings.Contains(url, "://"))
}

// sshUser extracts the username from an SSH URL. Defaults to "git".
func sshUser(url string) string {
	if after, ok := strings.CutPrefix(url, "ssh://"); ok {
		if at := strings.Index(after, "@"); at > 0 {
			return after[:at]
		}
	} else {
		if at := strings.Index(url, "@"); at > 0 {
			return url[:at]
		}
	}

	return "git"
}

// sshAuth builds an SSH AuthMethod for the given URL.
// Strategy: SSH agent first, then standard key files (~/.ssh/id_ed25519, id_rsa, id_ecdsa).
// Host key verification always uses ~/.ssh/known_hosts.
func sshAuth(url string) (transport.AuthMethod, error) {
	user := sshUser(url)

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolving home directory: %w", err)
	}

	khPath := filepath.Join(home, ".ssh", "known_hosts")

	hostKeyCallback, err := knownhosts.New(khPath)
	if err != nil {
		return nil, fmt.Errorf("reading ~/.ssh/known_hosts: %w", err)
	}

	// 1. SSH agent
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		if auth, err := gogitssh.NewSSHAgentAuth(user); err == nil {
			auth.HostKeyCallback = hostKeyCallback
			return auth, nil
		}
	}

	// 2. Standard key files
	for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
		keyPath := filepath.Join(home, ".ssh", name)

		auth, err := gogitssh.NewPublicKeysFromFile(user, keyPath, "")
		if err != nil {
			continue
		}

		auth.HostKeyCallback = hostKeyCallback

		return auth, nil
	}

	return nil, fmt.Errorf("no SSH authentication available: SSH agent not running and no usable key file found in ~/.ssh")
}

// CurrentBranch returns the short name of the currently checked-out branch in dir.
// Returns an error when dir is not a git repository, HEAD cannot be read, or HEAD is in a
// detached state (e.g. a tag checkout).
func CurrentBranch(dir string) (string, error) {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return "", fmt.Errorf("opening repository at %s: %w", dir, err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("reading HEAD: %w", err)
	}

	if !head.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not on a branch (detached HEAD or tag checkout)")
	}

	return head.Name().Short(), nil
}

// CheckErrorKind classifies why a remote status check failed.
type CheckErrorKind string

const (
	CheckErrorNone          CheckErrorKind = ""
	CheckErrorNetwork       CheckErrorKind = "network"
	CheckErrorAuth          CheckErrorKind = "auth"
	CheckErrorNotFound      CheckErrorKind = "not-found"
	CheckErrorUnknown       CheckErrorKind = "unknown"
	CheckErrorSourceMissing CheckErrorKind = "source-missing"
)

var (
	// ErrCheckNetwork is returned when a remote status check fails due to a network error.
	ErrCheckNetwork = errors.New("network error checking remote")
	// ErrCheckAuth is returned when a remote status check fails due to an authentication error.
	ErrCheckAuth = errors.New("authentication error checking remote")
	// ErrCheckNotFound is returned when the remote repository is not found.
	ErrCheckNotFound = errors.New("repository not found at remote")
	// ErrCheckUnknown is returned when a remote status check fails for an unknown reason.
	ErrCheckUnknown = errors.New("unknown error checking remote")
	// ErrCheckSourceMissing is returned when a local template's source path no longer
	// exists (or is no longer a git repository) so its status cannot be determined.
	ErrCheckSourceMissing = errors.New("local source path is missing")
)

// RemoteCheckResult is the outcome of CheckRemote.
type RemoteCheckResult struct {
	IsUpToDate    bool
	LatestVersion string
	ErrorKind     CheckErrorKind
}

// Err returns a typed error when the check failed, or nil on success.
// Callers can use errors.Is to distinguish the failure kind.
func (r RemoteCheckResult) Err() error {
	switch r.ErrorKind {
	case CheckErrorNetwork:
		return ErrCheckNetwork
	case CheckErrorAuth:
		return ErrCheckAuth
	case CheckErrorNotFound:
		return ErrCheckNotFound
	case CheckErrorUnknown:
		return ErrCheckUnknown
	case CheckErrorSourceMissing:
		return ErrCheckSourceMissing
	default:
		return nil
	}
}

// CheckRemoteContext queries the remote to determine whether the local repo at
// dir is up-to-date for the given branch/tag ref. It uses Remote.ListContext()
// and never modifies the local repository. SSH auth is resolved automatically.
// ctx is forwarded to the underlying network call; cancel it to abort early.
//
// On failure, ErrorKind is set in the result; errors are never returned.
func CheckRemoteContext(ctx context.Context, dir, url, branch string) (result RemoteCheckResult) {
	slog.Debug("git check-remote start", "repo", url, "branch", branch, "dest", dir)

	defer func() {
		slog.Debug("git check-remote result",
			"repo", url, "branch", branch, "dest", dir,
			"up_to_date", result.IsUpToDate,
			"latest_version", result.LatestVersion,
			"error_kind", string(result.ErrorKind),
		)
	}()

	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return RemoteCheckResult{ErrorKind: CheckErrorUnknown}
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return RemoteCheckResult{ErrorKind: CheckErrorUnknown}
	}

	listOpts := &gogit.ListOptions{}

	if isSSHURL(url) {
		auth, err := sshAuth(url)
		if err != nil {
			return RemoteCheckResult{ErrorKind: CheckErrorAuth}
		}

		listOpts.Auth = auth
	}

	refs, err := remote.ListContext(ctx, listOpts)
	if err != nil {
		if ctx.Err() != nil {
			return RemoteCheckResult{ErrorKind: CheckErrorNetwork}
		}

		return RemoteCheckResult{ErrorKind: classifyRemoteError(err)}
	}

	head, err := repo.Head()
	if err != nil {
		return RemoteCheckResult{ErrorKind: CheckErrorUnknown}
	}

	// currentVersion is the semver tag the local checkout sits exactly on, or "" when
	// HEAD is on an untagged branch commit. resolveStatus uses it to decide whether a
	// branch-tracked template should be compared by semver or by branch-tip commit.
	currentVersion := semverTagAtCommit(repo, head.Hash())

	return resolveStatus(refs, head.Hash(), branch, currentVersion)
}

// semverTagAtCommit returns the highest valid semver tag that points directly at
// hash (dereferencing annotated tags), or "" when no semver tag is exactly on that
// commit. This tells CheckRemoteContext whether the local checkout is pinned to a
// released version — as opposed to sitting on an arbitrary branch commit — without
// relying on git-describe output, whose "-<n>-g<hash>" suffix would otherwise be
// misread as a semver pre-release.
func semverTagAtCommit(repo *gogit.Repository, hash plumbing.Hash) string {
	tags, err := repo.Tags()
	if err != nil {
		return ""
	}

	var best *semver.Version

	var bestOrig string

	_ = tags.ForEach(func(ref *plumbing.Reference) error {
		h := ref.Hash()
		if obj, err := repo.TagObject(h); err == nil {
			h = obj.Target // annotated tag: resolve to the commit it points at
		}

		if h != hash {
			return nil
		}

		v, err := semver.NewVersion(ref.Name().Short())
		if err != nil {
			return nil
		}

		if best == nil || v.GreaterThan(best) {
			best = v
			bestOrig = v.Original()
		}

		return nil
	})

	return bestOrig
}

// CheckRemote is a context-free convenience wrapper around CheckRemoteContext.
func CheckRemote(dir, url, branch string) RemoteCheckResult {
	return CheckRemoteContext(context.Background(), dir, url, branch)
}

// CheckLocalSource determines whether a template registered from a local path
// is behind the current state of that path. Unlike
// CheckRemoteContext, it never touches the network or the template's git origin —
// the "source of truth" for a local template is the directory it was saved from.
//
// sourcePath is the on-disk path the template was saved from, as
// template.LocalSourcePath resolves it — any legacy "local:" prefix stripped
// and a leading ~ already expanded. savedCommit/savedVersion are the metadata
// recorded at save
// time. The path's current git-describe output is compared against them:
//
//   - path missing or not a git repository → ErrorKind CheckErrorSourceMissing.
//   - describe matches the saved commit and version → up-to-date.
//   - source still on the saved commit, differing only by the "-dirty" marker →
//     up-to-date. A dirty working tree flips the "-dirty" suffix on git-describe
//     output even though HEAD has not moved, so comparing it verbatim would report
//     a phantom update to the very same commit (issue #97).
//   - otherwise → not up-to-date, with LatestVersion set to the path's current version.
func CheckLocalSource(sourcePath, savedCommit, savedVersion string) (result RemoteCheckResult) {
	slog.Debug("git check-local start", "source", sourcePath, "saved_commit", savedCommit, "saved_version", savedVersion)

	defer func() {
		slog.Debug("git check-local result",
			"source", sourcePath,
			"up_to_date", result.IsUpToDate,
			"latest_version", result.LatestVersion,
			"error_kind", string(result.ErrorKind),
		)
	}()

	if info, err := os.Stat(sourcePath); err != nil || !info.IsDir() {
		return RemoteCheckResult{ErrorKind: CheckErrorSourceMissing}
	}

	desc, err := Describe(sourcePath)
	if err != nil {
		return RemoteCheckResult{ErrorKind: CheckErrorSourceMissing}
	}

	// When the source still sits on the saved commit, any version-string difference
	// can only be the "-dirty" marker (same commit ⇒ same tag, distance and short
	// hash). Uncommitted changes are transient working-tree state, not a new commit
	// to update to, so treat this as up-to-date rather than a phantom update.
	if desc.Commit == savedCommit && stripDirty(desc.Version) == stripDirty(savedVersion) {
		return RemoteCheckResult{IsUpToDate: true}
	}

	return RemoteCheckResult{IsUpToDate: false, LatestVersion: desc.Version}
}

// stripDirty removes the trailing "-dirty" marker that Describe appends for a
// working tree with uncommitted changes.
func stripDirty(version string) string {
	return strings.TrimSuffix(version, "-dirty")
}

// classifyRemoteError maps a remote.List error to a CheckErrorKind.
func classifyRemoteError(err error) CheckErrorKind {
	if _, ok := errors.AsType[*net.OpError](err); ok {
		return CheckErrorNetwork
	}

	switch {
	case errors.Is(err, transport.ErrAuthenticationRequired),
		errors.Is(err, transport.ErrAuthorizationFailed):
		return CheckErrorAuth
	case errors.Is(err, transport.ErrRepositoryNotFound):
		return CheckErrorNotFound
	}

	return CheckErrorUnknown
}

// resolveStatus compares remote refs against the local HEAD for the given ref.
// Tag-first resolution is used, consistent with Clone behaviour.
//
// currentVersion is the semver tag the local checkout sits exactly on (or "").
// It is only consulted for branch-tracked templates: when it is a valid semver
// version the check is resolved by semver rather than by branch-tip commit (see below).
func resolveStatus(refs []*plumbing.Reference, localHead plumbing.Hash, ref, currentVersion string) RemoteCheckResult {
	tagRef := plumbing.NewTagReferenceName(ref)
	branchRef := plumbing.NewBranchReferenceName(ref)

	remoteTags := map[string]struct{}{}

	for _, r := range refs {
		if r.Name().IsTag() {
			remoteTags[r.Name().Short()] = struct{}{}
		}
	}

	// Tag-first: if the remote has this ref as a tag, treat as semver template.
	for _, r := range refs {
		if r.Name() == tagRef {
			latest := latestSemverTag(remoteTags, ref)
			if latest == "" || latest == ref {
				return RemoteCheckResult{IsUpToDate: true}
			}

			return RemoteCheckResult{IsUpToDate: false, LatestVersion: latest}
		}
	}

	// Branch fallback.
	for _, r := range refs {
		if r.Name() == branchRef {
			// When the local checkout sits exactly on a released semver version, a newer
			// version must be a strictly-greater semver tag — not merely a newer commit on
			// the branch. This stops a lower-numbered tag pushed on a later commit (e.g.
			// 1.0.1 after 1.1.0) from being reported as an update (issue #83). Rolling
			// branches with no semver version fall back to comparing the branch-tip commit.
			if _, err := semver.NewVersion(currentVersion); err == nil {
				latest := latestSemverTag(remoteTags, currentVersion)
				if latest == "" || latest == currentVersion {
					return RemoteCheckResult{IsUpToDate: true}
				}

				return RemoteCheckResult{IsUpToDate: false, LatestVersion: latest}
			}

			return RemoteCheckResult{IsUpToDate: r.Hash() == localHead}
		}
	}

	return RemoteCheckResult{ErrorKind: CheckErrorNotFound}
}

// latestSemverTag returns the highest semver tag strictly greater than current.
// Returns "" if current is already the latest or no valid semver tags exist.
func latestSemverTag(tags map[string]struct{}, current string) string {
	cur, err := semver.NewVersion(current)
	if err != nil {
		return ""
	}

	var latest *semver.Version

	for tag := range tags {
		v, err := semver.NewVersion(tag)
		if err != nil {
			continue
		}

		if v.GreaterThan(cur) && (latest == nil || v.GreaterThan(latest)) {
			latest = v
		}
	}

	if latest == nil {
		return ""
	}

	return latest.Original()
}
