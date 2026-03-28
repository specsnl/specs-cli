# Boilr — Issues & PRs Review

Covering open (and select closed) issues from both
[Ilyes512/boilr](https://github.com/Ilyes512/boilr) and the upstream
[tmrts/boilr](https://github.com/tmrts/boilr). Dependabot / version-bump items excluded.

---

## Already addressed in v2 plans

| Issue | Title | Addressed by |
|-------|-------|-------------|
| tmrts#73 | Escaping `{{ }}` not intended as template expressions | `[[ ]]` delimiter change (plan 06) |
| tmrts#56 | Conditional creation of directories | Empty filename → skip (plan 06 / WIP branch) |
| tmrts#30 | File existence control | Empty filename → skip (plan 06 / WIP branch) |
| tmrts#68 | Panic on templating images [JPG] | Binary file fallback (WIP branch) |
| tmrts#69 | Panic on templating TeX files | Binary file fallback (WIP branch) |
| Ilyes512#56 | favicon.ico is not created | Binary file fallback (WIP branch) |
| tmrts#37 | Add Sprig template library | Already done in Ilyes512/boilr |

---

## v2 scope — decided

### Easy fixes (implement immediately)

| Issue | Title | Fix |
|-------|-------|-----|
| tmrts#79, tmrts#29 | `--use-defaults` closure bug — all variables get the same value | Fix variable capture in `handleBindDefaults` loop |
| tmrts#61 | Tag names reject `-` and `_` | Change tag validator from `Alphanumeric` to `AlphanumericExt` (already exists) |
| tmrts#67 | Invalid `project.json` not reported | Surface JSON decode error before template execution; don't swallow it |
| tmrts#27 | Error messages not helpful | Print command usage automatically on argument errors (`SilenceUsage: false` in Cobra) |

### XDG Base directory (Ilyes512#33)

Use `github.com/adrg/xdg` to resolve config and data directories.

```go
// v2 — respects $XDG_CONFIG_HOME, falls back to ~/.config/boilr
configDir, _ := xdg.ConfigFile("boilr")
```

Zero behaviour change for most users. Correct behaviour for users who set `$XDG_CONFIG_HOME`.

### `--values` flag (Ilyes512#48, tmrts#33)

Add a `--values <file.json>` flag to `boilr template use`.

Behaviour:
- Loads a JSON file of key/value pairs.
- Any key present in the file skips its prompt and uses the provided value.
- Keys absent from the file are still prompted interactively.
- `--use-defaults` and `--values` are mutually exclusive.

```bash
# Skip all prompts — use JSON file for everything
boilr template use laravel ~/projects/acme --values acme.json

# Pre-fill some keys, prompt for the rest
boilr template use laravel ~/projects/acme --values partial.json

# CI: pass individual values inline
boilr template use laravel ~/projects/acme --arg ProjectName=Acme --arg PhpVersion=8.4
```

Both `--values` (file) and `--arg Key=Value` (repeatable flag) are supported.
`--arg` values take precedence over `--values` file values.

### Referenced defaults in `project.json` (tmrts#6, Ilyes512#8)

Default values in `project.json` can reference other variables using `[[ ]]` template syntax.
Boilr resolves them in dependency order before prompting.

```json
{
    "ProjectName": "My Acme Project",
    "ProjectSlug": "[[kebabcase .ProjectName]]",
    "ProjectDescription": "A project called [[.ProjectName]]."
}
```

When the user answers `ProjectName`, the resolved value is used to compute the default for
`ProjectSlug` and `ProjectDescription` before those prompts appear.

Resolution uses a simple topological sort — circular references are a validation error.

### Hooks (tmrts#7, Ilyes512#7)

Hooks run shell commands before and after `boilr template use`. The two trigger points are:

| Hook | Working directory | Runs |
|------|------------------|-------|
| `pre-use` | template source directory | Before any files are rendered. Return non-zero to abort. |
| `post-use` | target (output) directory | After all files are written. Receives resolved context as env vars. |

#### Definition — two mutually exclusive forms

**Form A: inline in `project.yaml`**

Each hook is a list of commands executed sequentially. Any item can be a single command
string or a multiline bash script using YAML's block scalar (`|`).

```yaml
hooks:
  pre-use:
    - echo "Scaffolding [[.ProjectName]]..."

  post-use:
    - composer install
    - npm install
    - |
      git init
      git add -A
      git commit -m "Initial commit: [[.ProjectName]]"
    - echo "Done!"
```

- Each list item is passed to `bash -c` individually.
- Commands run sequentially; a non-zero exit code aborts the remaining steps.
- `[[ ]]` template expressions in hook commands are resolved before execution.

**Form B: script files in the template root**

```
template-root/
├── project.json
├── hooks/
│   ├── pre-use.sh
│   └── post-use.sh
└── template/
```

Both forms cannot be present at the same time — boilr errors if it finds a `hooks` key in
`project.json` AND a `hooks/` directory. Template authors choose one form.

#### Context as environment variables

Resolved context values are injected as environment variables so scripts can use them:

```bash
# post-use.sh
git init
git add -A
git commit -m "Initial commit: $ProjectName"
composer install
```

Variable names match the `project.json` keys exactly, uppercased:
`ProjectName` → `PROJECTNAME`, `PhpVersion` → `PHPVERSION`.

#### Skipping hooks

```bash
boilr template use laravel ~/projects/acme --no-hooks
```

### Simplified one-step command (tmrts#39)

Add `boilr use` as a shorthand that downloads to a temp directory, executes, and discards.
The existing `template download/save/use` lifecycle is retained for users who want a local
registry.

```bash
# One-shot — no local registry entry created
boilr use github:Ilyes512/boilr-laravel-project ~/projects/acme

# Equivalent long form (unchanged)
boilr template download Ilyes512/boilr-laravel-project laravel
boilr template use laravel ~/projects/acme
```

---

## Deferred

| Issue | Reason |
|-------|--------|
| tmrts#77 — Multiple templates per repo / branch | `user/repo:branch` syntax extension — can be added later without touching the core |
| Ilyes512#28 / tmrts#11 — Private repo auth | SSH/token plumbing in go-git; scope-heavy, post-v2 |
| tmrts#8 / Ilyes512#9 — TOML support | Superseded: `project.json` replaced by `project.yaml` in v2 |
| tmrts#34 — Input validation in prompts | huh has `Validate` on fields; easy to add after prompt layer is rewritten |
| tmrts#75 — Symlink support | Edge case; `filepath.Walk` skips symlinks by design |
| tmrts#4 — Update projects when templates change | Fundamentally hard (diffing rendered output) |
