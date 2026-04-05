# Phase 5 — Git & Host Utilities

## Goal

Parse template source strings (GitHub shorthand, full HTTPS URLs, local paths) and clone
remote repositories to disk using `go-git`.

## Done criteria

- `host.Parse(source)` returns a `*Source` for all supported source formats.
- `host.Parse` returns `ErrInvalidSource` for malformed GitHub shorthand.
- `git.Clone(url, dir, branch)` clones a repository into `dir` using `go-git`.
- When `branch` is non-empty, that branch or tag is checked out.
- All tests pass.

---

## Dependencies

```
go get github.com/go-git/go-git/v5@v5.17.2
```

---

## File overview

```
pkg/
├── host/
│   ├── github.go
│   └── github_test.go
└── util/
    └── git/
        ├── git.go
        └── git_test.go
```

---

## Files

### `pkg/host/github.go`

Parses a source string into a `Source` struct.

```go
type Source struct {
    CloneURL  string // HTTPS clone URL; empty for local sources
    Branch    string // branch or tag to check out; empty means default branch
    IsLocal   bool   // true when the source is a local file path
    LocalPath string // resolved local path; only set when IsLocal is true
}

func Parse(s string) (*Source, error)
```

Supported formats:

| Input | CloneURL | Branch | IsLocal | LocalPath |
|-------|----------|--------|---------|-----------|
| `github:user/repo` | `https://github.com/user/repo.git` | `` | false | `` |
| `github:user/repo:main` | `https://github.com/user/repo.git` | `main` | false | `` |
| `https://github.com/user/repo` | `https://github.com/user/repo.git` | `` | false | `` |
| `https://github.com/user/repo.git` | `https://github.com/user/repo.git` | `` | false | `` |
| `file:./path` | `` | `` | true | `./path` |
| `./path` | `` | `` | true | `./path` |
| `/abs/path` | `` | `` | true | `/abs/path` |

---

### `pkg/util/git/git.go`

Thin wrapper around `go-git`:

```go
// Clone clones the repository at url into dir.
// If branch is non-empty, that branch or tag is checked out instead of the
// repository's default branch.
func Clone(url, dir, branch string) error
```

---

## Tests

### `pkg/host/github_test.go`

Table-driven unit tests covering all supported source formats and the
`ErrInvalidSource` error case for malformed GitHub shorthand
(`github:justarepo` — missing the `user/` prefix).

### `pkg/util/git/git_test.go`

Integration test that clones a small public repository into a `t.TempDir()`
and verifies that the resulting `.git` directory is present.

The test is skipped automatically when `-short` is passed:

```
go test -short ./...
```

To run the integration test explicitly:

```
go test ./pkg/util/git/...
```

---

## Key notes

- `pkg/host` has **no external dependencies** — it only uses the standard
  library. Source format parsing is pure string manipulation.
- `pkg/util/git` wraps `go-git` with a minimal surface area. The caller
  (`specs use`, `specs template download`) owns the target directory lifecycle.
- The `branch` parameter is resolved first as a branch (`refs/heads/<name>`).
  If that reference does not exist on the remote, any partial state is cleaned
  up and the same value is retried as a tag (`refs/tags/<name>`). This means
  `github:user/repo:v1.2.3` works correctly for both branch and tag names.
- The integration test targets the public `specsnl/specs-cli` repository.
  Swap this for any small public repo if needed; the assertion is simply that
  `.git` exists after the clone.
