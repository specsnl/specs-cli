package git

import (
	"errors"
	"fmt"
	"os"

	ggit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// Clone clones the repository at url into dir.
// If branch is non-empty it is first resolved as a branch (refs/heads/<branch>);
// if that reference does not exist on the remote the same value is retried as a
// tag (refs/tags/<branch>). Passing an empty string checks out the default branch.
func Clone(url, dir, branch string) error {
	opts := &ggit.CloneOptions{URL: url}
	if branch != "" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
	}

	_, err := ggit.PlainClone(dir, false, opts)
	if err == nil {
		return nil
	}

	// If the reference was not found as a branch, retry as a tag.
	if branch != "" && errors.Is(err, plumbing.ErrReferenceNotFound) {
		// Remove any partial state left by the failed attempt.
		_ = os.RemoveAll(dir)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("cloning %s: %w", url, err)
		}
		opts.ReferenceName = plumbing.NewTagReferenceName(branch)
		if _, err = ggit.PlainClone(dir, false, opts); err != nil {
			return fmt.Errorf("cloning %s: %w", url, err)
		}
		return nil
	}

	return fmt.Errorf("cloning %s: %w", url, err)
}
