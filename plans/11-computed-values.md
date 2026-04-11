# Computed Values

## Problem

There is no way to define a value that is:
- Always derived from other inputs (never prompted)
- Available in template files under a clean name
- Composed of hardcoded strings, sprout functions, and references to user-provided inputs

The existing **referenced default** feature addresses a different need: a user input that
has a computed pre-fill but the user can still override it. Computed values are never shown
to the user at all — they are purely derived.

**Example use case:**

```yaml
# project.yaml
ProjectName: My Acme Project
ProjectShortName: acme-12

computed:
  ProjectSlug:    "[[toKebabCase .ProjectShortName]]"
  ProjectEnvName: "[[toUpperCase .ProjectShortName | replace \"-\" \"_\"]]"
  Year:           "[[now | date \"2006\"]]"
  DbName:         "[[toSnakeCase .ProjectShortName]]_production"
```

All four computed keys are available in template files as `[[.ProjectSlug]]`,
`[[.ProjectEnvName]]`, `[[.Year]]`, `[[.DbName]]` — exactly like user inputs.

---

## Decision

Add a top-level `computed:` section to `project.yaml`. Keys in `computed:` are:
- Evaluated **after** all user inputs are finalised (post-prompt)
- **Never** shown as prompts
- **Not** overridable via `--values` or `--arg`
- Merged into the context and available in template files and hook commands

---

## Syntax

```yaml
# User inputs (prompted)
ProjectName: My Acme Project
ProjectShortName: acme-12
UseSonarQube: false

# Computed — never prompted, always derived
computed:
  ProjectSlug:    "[[toKebabCase .ProjectShortName]]"
  ProjectEnvName: "[[toSnakeCase .ProjectShortName | toUpperCase]]"
  Year:           "[[now | date \"2006\"]]"
  DbName:         "[[toSnakeCase .ProjectShortName]]_production"
  # Can reference other computed values:
  DbTestName:     "[[.DbName | replace \"_production\" \"_test\"]]"

hooks:
  post-use:
    - composer install
```

Rules:
- Values are Go template expressions using `[[ ]]` delimiters and the full sprout FuncMap.
- Hardcoded strings without `[[` are valid (constant computed values).
- A computed key **must not** duplicate a user input key — error at load time.
- A computed key **cannot** be overridden by `--values` or `--arg` — error if attempted.

---

## Difference from Referenced Defaults

| | Referenced default | Computed value |
|---|---|---|
| Defined as | String value containing `[[` in the user input section | Key under `computed:` |
| User prompted? | Yes — shown with computed result as pre-fill | **No** |
| User can override? | Yes | **No** |
| Resolved when? | Before prompting (pre-fill calculation) | After all inputs are finalised |
| References other inputs? | Yes (pre-prompt defaults, not final values) | Yes (**final** post-prompt values) |
| Can reference other computed values? | No | Yes (topological sort) |

A referenced default is still the right tool when the user should be able to customise
the derived value. A computed value is the right tool when the value must always be
consistent — e.g. a database name derived from a slug that would break things if changed
independently.

---

## Resolution Order

```
1. Load project.yaml
2. Strip computed: and hooks: sections from the user-input map
3. Resolve referenced defaults (topological sort on [[ ]] in user input values)
4. Merge --values file overrides
5. Merge --arg flag overrides
6. Prompt user for remaining keys (huh form)
        ↓ final user context
7. Resolve computed values
   a. Identify dependency order (topological sort on .Key references in computed values)
   b. Error on cycles
   c. Execute each computed template against the current context in sorted order
   d. Add each result to the context before resolving dependents
        ↓ full context (user inputs + computed values)
8. Execute template files and hook commands
```

Step 7d is important: a computed value that references another computed value sees the
already-resolved result, not the raw template string.

---

## Dependency Resolution

Computed values may reference each other. The same topological sort algorithm used for
referenced defaults is applied:

1. Parse each computed value template and extract `.Key` references.
2. Build a dependency graph among computed keys (user-input keys are leaves — always resolved).
3. Apply Kahn's algorithm.
4. Error if a cycle is detected: name the keys involved.

**Example: valid dependency chain**

```yaml
computed:
  ProjectSlug: "[[toKebabCase .ProjectShortName]]"        # depends on user input
  DbName:      "[[.ProjectSlug]]_production"              # depends on ProjectSlug
  DbTestName:  "[[.DbName | replace \"_production\" \"_test\"]]"  # depends on DbName
```

Resolution order: `ProjectSlug` → `DbName` → `DbTestName`.

**Example: cycle (error)**

```yaml
computed:
  A: "[[.B]] suffix"
  B: "prefix [[.A]]"
  # A depends on B, B depends on A → cycle error
```

---

## Error Handling

| Situation | Behaviour |
|---|---|
| Computed key duplicates a user input key | Error at load time — names the conflicting key |
| `--values` or `--arg` targets a computed key | Error before prompting — names the key |
| Template syntax error in a computed value | Fatal error — names the key and the parse error |
| Reference to non-existent key | Fatal error — names the computed key and the missing reference |
| Cycle between computed values | Fatal error — lists the keys involved in the cycle |

All errors are fatal. Computed values are authored by the template creator, not the end
user, so silent fallbacks would mask bugs in the template.

---

## Template Availability

Computed values are merged into the context after resolution. In template files:

```
# Template file content using [[ ]] delimiters
DB_DATABASE=[[.DbName]]
DB_TEST_DATABASE=[[.DbTestName]]
APP_ENV_PREFIX=[[.ProjectEnvName]]
```

In hook commands (inline or `hooks/` scripts), computed values are available as environment
variables using the same uppercased naming convention as user inputs:

```yaml
hooks:
  post-use:
    - echo "Setting up [[.ProjectSlug]] (DB: [[.DbName]])"
```

---

## Validation

`specs template validate` should check computed values by:
1. Verifying no computed key conflicts with a user input key.
2. Verifying each computed template parses successfully.
3. Running a dry execute with all defaults applied to catch runtime reference errors.

---

## `project.yaml` Format (updated)

```yaml
# User inputs
ProjectName: My Acme Project
ProjectShortName: acme-12
ProjectDescription: A Laravel project scaffolded with specs.

PhpVersion:
  - "8.5"
  - "8.4"

UseSonarQube: false

# Referenced default — promptable, user can override
ProjectSlug: "[[toKebabCase .ProjectShortName]]"

# Computed — never prompted, always derived from final inputs
computed:
  ProjectEnvName: "[[toSnakeCase .ProjectShortName | toUpperCase]]"
  Year:           "[[now | date \"2006\"]]"
  DbName:         "[[.ProjectSlug]]_production"
  DbTestName:     "[[.DbName | replace \"_production\" \"_test\"]]"

hooks:
  post-use:
    - composer install
    - npm install
```

---

## Implementation

### `pkg/template/context.go` additions

```go
// computedSection extracts and removes the "computed" key from a raw context map.
// Returns the computed definitions as a separate map.
func extractComputed(raw map[string]any) (userCtx map[string]any, computedDefs map[string]string, err error)

// resolveComputed evaluates each computed template against the finalised user context.
// Returns the full merged context (user inputs + computed values).
func resolveComputed(ctx map[string]any, defs map[string]string) (map[string]any, error)
```

`resolveComputed` reuses the same topological sort implementation as `resolveReferencedDefaults`.

### Changes to `ParseContext`

`ParseContext` is split into two calls to fit the new resolution order:

```go
// LoadUserContext loads and returns user inputs + computed definitions separately.
// Referenced defaults are resolved. computed: and hooks: sections are stripped.
func LoadUserContext(templateRoot string) (userCtx map[string]any, computedDefs map[string]string, err error)

// ApplyComputed resolves computed values against the finalised context and merges them in.
// Called after prompting/overrides are complete.
func ApplyComputed(ctx map[string]any, defs map[string]string) (map[string]any, error)
```

### Changes to `Template` struct

```go
type Template struct {
    Root         string
    Context      map[string]any // user inputs (pre-prompt)
    ComputedDefs map[string]string      // raw computed definitions (resolved post-prompt)
    Metadata     *Metadata
    funcMap      template.FuncMap
    verbatim     *VerbatimRules
}
```

`Execute` calls `ApplyComputed` internally before the walk if `ComputedDefs` is non-empty.
