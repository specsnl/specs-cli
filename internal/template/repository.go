package template

import (
	"os"
	"path/filepath"
	"strings"
)

// legacyLocalPrefix is the marker older versions wrote in front of a Repository
// value referring to a directory on disk. It is no longer written — a path
// identifies itself by its first character — but every __metadata.json already
// installed still carries it, and that file is never rewritten except on
// upgrade. Reading it therefore has to keep working indefinitely.
const legacyLocalPrefix = "local:"

// IsLocalRepository reports whether a Metadata.Repository refers to a directory
// on disk rather than a remote to clone from.
//
// A path is recognised by its first character: absolute (/), relative (. or ..)
// or home-relative (~). None of those can begin a URL, an scp-style remote or
// an owner/repo shorthand, so the test is unambiguous. The legacy local: prefix
// is accepted too.
func IsLocalRepository(repo string) bool {
	if strings.HasPrefix(repo, legacyLocalPrefix) {
		return true
	}

	return strings.HasPrefix(repo, "/") ||
		strings.HasPrefix(repo, ".") ||
		strings.HasPrefix(repo, "~")
}

// LocalSourcePath turns a local Repository value into a path on disk: it strips
// a legacy local: prefix, then expands a leading ~ to the home directory.
//
// Every caller that touches the filesystem with the value must go through this.
// A stored "~/x" is not a path any syscall understands.
func LocalSourcePath(repo string) string {
	return expandHome(strings.TrimPrefix(repo, legacyLocalPrefix))
}

// StorableSourcePath is what a Repository value should be when a local template
// is saved: an absolute path with the home directory written as ~, and no
// marker. The stored form and the form a reader sees are then the same string,
// so nothing has to be decoded to be read.
func StorableSourcePath(absPath string) string {
	return CollapseHome(absPath)
}

// CollapseHome replaces the home directory with ~, leaving a path outside it
// alone. Inverse of expandHome.
func CollapseHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}

	if path == home {
		return "~"
	}

	// The separator is part of the prefix, so /home/bobby is not reported as
	// living inside /home/bob.
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}

	return path
}

// expandHome replaces a leading ~ with the home directory.
//
// A path that is not home-relative, or a home directory that cannot be
// resolved, is returned unchanged: the caller then fails on the literal path,
// which is a clearer error than silently reading somewhere else.
func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		return path
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}

	if path == "~" {
		return home
	}

	return filepath.Join(home, path[2:])
}
