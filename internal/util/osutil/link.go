package osutil

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/specsnl/specs-cli/internal/specs"
)

// maxLinkHops matches Linux's MAXSYMLINKS; a longer chain is treated as a loop.
const maxLinkHops = 40

// LinkError reports a symlink that was left out of a copy or a render.
type LinkError struct {
	Path   string // the link, relative to the root it was checked against
	Target string // the link's target as written
	Err    error
}

func (e *LinkError) Error() string {
	return fmt.Sprintf("%s → %s: %v", e.Path, e.Target, e.Err)
}

func (e *LinkError) Unwrap() error { return e.Err }

// IsLink reports whether d is a symlink.
func IsLink(d fs.DirEntry) bool {
	return d.Type()&fs.ModeSymlink != 0
}

// ResolveLink follows the symlink at path, and every link along the way, and
// returns the real path the chain ends at. Every step must stay inside root:
// an absolute target, or a relative one that climbs out of root even for one
// step, fails with specs.ErrSymlinkOutside. A chain that ends at nothing, or
// loops, fails with specs.ErrSymlinkDangling.
//
// Components are resolved one at a time rather than cleaned lexically, because
// "a/../b" where a is itself a link climbs from a's target, not from the
// directory holding a.
func ResolveLink(root, path string) (string, error) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", err
	}

	if !filepath.IsLocal(rel) {
		return "", specs.ErrSymlinkOutside
	}

	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}

	resolved := realRoot
	pending := splitPath(rel)
	hops := 0

	for len(pending) > 0 {
		part := pending[0]
		pending = pending[1:]

		switch part {
		case ".":
			continue
		case "..":
			if resolved == realRoot {
				return "", specs.ErrSymlinkOutside
			}

			resolved = filepath.Dir(resolved)

			continue
		}

		next := filepath.Join(resolved, part)

		info, err := os.Lstat(next)
		if err != nil {
			return "", specs.ErrSymlinkDangling
		}

		if info.Mode()&fs.ModeSymlink == 0 {
			resolved = next
			continue
		}

		hops++
		if hops > maxLinkHops {
			return "", specs.ErrSymlinkDangling
		}

		target, err := os.Readlink(next)
		if err != nil {
			return "", err
		}

		if filepath.IsAbs(target) || filepath.VolumeName(target) != "" {
			return "", specs.ErrSymlinkOutside
		}

		pending = append(splitPath(target), pending...)
	}

	return resolved, nil
}

// LinkTarget returns the target of the symlink at path, as written, when the
// link resolves inside root. A link that does not is reported as a *LinkError
// with its path relative to root.
func LinkTarget(root, path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", err
	}

	if _, err := ResolveLink(root, path); err != nil {
		if !errors.Is(err, specs.ErrSymlinkOutside) && !errors.Is(err, specs.ErrSymlinkDangling) {
			return "", err
		}

		return target, &LinkError{Path: relSlash(root, path), Target: target, Err: err}
	}

	return target, nil
}

// Symlink creates link pointing at target, creating its parent directories and
// replacing a file or link already at link.
func Symlink(target, link string) error {
	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		return err
	}

	if info, err := os.Lstat(link); err == nil && !info.IsDir() {
		if err := os.Remove(link); err != nil {
			return err
		}
	}

	return os.Symlink(target, link)
}

func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool {
		return r < 0x80 && os.IsPathSeparator(uint8(r))
	})
}

func relSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}
