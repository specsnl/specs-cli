package host

import (
	"fmt"
	"strings"
)

// Source represents a resolved template source — either a remote clone target or a local path.
type Source struct {
	CloneURL  string // empty for local paths
	LocalPath string // empty for remote sources
	Branch    string // empty = default branch (HEAD)
}

// IsLocal reports whether the source is a local path rather than a remote URL.
func (s *Source) IsLocal() bool {
	return s.LocalPath != ""
}

// IsSSH reports whether the source uses SSH transport.
func (s *Source) IsSSH() bool {
	return strings.HasPrefix(s.CloneURL, "ssh://") ||
		(strings.Contains(s.CloneURL, "@") && strings.Contains(s.CloneURL, ":"))
}

// Parse parses a source string into a Source.
//
// Accepted formats:
//   - github:user/repo              GitHub shorthand
//   - github:user/repo:branch       GitHub shorthand with branch
//   - https://github.com/user/repo  Full HTTPS URL (optional .git suffix is stripped)
//   - git@github.com:user/repo      SCP-style SSH URL (optional .git suffix is stripped)
//   - ssh://git@github.com/user/repo Explicit SSH URL
//   - file:./path                   Explicit local-path prefix
//   - ./path  ../path  /path        Implicit local path (relative or absolute)
func Parse(input string) (*Source, error) {
	switch {
	case strings.HasPrefix(input, "github:"):
		return parseGitHub(input)
	case strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://"):
		return parseHTTPS(input)
	case strings.HasPrefix(input, "ssh://"):
		return parseSSH(input)
	case isScpStyle(input):
		return parseSSH(input)
	case strings.HasPrefix(input, "file:"):
		return &Source{LocalPath: strings.TrimPrefix(input, "file:")}, nil
	case strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") || strings.HasPrefix(input, "/"):
		return &Source{LocalPath: input}, nil
	default:
		return nil, fmt.Errorf("unrecognised source format %q — use github:user/repo, an HTTPS URL, an SSH URL, or a local path", input)
	}
}

// parseGitHub handles the "github:user/repo" and "github:user/repo:branch" forms.
func parseGitHub(input string) (*Source, error) {
	// Strip the "github:" prefix and split on ":".
	// Valid forms after stripping:  "user/repo"  or  "user/repo:branch"
	rest := strings.TrimPrefix(input, "github:")
	parts := strings.SplitN(rest, ":", 2)

	repo := parts[0]
	if !strings.Contains(repo, "/") {
		return nil, fmt.Errorf("github source must be in the form github:user/repo, got %q", input)
	}

	s := &Source{
		CloneURL: "https://github.com/" + repo,
	}
	if len(parts) == 2 {
		s.Branch = parts[1]
	}
	return s, nil
}

// parseHTTPS normalises a full HTTPS URL: strips a trailing ".git" suffix.
func parseHTTPS(input string) (*Source, error) {
	url := strings.TrimSuffix(input, ".git")
	return &Source{CloneURL: url}, nil
}

// parseSSH stores an SSH URL as-is, stripping a trailing ".git" suffix.
func parseSSH(input string) (*Source, error) {
	return &Source{CloneURL: strings.TrimSuffix(input, ".git")}, nil
}

// isScpStyle detects SCP-style SSH URLs (git@host:path) — has "@" before ":" and no "://" scheme.
func isScpStyle(input string) bool {
	atIdx := strings.Index(input, "@")
	colonIdx := strings.Index(input, ":")
	return atIdx > 0 && colonIdx > atIdx && !strings.Contains(input, "://")
}
