package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

	if opts.Branch != "" {
		cloneOpts.ReferenceName = plumbing.NewBranchReferenceName(opts.Branch)
		cloneOpts.SingleBranch = true
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

	_, err := gogit.PlainClone(dir, false, cloneOpts)
	if err != nil {
		return fmt.Errorf("cloning %s: %w", url, err)
	}
	return nil
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
