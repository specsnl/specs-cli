package host

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// nameRe matches a GitHub owner or repo name: starts and ends with alphanumeric,
// middle may include dots, underscores, hyphens. Single-char names are allowed.
var nameRe = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9._-]*[A-Za-z0-9])?$`)

const (
	maxOwnerLen = 39
	maxRepoLen  = 100
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
	rest := strings.TrimPrefix(input, "github:")
	parts := strings.SplitN(rest, ":", 2)

	repoPart := parts[0]

	before, after, ok := strings.Cut(repoPart, "/")
	if !ok {
		return nil, fmt.Errorf("invalid github source: missing owner/repo separator in %q", input)
	}

	owner := before
	repo := after

	if strings.Contains(repo, "/") {
		return nil, fmt.Errorf("invalid github source: too many path segments in %q", input)
	}

	if err := validateGitHubName("owner", owner, maxOwnerLen); err != nil {
		return nil, fmt.Errorf("invalid github source: %w", err)
	}

	if err := validateGitHubName("repo", repo, maxRepoLen); err != nil {
		return nil, fmt.Errorf("invalid github source: %w", err)
	}

	s := &Source{
		CloneURL: "https://github.com/" + owner + "/" + repo,
	}

	if len(parts) == 2 {
		branch := parts[1]
		if err := validateBranch(branch); err != nil {
			return nil, fmt.Errorf("invalid github source: %w", err)
		}

		s.Branch = branch
	}

	return s, nil
}

// validateGitHubName checks that a GitHub owner or repo name is valid.
func validateGitHubName(kind, name string, maxLen int) error {
	if name == "" {
		return fmt.Errorf("%s is empty", kind)
	}

	if len(name) > maxLen {
		return fmt.Errorf("%s %q exceeds %d characters", kind, name, maxLen)
	}

	if !nameRe.MatchString(name) {
		return fmt.Errorf("%s name %q is not a valid GitHub name (use letters, numbers, dots, hyphens, underscores)", kind, name)
	}

	return nil
}

// validateBranch checks that a branch name is non-empty, has no whitespace, and no "..".
func validateBranch(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch is empty")
	}

	if strings.ContainsAny(branch, " \t\n\r") {
		return fmt.Errorf("branch %q contains whitespace", branch)
	}

	if strings.Contains(branch, "..") {
		return fmt.Errorf("branch %q contains \"..\"", branch)
	}

	return nil
}

// parseHTTPS normalises a full HTTPS URL: strips a trailing ".git" suffix.
func parseHTTPS(input string) (*Source, error) {
	u, err := url.Parse(input)
	if err != nil {
		return nil, fmt.Errorf("invalid HTTPS URL %q: %w", input, err)
	}

	if u.Host == "" {
		return nil, fmt.Errorf("invalid HTTPS URL %q: missing host", input)
	}

	if err := validateURLPath(u.Path, input); err != nil {
		return nil, err
	}

	return &Source{CloneURL: strings.TrimSuffix(input, ".git")}, nil
}

// parseSSH stores an SSH URL as-is, stripping a trailing ".git" suffix.
func parseSSH(input string) (*Source, error) {
	if strings.HasPrefix(input, "ssh://") {
		u, err := url.Parse(input)
		if err != nil {
			return nil, fmt.Errorf("invalid SSH URL %q: %w", input, err)
		}

		if u.Host == "" {
			return nil, fmt.Errorf("invalid SSH URL %q: missing host", input)
		}

		if err := validateURLPath(u.Path, input); err != nil {
			return nil, err
		}
	} else {
		// SCP-style: user@host:path
		colonIdx := strings.Index(input, ":")

		path := input[colonIdx+1:]
		if err := validateScpPath(path, input); err != nil {
			return nil, err
		}
	}

	return &Source{CloneURL: strings.TrimSuffix(input, ".git")}, nil
}

// validateURLPath checks that a URL path has at least two non-empty segments (owner and repo).
func validateURLPath(path, input string) error {
	p := strings.TrimPrefix(path, "/")
	p = strings.TrimSuffix(p, ".git")

	if countNonEmptySegments(p) < 2 {
		return fmt.Errorf("invalid URL %q: path must have at least two non-empty segments (owner/repo)", input)
	}

	return nil
}

// validateScpPath checks that an SCP-style path (no leading slash) has at least two non-empty segments.
func validateScpPath(path, input string) error {
	p := strings.TrimSuffix(path, ".git")
	if countNonEmptySegments(p) < 2 {
		return fmt.Errorf("invalid SSH URL %q: path must have at least two non-empty segments (owner/repo)", input)
	}

	return nil
}

func countNonEmptySegments(path string) int {
	n := 0

	for seg := range strings.SplitSeq(path, "/") {
		if seg != "" {
			n++
		}
	}

	return n
}

// isScpStyle detects SCP-style SSH URLs (git@host:path) — has "@" before ":" and no "://" scheme.
func isScpStyle(input string) bool {
	atIdx := strings.Index(input, "@")
	colonIdx := strings.Index(input, ":")

	return atIdx > 0 && colonIdx > atIdx && !strings.Contains(input, "://")
}
