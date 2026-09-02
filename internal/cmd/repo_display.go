package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/specsnl/specs-cli/internal/util/output"
)

// localRepoPrefix marks a Repository value that refers to a directory on disk
// (registered via 'specs template save') rather than a remote git URL.
const localRepoPrefix = "local:"

// defaultHost is the host a bare owner/repo label means. GitHub is the default
// for this CLI, so its name is noise in a table and is dropped from the label.
const defaultHost = "github.com"

// isLocalRepo reports whether repo refers to a local source path.
func isLocalRepo(repo string) bool {
	return strings.HasPrefix(repo, localRepoPrefix)
}

// repoCell builds the Repository cell: a label short enough to read, carrying
// the stored value for --output json and, for a remote, the URL to open.
//
// The label drops what a reader does not need. A remote loses its scheme, and
// a GitHub remote loses the host as well, because a bare owner/repo means
// GitHub here — the same shorthand the source parser accepts. A local path
// loses the local: marker and collapses $HOME to ~. Everything else is left
// verbatim: an SSH remote and a legacy hand-written value are shown as stored,
// and neither is linked, being nothing a terminal can open.
//
// This lives in cmd rather than in the renderer because every rule above is
// knowledge about what a specs Repository value *means*, and because the ~
// collapse reads the environment — which RenderTable deliberately does not.
func repoCell(repo string) output.Cell {
	switch {
	case isLocalRepo(repo):
		return output.Cell{Value: repo, Text: shortenPath(strings.TrimPrefix(repo, localRepoPrefix))}

	case strings.HasPrefix(repo, "https://"), strings.HasPrefix(repo, "http://"):
		return output.Cell{Value: repo, Text: shortenURL(repo), Link: repo}

	default:
		return output.Cell{Value: repo}
	}
}

// shortenURL drops the scheme, and the host too when it is the default one.
//
// A trailing slash and a .git suffix are trimmed from the label. host.Parse
// strips .git when a template is downloaded, but a hand-written __metadata.json
// can carry either, and a trailing slash survives its validation.
func shortenURL(rawURL string) string {
	_, rest, ok := strings.Cut(rawURL, "://")
	if !ok {
		return rawURL
	}

	rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")

	// Only the host itself is dropped, never a host that merely ends in it, so
	// git.github.com keeps its name.
	if host, path, found := strings.Cut(rest, "/"); found && host == defaultHost {
		return path
	}

	return rest
}

// shortenPath collapses $HOME to ~, leaving a path outside it alone.
func shortenPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}

	if path == home {
		return "~"
	}

	// filepath.Separator rather than a bare prefix check, so /home/bobby is not
	// reported as living inside /home/bob.
	if rest, ok := strings.CutPrefix(path, home+string(filepath.Separator)); ok {
		return "~" + string(filepath.Separator) + rest
	}

	return path
}
