package host

import (
	"errors"
	"fmt"
	"strings"
)

// ErrInvalidSource is returned when a source string cannot be parsed.
var ErrInvalidSource = errors.New("invalid template source")

// Source represents a parsed template source.
type Source struct {
	CloneURL  string // HTTPS clone URL; empty for local sources
	Branch    string // branch or tag to check out; empty means default branch
	IsLocal   bool   // true when the source is a local file path
	LocalPath string // resolved local path; only set when IsLocal is true
}

// Parse parses a source string into a Source.
//
// Supported formats:
//
//	github:user/repo            → https://github.com/user/repo.git
//	github:user/repo:branch     → same, checking out the given branch or tag
//	https://github.com/user/repo → normalised to HTTPS clone URL
//	file:./path                 → local path
//	./path  or  /abs/path       → local path (implicit)
func Parse(s string) (*Source, error) {
	switch {
	case strings.HasPrefix(s, "github:"):
		return parseGitHub(strings.TrimPrefix(s, "github:"))
	case strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://"):
		return parseHTTPS(s)
	case strings.HasPrefix(s, "file:"):
		return &Source{IsLocal: true, LocalPath: strings.TrimPrefix(s, "file:")}, nil
	default:
		// Treat anything else as a local path (e.g. ./my-template or /abs/path).
		return &Source{IsLocal: true, LocalPath: s}, nil
	}
}

// parseGitHub handles the "user/repo" or "user/repo:branch" part after
// stripping the "github:" prefix.
func parseGitHub(s string) (*Source, error) {
	parts := strings.SplitN(s, ":", 2)
	repoPart := parts[0]

	if !strings.Contains(repoPart, "/") {
		return nil, fmt.Errorf("%w: github source must be in user/repo format, got %q", ErrInvalidSource, s)
	}

	src := &Source{
		CloneURL: "https://github.com/" + repoPart + ".git",
	}
	if len(parts) == 2 {
		src.Branch = parts[1]
	}
	return src, nil
}

// parseHTTPS handles full HTTP/HTTPS repository URLs.
// The URL is normalised to end with ".git" if it does not already.
func parseHTTPS(s string) (*Source, error) {
	cloneURL := s
	if !strings.HasSuffix(cloneURL, ".git") {
		cloneURL += ".git"
	}
	return &Source{CloneURL: cloneURL}, nil
}
