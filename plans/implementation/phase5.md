# Phase 5 — Git & Host Utilities

## Goal

Parse source strings (GitHub shorthands, HTTPS URLs, SSH URLs, local paths) into a
normalised `Source` type, and clone remote repositories to disk using `go-git`. SSH URLs
are detected automatically and authenticated via SSH agent or standard key files.
These packages form the foundation for `specs template download` and `specs use` (Phase 6).

## Done criteria

- `host.Parse()` correctly identifies all source formats and returns a populated `Source`.
- `host.Source.IsSSH()` correctly identifies SSH sources.
- `git.Clone()` clones a remote repository to a target directory.
- `git.Clone()` supports cloning a specific branch or git tag via the `Branch` field.
- `git.Clone()` resolves the ref as a git tag first (`refs/tags/<ref>`); falls back to a branch (`refs/heads/<ref>`) if the tag is not found.
- `git.Clone()` automatically builds SSH auth from the URL — no caller changes needed.
- Auth strategy: SSH agent first, then `~/.ssh/id_ed25519` / `id_rsa` / `id_ecdsa`.
- Host key verification uses `~/.ssh/known_hosts` — verification is never skipped.
- All `pkg/host` tests pass without network access.
- The `pkg/util/git` integration tests are guarded by a build tag and only run inside the
  Docker container (`/.dockerenv` must be present).

---

## Dependencies

```
go get github.com/go-git/go-git/v5
```

No additional packages needed for SSH — all SSH-related libraries are already present as
transitive dependencies of `go-git`:

- `github.com/go-git/go-git/v5/plumbing/transport/ssh` — SSH auth methods
- `golang.org/x/crypto/ssh/knownhosts` — known_hosts parsing
- `github.com/xanzy/ssh-agent` — SSH agent support

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

| Input | Kind | CloneURL stored | Branch/Tag |
|---|---|---|---|
| `github:user/repo` | GitHub shorthand | `https://github.com/user/repo` | default |
| `github:user/repo:main` | GitHub shorthand + branch | `https://github.com/user/repo` | `main` |
| `github:user/repo:0.1.0` | GitHub shorthand + tag | `https://github.com/user/repo` | `0.1.0` |
| `https://github.com/user/repo` | Full HTTPS URL | as-is | default |
| `https://github.com/user/repo.git` | Full HTTPS URL with .git | stripped | default |
| `git@github.com:user/repo` | SCP-style SSH | `git@github.com:user/repo` | default |
| `git@github.com:user/repo.git` | SCP-style SSH with .git | stripped | default |
| `ssh://git@github.com/user/repo` | Explicit SSH scheme | as-is | default |
| `file:./my-template` | Local path (explicit prefix) | — | — |
| `./my-template` | Local path (relative) | — | — |
| `/absolute/path` | Local path (absolute) | — | — |

Any other input is an error.

---

## Key notes

- **SSH auth is automatic.** `git.Clone()` detects SSH URLs by scheme/format and builds
  auth without any caller changes. No new `CloneOptions` fields.
- **No insecure host key skip.** `~/.ssh/known_hosts` is required for SSH clones.
- **No passphrase prompting.** Encrypted key files without an agent are skipped silently.
  Users with passphrase-protected keys should use `ssh-agent`.
- **Tag-first ref resolution.** `cloneWithRef` tries `refs/tags/<ref>` first. If go-git
  returns "couldn't find remote ref", it cleans up the partial clone and retries as
  `refs/heads/<ref>`. This matches native `git clone -b` priority (tags win over same-named
  branches) while keeping the API simple — callers just pass the ref string without caring
  whether it is a branch or tag.
- **SSH clone not integration-tested.** Wiring up a self-contained SSH server requires
  credentials or a local sshd. SSH URL parsing is covered by `pkg/host` unit tests;
  SSH transport correctness is delegated to go-git's own test suite. See `TestClone_SSH`.
- **Local paths are not cloned.** `host.Parse()` distinguishes remote from local sources.
  The caller (Phase 6 commands) copies local paths with `osutil` instead of calling
  `git.Clone()`.
- **`Depth: 0` is a full clone**, not the default. Always pass `Depth: 1` from command
  handlers unless there is a specific reason for a full history.
- **`go-git` does not need `git` on PATH.** It is a pure-Go implementation.
- **`host` package is import-only.** No side effects, no global state — safe from any goroutine.

---

## Verification

```bash
# Unit tests (no network required)
go test ./pkg/host/...

# Build check
go build ./...

# Integration tests (run inside Docker via task docker:test)
go test -tags=integration ./pkg/util/git/...
```
