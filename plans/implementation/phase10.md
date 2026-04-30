# Phase 10 — Template status tracking, update & upgrade

## Goal

Give users visibility into whether their registered templates are outdated, and let them
apply updates. Three capabilities are added together because they share the same
infrastructure (remote ref checking, status caching):

1. A **Status** column in `specs template list` that shows whether each template is
   up-to-date, has an update available, or had a check error.
2. `specs template update [name]` — manually refresh the cached status (like `git fetch`).
3. `specs template upgrade <name> | --all` — actually apply the update by re-cloning.

Remote checks happen **at most once per day** when `template list` runs; `template update`
forces an immediate refresh.

---

## Done criteria

- `specs template list` shows a Status column.
- For remote templates whose cached status is older than 24 hours, the status is
  re-fetched automatically (in parallel) when `template list` runs.
- For local templates (saved via `template save`), Status shows `-`.
- `specs template update` refreshes the status cache; `specs template update <name>`
  refreshes a single template.
- `specs template upgrade <name>` re-clones the template to the latest commit on its
  branch (or to the latest semver tag if the template was downloaded from a tag ref).
- `specs template upgrade --all` upgrades every remote template; local templates are
  skipped with an informational message.
- Network failures (DNS errors, timeouts, no route) produce a **single** warning line
  below the table — not one error per template.
- Per-template errors (auth failure, repository not found) show in the Status column.
- The `Branch` ref used during download is persisted in `__metadata.json` and used by
  all subsequent status checks and upgrades.
- All tests pass; no regressions in existing commands.

---

## File overview

```
pkg/
├── specs/
│   └── configuration.go       (modified — add StatusFile constant)
├── template/
│   ├── metadata.go            (modified — add Branch field)
│   └── status.go              (new — TemplateStatus struct, load/save/IsStale)
├── util/git/
│   └── git.go                 (modified — add CheckErrorKind, RemoteCheckResult, CheckRemote)
└── cmd/
    ├── metadata.go            (modified — add branch param to writeMetadata)
    ├── template_download.go   (modified — pass src.Branch to writeMetadata)
    ├── template_save.go       (modified — pass "" for branch)
    ├── template_list.go       (modified — Status column, parallel refresh, error grouping)
    ├── template_update.go     (new — specs template update)
    ├── template_upgrade.go    (new — specs template upgrade)
    └── template.go            (modified — register two new subcommands)
```

---

## Step 1 — Add `Branch` to Metadata

### `pkg/specs/configuration.go`

Add alongside the existing file name constants:

```go
StatusFile = "__status.json"
```

### `pkg/template/metadata.go`

Add `Branch` after `Repository`:

```go
type Metadata struct {
    Name       string   `json:"Name"`
    Repository string   `json:"Repository"`
    Branch     string   `json:"Branch,omitempty"`
    Created    JSONTime `json:"Created"`
    Commit     string   `json:"Commit,omitempty"`
    Version    string   `json:"Version,omitempty"`
}
```

### `pkg/cmd/metadata.go`

```go
func writeMetadata(templateRoot, name, repository, branch, commit, version string) error {
    m := pkgtemplate.Metadata{
        Name:       name,
        Repository: repository,
        Branch:     branch,
        Created:    pkgtemplate.JSONTime{Time: time.Now().UTC()},
        Commit:     commit,
        Version:    version,
    }
    // ... marshal and write as before
}
```

### `pkg/cmd/template_download.go`

Change the `writeMetadata` call (line 53) to pass `src.Branch`:

```go
if err := writeMetadata(dest, name, src.CloneURL, src.Branch, desc.Commit, desc.Version); err != nil {
    return err
}
```

### `pkg/cmd/template_save.go`

Pass `""` for branch (local saves have no remote branch):

```go
if err := writeMetadata(dest, name, src.LocalPath, "", desc.Commit, desc.Version); err != nil {
    return err
}
```

---

## Step 2 — Status file infrastructure

### `pkg/template/status.go` (new)

```go
package template

import (
    "encoding/json"
    "os"
    "path/filepath"
    "time"

    "github.com/specsnl/specs-cli/pkg/specs"
    pkggit "github.com/specsnl/specs-cli/pkg/util/git"
)

// TemplateStatus is the cached result of the most recent remote status check.
// Stored in __status.json inside each template directory.
type TemplateStatus struct {
    CheckedAt     JSONTime              `json:"CheckedAt"`
    IsUpToDate    bool                  `json:"IsUpToDate"`
    LatestVersion string                `json:"LatestVersion,omitempty"` // semver tags: newer tag if one exists
    ErrorKind     pkggit.CheckErrorKind `json:"ErrorKind,omitempty"`     // non-empty when last check failed
}

// IsStale returns true when the cached status is older than 24 hours.
func (s *TemplateStatus) IsStale() bool {
    return time.Since(s.CheckedAt.Time) > 24*time.Hour
}

// LoadStatus reads __status.json from templateRoot.
// Missing file is not an error — returns nil, nil.
func LoadStatus(templateRoot string) (*TemplateStatus, error) {
    data, err := os.ReadFile(filepath.Join(templateRoot, specs.StatusFile))
    if os.IsNotExist(err) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    var s TemplateStatus
    if err := json.Unmarshal(data, &s); err != nil {
        return nil, err
    }
    return &s, nil
}

// SaveStatus writes __status.json into templateRoot.
func SaveStatus(templateRoot string, s *TemplateStatus) error {
    data, err := json.MarshalIndent(s, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile(filepath.Join(templateRoot, specs.StatusFile), data, 0644)
}
```

---

## Step 3 — Remote status check in `pkg/util/git`

### `pkg/util/git/git.go`

Add after the existing `DescribeResult` block:

```go
// CheckErrorKind classifies why a remote status check failed.
type CheckErrorKind string

const (
    CheckErrorNone     CheckErrorKind = ""
    CheckErrorNetwork  CheckErrorKind = "network"   // DNS failure, no route, timeout
    CheckErrorAuth     CheckErrorKind = "auth"       // authentication / authorisation rejected
    CheckErrorNotFound CheckErrorKind = "not-found"  // repository no longer exists
    CheckErrorUnknown  CheckErrorKind = "unknown"
)

// RemoteCheckResult is the outcome of CheckRemote.
type RemoteCheckResult struct {
    IsUpToDate    bool
    LatestVersion string         // non-empty only for semver tags when a newer one exists
    ErrorKind     CheckErrorKind // non-empty when the check could not complete
}

// CheckRemote queries the remote to determine whether the local repo at dir is
// up-to-date for the given branch/tag ref. It uses Remote.List() and never
// modifies the local repository. SSH auth is resolved automatically.
//
// On failure, ErrorKind is set in the result and error is nil — callers should
// inspect ErrorKind rather than the returned error, which is always nil.
func CheckRemote(dir, url, branch string) (RemoteCheckResult, error) {
    repo, err := gogit.PlainOpen(dir)
    if err != nil {
        return RemoteCheckResult{ErrorKind: CheckErrorUnknown}, nil
    }

    remote, err := repo.Remote("origin")
    if err != nil {
        return RemoteCheckResult{ErrorKind: CheckErrorUnknown}, nil
    }

    listOpts := &gogit.ListOptions{}
    if isSSHURL(url) {
        auth, err := sshAuth(url)
        if err != nil {
            return RemoteCheckResult{ErrorKind: CheckErrorAuth}, nil
        }
        listOpts.Auth = auth
    }

    refs, err := remote.List(listOpts)
    if err != nil {
        return RemoteCheckResult{ErrorKind: classifyRemoteError(err)}, nil
    }

    head, err := repo.Head()
    if err != nil {
        return RemoteCheckResult{ErrorKind: CheckErrorUnknown}, nil
    }

    return resolveStatus(refs, head.Hash(), branch), nil
}

// classifyRemoteError maps a remote.List error to a CheckErrorKind.
func classifyRemoteError(err error) CheckErrorKind {
    var netErr *net.OpError
    if errors.As(err, &netErr) {
        return CheckErrorNetwork
    }
    switch {
    case errors.Is(err, transport.ErrAuthenticationRequired),
        errors.Is(err, transport.ErrAuthorizationFailed):
        return CheckErrorAuth
    case errors.Is(err, transport.ErrRepositoryNotFound):
        return CheckErrorNotFound
    }
    return CheckErrorUnknown
}

// resolveStatus compares remote refs against the local HEAD for the given ref.
// Tag-first resolution is used, consistent with Clone behaviour.
func resolveStatus(refs []*plumbing.Reference, localHead plumbing.Hash, ref string) RemoteCheckResult {
    tagRef := plumbing.NewTagReferenceName(ref)
    branchRef := plumbing.NewBranchReferenceName(ref)

    // Collect remote tags for semver comparison.
    remoteTags := map[string]struct{}{}
    for _, r := range refs {
        if r.Name().IsTag() {
            remoteTags[r.Name().Short()] = struct{}{}
        }
    }

    // Tag-first: if the remote has this ref as a tag, treat as semver template.
    for _, r := range refs {
        if r.Name() == tagRef {
            latest := latestSemverTag(remoteTags, ref)
            if latest == "" || latest == ref {
                return RemoteCheckResult{IsUpToDate: true}
            }
            return RemoteCheckResult{IsUpToDate: false, LatestVersion: latest}
        }
    }

    // Branch fallback.
    for _, r := range refs {
        if r.Name() == branchRef {
            return RemoteCheckResult{IsUpToDate: r.Hash() == localHead}
        }
    }

    return RemoteCheckResult{ErrorKind: CheckErrorNotFound}
}

// latestSemverTag returns the highest semver tag in tags that is strictly greater
// than current. Returns "" if current is already the latest or cannot be parsed.
func latestSemverTag(tags map[string]struct{}, current string) string {
    // Uses the semver package already available via the sprout dependency.
    // Implementation: parse each tag as semver, return max > current.
    // Returns "" if current is already the max or no valid semver tags exist.
    ...
}
```

**Required additional imports** in `git.go`:
- `"errors"`
- `"net"`
- `"github.com/go-git/go-git/v5/plumbing/transport"`

---

## Step 4 — Enhance `template list`

### `pkg/cmd/template_list.go`

Key changes:

1. Add `"Status"` between `"Version"` and `"Created"` in headers.
2. For each template, load `__status.json` alongside `__metadata.json`.
3. Identify stale remote templates (have a `Repository` + `Branch`, and status is nil or `IsStale()`).
4. Fetch stale statuses concurrently with a `sync.WaitGroup`; write results back to `__status.json`.
5. After all fetches, group errors: if any result has `CheckErrorNetwork`, print a single trailing warning. Per-template `CheckErrorAuth` / `CheckErrorNotFound` / `CheckErrorUnknown` are shown in the Status column.

```go
// statusLabel returns the Status column string for a template.
func statusLabel(status *pkgtemplate.TemplateStatus, hasRemote bool) string {
    if !hasRemote {
        return "-"
    }
    if status == nil {
        return "unknown"
    }
    switch status.ErrorKind {
    case pkggit.CheckErrorNetwork:
        return "unknown (offline?)"
    case pkggit.CheckErrorAuth:
        return "auth error"
    case pkggit.CheckErrorNotFound:
        return "not found"
    case pkggit.CheckErrorUnknown:
        return "check failed"
    }
    if status.IsUpToDate {
        return "up-to-date"
    }
    if status.LatestVersion != "" {
        return "update: " + status.LatestVersion
    }
    return "update available"
}
```

Network-error grouping after the table is rendered:

```go
if networkErrorSeen {
    output.Warn("could not reach one or more remotes — status may be outdated")
}
```

---

## Step 5 — `specs template update`

### `pkg/cmd/template_update.go` (new)

```
specs template update [name]
```

- No name: refresh all remote templates (those with a non-empty `Repository` and `Branch`).
- With name: refresh that specific template only.
- Runs `pkggit.CheckRemote()`, saves updated `__status.json` for each.
- After all checks, applies the same error-kind grouping as `template list`:
  - One combined network warning if any `CheckErrorNetwork` result is seen.
  - Per-template `output.Warn` for auth / not-found errors.
- Prints which templates have updates available; prints `"all templates are up-to-date"` if none.

---

## Step 6 — `specs template upgrade`

### `pkg/cmd/template_upgrade.go` (new)

```
specs template upgrade <name>
specs template upgrade --all
```

- Errors if both a name and `--all` are given, or if neither is given.
- For each target template:
  1. Load `__metadata.json`. Skip with a notice if `Repository` or `Branch` is empty
     (local template, no upgrade path).
  2. Determine the target ref:
     - For **semver tag** templates: call `pkggit.CheckRemote()` to find `LatestVersion`.
       If `LatestVersion` is non-empty, use it; otherwise keep the current tag (already latest).
     - For **branch** templates: use `meta.Branch` unchanged.
  3. Remove the existing template directory.
  4. Call `pkggit.Clone(meta.Repository, dest, pkggit.CloneOptions{Branch: targetRef})`.
  5. Call `writeMetadata(dest, name, meta.Repository, meta.Branch, desc.Commit, desc.Version)`.
     For semver upgrades where the tag changed, update `meta.Branch` to the new tag.
  6. Remove `__status.json` (stale after re-clone; regenerated on next `template list`).
  7. Print `"template %q upgraded"` (or `"template %q is already up-to-date"` when no newer version).
- `--all` flag: iterates all templates; errors (auth, not-found, network) are logged per
  template and the command continues with remaining templates.

---

## Step 7 — Register new commands

### `pkg/cmd/template.go`

Add alongside the existing `AddCommand` calls:

```go
cmd.AddCommand(newTemplateUpdateCmd())
cmd.AddCommand(newTemplateUpgradeCmd())
```

---

## Tests

### `pkg/util/git/remote_check_test.go` — `classifyRemoteError` and `resolveStatus`

| Test | Scenario |
|---|---|
| `TestClassifyRemoteError_Network` | `&net.OpError{...}` → `CheckErrorNetwork` |
| `TestClassifyRemoteError_Auth_AuthenticationRequired` | `transport.ErrAuthenticationRequired` → `CheckErrorAuth` |
| `TestClassifyRemoteError_Auth_AuthorizationFailed` | `transport.ErrAuthorizationFailed` → `CheckErrorAuth` |
| `TestClassifyRemoteError_NotFound` | `transport.ErrRepositoryNotFound` → `CheckErrorNotFound` |
| `TestClassifyRemoteError_Unknown` | `errors.New("something else")` → `CheckErrorUnknown` |
| `TestResolveStatus_BranchUpToDate` | branch hash matches local HEAD → `IsUpToDate: true` |
| `TestResolveStatus_BranchBehind` | branch hash differs → `IsUpToDate: false` |
| `TestResolveStatus_TagAlreadyLatest` | tag ref present, no newer semver tag → `IsUpToDate: true` |
| `TestResolveStatus_TagNewerExists` | newer semver tag exists → `IsUpToDate: false, LatestVersion: "v2.0.0"` |
| `TestResolveStatus_RefNotFound` | neither tag nor branch in remote refs → `CheckErrorNotFound` |
| `TestLatestSemverTag_NewerExists` | v1.1.0 current, v2.0.0 available → returns `"v2.0.0"` |
| `TestLatestSemverTag_AlreadyLatest` | v1.1.0 is highest → returns `""` |
| `TestLatestSemverTag_InvalidCurrent` | non-semver current → returns `""` |

### `pkg/template/status_test.go`

| Test | Scenario |
|---|---|
| `TestIsStale_Fresh` | `CheckedAt` 1 hour ago → false |
| `TestIsStale_Old` | `CheckedAt` 25 hours ago → true |
| `TestLoadStatus_Missing` | no `__status.json` → nil, nil |
| `TestStatusRoundtrip` | save + load returns same struct |

### `pkg/cmd/template_list_test.go`

| Test | Scenario |
|---|---|
| `TestStatusLabel` | table-driven: all 9 branches of `statusLabel` (no remote, nil status, network/auth/not-found/unknown errors, up-to-date, update with version, update available) |
| `TestList_StatusColumn_FreshUpToDate` | remote template with fresh `__status.json` showing up-to-date |
| `TestList_StatusColumn_LocalNoStatus` | local template (no branch) shows `-` in status column |
| `TestList_StatusColumn_NetworkWarn` | stale status causes CheckRemote call; command succeeds without panic |

### `pkg/cmd/template_update_test.go`

| Test | Scenario |
|---|---|
| `TestUpdate_NoArgs_EmptyRegistry` | no args on empty registry → succeeds |
| `TestUpdate_NamedLocalTemplate_Skipped` | local template (no branch) is silently skipped |
| `TestUpdate_TooManyArgs` | two positional args → error |

### `pkg/cmd/template_upgrade_test.go`

| Test | Scenario |
|---|---|
| `TestUpgrade_LocalSkipped` | template with empty `Branch` → skipped with notice |
| `TestUpgrade_AllFlagMutualExclusion` | `upgrade --all mytemplate` → error |
| `TestUpgrade_NeitherAllNorName` | no args and no `--all` → error |
| `TestUpgrade_NonexistentTemplate` | named template not in registry → error |
