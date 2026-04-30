package git

import (
	"errors"
	"fmt"
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

// Clone clones the repository at url into dir using a shallow clone (Depth 1 by default).
// dir must not already exist — go-git creates it.
// SSH URLs (git@host:path or ssh://host/path) are detected automatically and authenticated
// via SSH agent or standard key files in ~/.ssh.
func Clone(url, dir string, opts CloneOptions) error {
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
		return cloneWithRef(url, dir, cloneOpts, opts.Branch)
	}

	_, err := gogit.PlainClone(dir, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("cloning %s: %w", url, err)
	}
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

	// Tag ref not found — retry as a branch.
	os.RemoveAll(dir)
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
		return DescribeResult{}, fmt.Errorf("opening repository at %s: %w", dir, err)
	}

	head, err := repo.Head()
	if err != nil {
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

	return DescribeResult{
		Commit:  commit,
		Version: buildVersion(repo, head.Hash(), shortHash, dirty),
	}, nil
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
	if strings.HasPrefix(url, "ssh://") {
		rest := strings.TrimPrefix(url, "ssh://")
		if at := strings.Index(rest, "@"); at > 0 {
			return rest[:at]
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

// CheckErrorKind classifies why a remote status check failed.
type CheckErrorKind string

const (
	CheckErrorNone     CheckErrorKind = ""
	CheckErrorNetwork  CheckErrorKind = "network"
	CheckErrorAuth     CheckErrorKind = "auth"
	CheckErrorNotFound CheckErrorKind = "not-found"
	CheckErrorUnknown  CheckErrorKind = "unknown"
)

// RemoteCheckResult is the outcome of CheckRemote.
type RemoteCheckResult struct {
	IsUpToDate    bool
	LatestVersion string
	ErrorKind     CheckErrorKind
}

// CheckRemote queries the remote to determine whether the local repo at dir is
// up-to-date for the given branch/tag ref. It uses Remote.List() and never
// modifies the local repository. SSH auth is resolved automatically.
//
// On failure, ErrorKind is set in the result and error is nil.
func CheckRemote(dir, url, branch string) (RemoteCheckResult, error) {
	repo, err := gogit.PlainOpen(dir)
	if err != nil {
		return RemoteCheckResult{ErrorKind: CheckErrorUnknown}, nil
	}

	remote, err := repo.Remote("origin")
	if err != nil {
		return RemoteCheckResult{ErrorKind: CheckErrorUnknown}, nil
	}

	listOpts := &gogit.ListOptions{}
	if isSSHURL(url) {
		auth, err := sshAuth(url)
		if err != nil {
			return RemoteCheckResult{ErrorKind: CheckErrorAuth}, nil
		}
		listOpts.Auth = auth
	}

	refs, err := remote.List(listOpts)
	if err != nil {
		return RemoteCheckResult{ErrorKind: classifyRemoteError(err)}, nil
	}

	head, err := repo.Head()
	if err != nil {
		return RemoteCheckResult{ErrorKind: CheckErrorUnknown}, nil
	}

	return resolveStatus(refs, head.Hash(), branch), nil
}

// classifyRemoteError maps a remote.List error to a CheckErrorKind.
func classifyRemoteError(err error) CheckErrorKind {
	var netErr *net.OpError
	if errors.As(err, &netErr) {
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
func resolveStatus(refs []*plumbing.Reference, localHead plumbing.Hash, ref string) RemoteCheckResult {
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
