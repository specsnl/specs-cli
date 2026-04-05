# Phase 5 — Git & Host Utilities

## Goal

Parse source strings (GitHub shorthands, HTTPS URLs, local paths) into a normalised
`Source` type, and clone remote repositories to disk using `go-git`. These packages form
the foundation for `specs template download` and `specs use` (Phase 6).

## Done criteria

- `host.Parse()` correctly identifies all source formats and returns a populated `Source`.
- `git.Clone()` clones a remote repository to a target directory.
- `git.Clone()` supports cloning a specific branch.
- All `pkg/host` tests pass without network access.
- The `pkg/util/git` integration test is guarded by a build tag and can be run separately.

---

## Dependencies

```
go get github.com/go-git/go-git/v5
```

---

## File overview

```
pkg/
├── host/
│   ├── source.go
│   └── source_test.go
└── util/
    └── git/
        ├── git.go
        └── git_test.go
```

---

## Source formats

| Input | Kind | Clone URL | Branch |
|---|---|---|---|
| `github:user/repo` | GitHub shorthand | `https://github.com/user/repo` | default |
| `github:user/repo:main` | GitHub shorthand + branch | `https://github.com/user/repo` | `main` |
| `https://github.com/user/repo` | Full HTTPS URL | as-is | default |
| `https://github.com/user/repo.git` | Full HTTPS URL with .git | stripped to `.../repo` | default |
| `file:./my-template` | Local path (explicit prefix) | — | — |
| `./my-template` | Local path (relative) | — | — |
| `/absolute/path` | Local path (absolute) | — | — |

Any other input is an error.

---

## Files

### `pkg/host/source.go`

```go
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

// Parse parses a source string into a Source.
//
// Accepted formats:
//   - github:user/repo              GitHub shorthand
//   - github:user/repo:branch       GitHub shorthand with branch
//   - https://github.com/user/repo  Full HTTPS URL (optional .git suffix is stripped)
//   - file:./path                   Explicit local-path prefix
//   - ./path  ../path  /path        Implicit local path (relative or absolute)
func Parse(input string) (*Source, error) {
    switch {
    case strings.HasPrefix(input, "github:"):
        return parseGitHub(input)
    case strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://"):
        return parseHTTPS(input)
    case strings.HasPrefix(input, "file:"):
        return &Source{LocalPath: strings.TrimPrefix(input, "file:")}, nil
    case strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") || strings.HasPrefix(input, "/"):
        return &Source{LocalPath: input}, nil
    default:
        return nil, fmt.Errorf("unrecognised source format %q — use github:user/repo, an HTTPS URL, or a local path", input)
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
```

---

### `pkg/util/git/git.go`

```go
package git

import (
    "fmt"

    gogit "github.com/go-git/go-git/v5"
    "github.com/go-git/go-git/v5/plumbing"
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

    _, err := gogit.PlainClone(dir, false, cloneOpts)
    if err != nil {
        return fmt.Errorf("cloning %s: %w", url, err)
    }
    return nil
}
```

**Why `Depth: 1` by default:** Template consumers only need the latest commit. A shallow
clone is significantly faster and avoids pulling the entire git history of large repos.

**Why `SingleBranch: true` when a branch is given:** Without it, go-git fetches all
remote refs even when `ReferenceName` is set — negating the speed benefit of specifying
a branch.

---

## Tests

### `pkg/host/source_test.go`

All tests are pure unit tests — no network access required.

```go
package host_test

import (
    "testing"

    "github.com/specsnl/specs-cli/pkg/host"
)

func TestParse(t *testing.T) {
    tests := []struct {
        name      string
        input     string
        wantURL   string
        wantBranch string
        wantLocal string
        wantErr   bool
    }{
        {
            name:    "github shorthand",
            input:   "github:user/repo",
            wantURL: "https://github.com/user/repo",
        },
        {
            name:       "github shorthand with branch",
            input:      "github:user/repo:main",
            wantURL:    "https://github.com/user/repo",
            wantBranch: "main",
        },
        {
            name:    "full https url",
            input:   "https://github.com/user/repo",
            wantURL: "https://github.com/user/repo",
        },
        {
            name:    "https url with .git suffix",
            input:   "https://github.com/user/repo.git",
            wantURL: "https://github.com/user/repo",
        },
        {
            name:      "file prefix local path",
            input:     "file:./my-template",
            wantLocal: "./my-template",
        },
        {
            name:      "relative local path",
            input:     "./my-template",
            wantLocal: "./my-template",
        },
        {
            name:      "parent relative path",
            input:     "../my-template",
            wantLocal: "../my-template",
        },
        {
            name:      "absolute local path",
            input:     "/home/user/templates/my-template",
            wantLocal: "/home/user/templates/my-template",
        },
        {
            name:    "unknown format",
            input:   "git@github.com:user/repo.git",
            wantErr: true,
        },
        {
            name:    "github shorthand missing slash",
            input:   "github:repo-only",
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            src, err := host.Parse(tt.input)
            if tt.wantErr {
                if err == nil {
                    t.Fatalf("Parse(%q) = nil error, want error", tt.input)
                }
                return
            }
            if err != nil {
                t.Fatalf("Parse(%q) error: %v", tt.input, err)
            }
            if src.CloneURL != tt.wantURL {
                t.Errorf("CloneURL = %q, want %q", src.CloneURL, tt.wantURL)
            }
            if src.Branch != tt.wantBranch {
                t.Errorf("Branch = %q, want %q", src.Branch, tt.wantBranch)
            }
            if src.LocalPath != tt.wantLocal {
                t.Errorf("LocalPath = %q, want %q", src.LocalPath, tt.wantLocal)
            }
        })
    }
}

func TestSource_IsLocal(t *testing.T) {
    local, _ := host.Parse("./my-template")
    if !local.IsLocal() {
        t.Error("./my-template should be local")
    }

    remote, _ := host.Parse("github:user/repo")
    if remote.IsLocal() {
        t.Error("github:user/repo should not be local")
    }
}
```

---

### `pkg/util/git/git_test.go`

The integration test is guarded by the `integration` build tag. It is skipped in CI by
default; run with `go test -tags=integration ./pkg/util/git/...`.

```go
//go:build integration

package git_test

import (
    "os"
    "testing"

    pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

// TestClone_ShallowClone clones a small public repository and verifies the result.
// Requires network access. Run with: go test -tags=integration ./pkg/util/git/...
func TestClone_ShallowClone(t *testing.T) {
    dir := t.TempDir()

    err := pkggit.Clone("https://github.com/specsnl/specs-cli", dir, pkggit.CloneOptions{
        Depth: 1,
    })
    if err != nil {
        t.Fatalf("Clone: %v", err)
    }

    // Verify the repo has content.
    if _, err := os.Stat(dir + "/go.mod"); os.IsNotExist(err) {
        t.Error("cloned repo missing go.mod")
    }
}

func TestClone_SpecificBranch(t *testing.T) {
    dir := t.TempDir()

    err := pkggit.Clone("https://github.com/specsnl/specs-cli", dir, pkggit.CloneOptions{
        Branch: "main",
        Depth:  1,
    })
    if err != nil {
        t.Fatalf("Clone with branch: %v", err)
    }

    if _, err := os.Stat(dir + "/go.mod"); os.IsNotExist(err) {
        t.Error("cloned repo missing go.mod")
    }
}

func TestClone_InvalidURL(t *testing.T) {
    dir := t.TempDir()

    err := pkggit.Clone("https://github.com/specsnl/this-repo-does-not-exist-xyz", dir, pkggit.CloneOptions{})
    if err == nil {
        t.Fatal("expected error for non-existent repository, got nil")
    }
}
```

---

## Key notes

- **No `ssh://` support.** Only HTTPS clone URLs are supported in v2. SSH/token auth is
  deferred (see [07-issues-and-prs.md](../07-issues-and-prs.md)).
- **Local paths are not cloned.** `host.Parse()` distinguishes remote from local sources.
  The caller (Phase 6 commands) copies local paths with `osutil` instead of calling
  `git.Clone()`.
- **`Depth: 0` is a full clone**, not the default. Always pass `Depth: 1` from command
  handlers unless there is a specific reason for a full history.
- **Integration test repo.** The test clones `specsnl/specs-cli` itself because it is
  guaranteed to exist and be a small repo. If the repo is renamed, update the test.
- **`go-git` does not need `git` on PATH.** It is a pure-Go implementation — no system
  dependency on the `git` binary.
- **`host` package is import-only.** It has no side effects and no global state — safe to
  use from any goroutine.
