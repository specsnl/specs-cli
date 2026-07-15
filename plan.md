# PR #69 follow-up: logger architecture + review fixes

## Context

PR #69 (issue [#51](https://github.com/specsnl/specs-cli/issues/51)) adds structured
logging across the CLI but mixes two patterns: an injected `App.Logger` threaded
through `pkgtemplate.Get`, `hooks.Load`, etc., and direct `slog.Default()` calls in
`pkg/registry` and the hook fallback path. The branch sets `slog.SetDefault(app.Logger)`
at the end of `PersistentPreRunE` (`pkg/cmd/root.go:69`), so both pipes exist
side-by-side — the worst of both worlds.

A code review also flagged ten concrete problems (swallowed `CheckRemote` errors,
mixed attribute keys, no logging in `pkg/util/git`, weak `registry.Upgrade()` coverage,
inconsistent `LoadStatus` failure logging, prompt-source semantics, missing tests,
remaining `, _ :=` audit, docs not matching emitted attributes).

Issue [#67](https://github.com/specsnl/specs-cli/issues/67) (rename `pkg/` →
`internal/`) removes the only meaningful argument for keeping a logger-injection
API — there are no external consumers to consider. The CLI is a single-process
binary; `log/slog` is built around the default-logger model.

This plan does the architectural cleanup **and** the ten review fixes in a single
amend on the current branch (`51-improve-observability-via-structured-logging`).

## Decision: drop logger injection, commit to `slog.Default()`

After the migration, no production function takes a `*slog.Logger` parameter and no
struct stores one. Every package emits logs via the package-level `slog.Debug/Info/...`
functions, which route through the global default. The CLI entrypoint configures the
default logger once at startup and swaps it when `--debug --output=json` flips the
handler. Tests silence logs with a `TestMain` per package that calls
`slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))`.

The `LevelVar` and `HandlerFactory` machinery on `App` is retained because runtime
level changes from `--debug` still need to mutate the live handler.

---

## Implementation

### 1. Logger plumbing collapse

**`pkg/cmd/app.go`** (`pkg/cmd/app.go:49-99`)
- Remove the `Logger *slog.Logger` field from `App`.
- `NewApp()` no longer stores a logger pointer; it builds the default handler and
  calls `slog.SetDefault(slog.New(handler))` directly. The `LevelVar` stays on `App`
  so `WithDebug` and the `PersistentPreRunE` debug toggle still work.
- `WithHandler` rebuilds the handler from the factory and calls
  `slog.SetDefault(...)` instead of assigning to `a.Logger`.
- Drop the `templateGet` helper — call `pkgtemplate.Get(root, cfg)` directly.

**`pkg/cmd/root.go`** (`pkg/cmd/root.go:33-66`)
- In `PersistentPreRunE`, when `--debug --output=json` switches the handler,
  call `slog.SetDefault(slog.New(slog.NewJSONHandler(cmd.ErrOrStderr(), ...)))`
  instead of mutating `app.Logger`. Drop the trailing standalone `slog.SetDefault`
  (it becomes redundant).

**`pkg/template/template.go`** (`pkg/template/template.go:65-145, 165-237`)
- `Get(templateRoot, cfg)` — drop the `logger *slog.Logger` parameter.
- Remove the `logger` field from `Template` struct.
- Replace every `logger.X(...)` / `t.logger.X(...)` with `slog.X(...)`.

**`pkg/template/context.go`** (`pkg/template/context.go:58-102`)
- `ApplyComputed(ctx, defs, funcMap, delims)` — drop the logger parameter.
- Use `slog.Debug(...)` directly.

**`pkg/template/functions.go`** (the sprout `WithLogger` call site)
- Pass `slog.Default()` to `sprout.WithLogger()` (sprout's API expects a `*slog.Logger`).

**`pkg/hooks/hooks.go`** (`pkg/hooks/hooks.go:18-115`)
- `Load(templateRoot, projectConfig, envPrefix)` — drop the logger parameter.
- Remove the `logger *slog.Logger` field from `Hooks`.
- Remove the `if logger == nil { logger = slog.Default() }` fallback in `Run`.
- All `logger.Debug(...)` → `slog.Debug(...)`.

**`pkg/cmd/template_use.go`** (`pkg/cmd/template_use.go:71-141, 200-340`)
- Remove `a.Logger` arguments from `pkgtemplate.Get`, `hooks.Load`,
  `pkgtemplate.ApplyComputed`, `promptContext`, `runPromptPass`.
- `promptContext` and `runPromptPass` lose their `logger *slog.Logger` parameters;
  use `slog.Debug(...)` directly.

**`pkg/cmd/template_list.go`**, **`template_update.go`**, **`template_download.go`**,
**`use.go`**
- `app.Logger.X(...)` → `slog.X(...)`.

**`pkg/registry/registry.go`** (`pkg/registry/registry.go:40-104`)
- Already uses `slog.Default().X(...)`. Replace with package-level `slog.X(...)`
  for consistency with the rest of the codebase.

### 2. Move git logging into `pkg/util/git` (review #3)

**`pkg/util/git/git.go`** — instrument the three core functions:

- `Clone` (`pkg/util/git/git.go:35`): `slog.Debug("git clone start", "repo", url, "dest", dir, "branch", opts.Branch)` and a success log on exit.
- `Describe` (`pkg/util/git/git.go:102`): `slog.Debug("git describe", "dest", dir, "commit", result.Commit, "version", result.Version)`; debug log on error.
- `CheckRemoteContext` (`pkg/util/git/git.go:290`): debug log on entry with `repo`, `branch`, `dest`; debug log on exit with `up_to_date`, `latest_version`, `error_kind`.

Once instrumented, **remove the duplicated logs** at the call sites:
- `pkg/cmd/template_download.go:53-63` (clone + describe)
- `pkg/cmd/use.go` (clone)
- `pkg/cmd/template_list.go:78-86` (`checking remote status` + `remote status result`)
- `pkg/cmd/template_update.go:50-59` (same)
- `pkg/registry/registry.go` — `CheckRemote`/`Clone`/`Describe` calls no longer need
  per-callsite duplication.

### 3. Drop the unused `error` return on `CheckRemote` (review #1)

`CheckRemoteContext` returns `(RemoteCheckResult, error)` but the implementation
returns `nil` for the error in **every** path — failures are encoded in
`result.ErrorKind` and exposed via `result.Err()`. Drop the `error` from the
signature so `result, _ := ...` patterns disappear and the contract matches reality.

- `pkg/util/git/git.go:290, 327`: change signatures to return only `RemoteCheckResult`.
- Update all callers: `pkg/cmd/app.go:59` (checkRemoteFn typedef),
  `template_list.go:78`, `template_update.go:50`, `registry.go:75`.

### 4. Add logging in `registry.Upgrade()` (review #4)

`pkg/registry/registry.go:65-104`:
- Log entry: `slog.Debug("upgrade start", "template", name)`.
- Log the resolved `targetRef` after `CheckRemote`: `slog.Debug("upgrade target", "template", name, "repo", meta.Repository, "branch", meta.Branch, "target_ref", targetRef, "latest_version", result.LatestVersion)`.
- Log the status file removal: `if err := os.Remove(...); err != nil && !os.IsNotExist(err) { slog.Debug("removing stale status file failed", "template", name, "error", err) }`.
- Log completion: `slog.Debug("upgrade complete", "template", name, "target_ref", targetRef)`.
- Clone/Describe logs come from the `pkg/util/git` instrumentation in §2 — no
  duplication needed here.

### 5. Stable attribute key conventions (review #2, #10)

Standardize on this final key set:

| Key        | Meaning                                                                                   |
|------------|-------------------------------------------------------------------------------------------|
| `template` | Registered template name (e.g. `"minimal"`) — primary user identifier                     |
| `path`     | Template-relative source path of a file (e.g. `"src/foo.go"`)                             |
| `dest`     | Absolute destination path on the filesystem                                               |
| `repo`     | Remote repository URL                                                                     |
| `branch`   | Git branch or tag ref                                                                     |
| `commit`   | Full git commit SHA                                                                       |
| `version`  | git-describe-style version string                                                         |
| `trigger`  | Hook trigger name (`pre-use`, `post-use`)                                                 |
| `key`      | Context variable name                                                                     |
| `source`   | How a context value was provided (`default`/`prompt`/`values_file`/`arg_flag`/`computed`) |
| `action`   | File decision (`render`/`verbatim`/`skip`)                                                |
| `error`    | Underlying error (formatted as `%v`)                                                      |

Replace mismatched uses:
- `pkg/template/template.go:92, 117, 130, 163` — `"root", templateRoot` → `"template", templateRoot`.
- `pkg/cmd/template_list.go:48, 53, 77, 80-86, 94, 101` — `"name"` → `"template"`.
- `pkg/cmd/template_update.go:43, 51, 53-58, 96` — `"name"` → `"template"`.
- `pkg/registry/registry.go:48, 53, 73` — `"name"` → `"template"`.

### 6. Prompt-source semantics — log final winner only (review #6)

Current behavior in `template_use.go`:
- Line 96 logs `source="values_file"` for every key in the values file.
- Line 108 logs `source="arg_flag"` for `--arg` keys.
- Line 119 logs `source="default"` for non-provided keys (when `--use-defaults`).
- Lines 254, 259 log `source="prompt"` when the user is prompted.
- `context.go:100` logs `source="computed"` after `ApplyComputed`.

Problem: a key can pass through `values_file` *and* `arg_flag` (override), or be
prompted after a `values_file` entry was discarded. Today both logs fire.

Fix: defer the resolution log until **after** the final value is established for
each key.

- Replace the per-step logs with a single "resolved sources" emission immediately
  before `ApplyComputed`. Build a `map[string]string` (key → final source) as
  values move through values_file → arg_flag → prompt → default, overwriting
  previous entries. Then emit one `slog.Debug("context key resolved", "key", k, "source", finalSource)` per key in alphabetical order.
- `ApplyComputed` continues to log `source="computed"` for its keys (those never
  overlap with user-input keys per `extractComputed`'s conflict check).

### 7. LoadStatus / metadata failure logging consistency (review #5)

Audit every `LoadMetadata`/`LoadStatus` caller and ensure every error path logs at
debug:
- `pkg/cmd/template_list.go:45, 50` ✓ already logs
- `pkg/cmd/template_update.go:43, 95` ✓ already logs
- `pkg/registry/registry.go:43, 50, 73` ✓ already logs
- `pkg/template/template.go:120` (LoadMetadata inside `Get`) ✓ already logs
- Verify no other callers exist via `Grep "LoadStatus\|LoadMetadata"`.

Also instrument the two currently-swallowed file mutations:
- `pkg/cmd/template_list.go:101`: `_ = pkgtemplate.SaveStatus(...)` → log on error.
- `pkg/cmd/template_update.go:64`: same.

### 8. Repo-wide audit of `, _ :=` and `_ = err` (review #9)

After the targeted fixes above, run a sweep:
- `Grep -n ", _ :=" pkg` and `Grep -n "_ = " pkg` (Go files).
- For each hit, decide: (a) intentional discard, leave alone; (b) diagnostic value,
  add `slog.Debug(...)`; (c) actual bug, propagate the error.
- Document any intentional discards with a one-line comment where the reason is non-obvious.

### 9. Tests (review #7, #8)

**TestMain pattern** — add `helpers_test.go` files (or extend existing ones) in
every package whose tests trigger logging code paths. Each contains:

```go
func TestMain(m *testing.M) {
    slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
    os.Exit(m.Run())
}
```

Packages: `pkg/template`, `pkg/hooks`, `pkg/cmd`, `pkg/registry`, `pkg/util/git`.

**Remove `discardLogger()`** from `pkg/template/helpers_test.go` and
`pkg/hooks/hooks_test.go` — no longer needed after the signature change. Update
all `Load(...)` / `Get(...)` callers in tests.

**New tests:**

- `pkg/template/metadata_test.go` — verify malformed `__metadata.json` produces a
  parse error from `LoadMetadata` (already partly tested; ensure the error path
  is asserted).
- `pkg/cmd/template_list_test.go` — table-driven test for the `template list`
  remote-check path: with a fake `checkRemoteFn` returning each `ErrorKind`,
  assert the row's status label and that logging attributes are emitted (capture
  via a per-test `slog.SetDefault` swap with `slog.NewTextHandler(&buf, ...)`).
- `pkg/cmd/root_test.go` — assert `--debug --output=json` keeps logs on stderr
  (JSON handler) while command data goes to stdout. Run a `template list` against
  a registry with one local template; parse stderr as NDJSON and stdout as JSON
  table; assert disjoint streams.
- `pkg/cmd/template_use_test.go` — verify the prompt-source dedup: pass
  `--values` plus `--arg` for the same key, assert the final log has
  `source="arg_flag"` not `source="values_file"`.
- `pkg/util/git` — light coverage for the new git-layer log entries via a
  capture handler; assert `repo`, `dest`, `branch` are present in `Clone` entry.

### 10. Docs update (review #10)

`docs/content/docs/architecture/overview.md` (the "Logging" section that PR #69
adds, around the end of the file):

- Replace the **Flags** description: the slog default is set in
  `NewApp()`/`PersistentPreRunE`; remove the obsolete "stored in `App.Logger`"
  sentence.
- Update the **Log points** table to match the actually-emitted attributes.
  Notably: `pkg/template` `Get` now uses `template` (not `root`); add
  `pkg/util/git` rows for `Clone`, `Describe`, `CheckRemoteContext`; add
  `pkg/registry` row for `Upgrade`.
- Replace the **Consistent attribute keys** table with the final set from §5.
- Note (under the example) that `slog.SetDefault` is called by `NewApp()` and
  re-set in `PersistentPreRunE` when `--debug --output=json` swaps the handler.

`README.md` — verify nothing user-facing changed. `--debug` and `--output=json`
flag semantics are unchanged; no update expected, but spot-check to confirm.

## Critical files (final list)

```
pkg/cmd/app.go
pkg/cmd/root.go
pkg/cmd/template_use.go
pkg/cmd/template_list.go
pkg/cmd/template_update.go
pkg/cmd/template_download.go
pkg/cmd/use.go
pkg/hooks/hooks.go
pkg/hooks/hooks_test.go
pkg/template/template.go
pkg/template/context.go
pkg/template/context_test.go
pkg/template/functions.go
pkg/template/metadata.go
pkg/template/helpers_test.go
pkg/registry/registry.go
pkg/util/git/git.go
docs/content/docs/architecture/overview.md
```

Plus new `helpers_test.go` / `TestMain` files in any package not already covered,
and new test files listed in §9.

## Reuse map

Nothing new needs inventing; everything is rewiring existing functions:

- `slog` package-level `Debug/Info/Warn/Error` (Go stdlib) replaces every
  injected-logger call.
- `slog.SetDefault` replaces `App.Logger` mutation.
- `RemoteCheckResult.Err()` (`pkg/util/git/git.go:269`) already exposes typed
  errors — keeps callers that want typed errors well-served after the signature
  change in §3.
- `output.Writer` (warnings/errors to humans) stays untouched — it serves a
  different purpose from `slog` (user-facing UX vs. diagnostic stream).

## Verification

1. **Compile & unit tests:** `task test` (per `.github/instructions/executing-commands.md`).
2. **Static analysis:** `task lint` or whatever is configured under
   `.github/workflows/` (run via the `ci-green-check` skill at the end).
3. **CLI smoke:**
   - `specs --debug template ls 2>&1 1>/dev/null | head -20` — should show
     structured debug lines with stable attribute keys; no `root=` should appear.
   - `specs --debug --output=json template ls 2>logs.ndjson 1>data.json` — verify
     `logs.ndjson` parses line-by-line as JSON with the expected keys; `data.json`
     is the table data only.
   - `specs --debug template download <repo> testtpl 2>&1 | grep "git clone"` —
     verify the git-layer log entry fires with `repo`, `dest`, `branch`.
   - `specs --debug template use testtpl ./out --arg Name=foo --values vals.json`
     where `vals.json` contains `{"Name":"bar"}` — verify only **one** log line
     for `key=Name` and it has `source="arg_flag"`.
   - `specs --debug template upgrade testtpl` — verify the new `upgrade *` log
     points fire.
4. **Test coverage for log-sensitive paths:** confirm the new tests in §9 all
   pass, including the capture-handler assertions.
5. **Docs check:** grep `docs/content/docs/architecture/overview.md` for any
   attribute key not in the §5 table; should be none.
