# Boilr — Supporting Packages Reference

## `pkg/boilr` — Global Configuration

**`configuration.go`**

| Symbol | Value | Description |
|--------|-------|-------------|
| `AppName` | `"boilr"` | Application name |
| `ConfigDirPath` | `".config/boilr"` | Relative to `$HOME` |
| `ConfigFileName` | `"config.json"` | Optional user overrides |
| `TemplateDir` | `"templates"` | Registry sub-directory name |
| `ContextFileName` | `"project.json"` | Variable schema file |
| `TemplateDirName` | `"template"` | Rendered files sub-directory |
| `TemplateMetadataName` | `"__metadata.json"` | Auto-generated metadata file |

```go
type Configuration struct {
    FilePath        string
    ConfigDirPath   string
    TemplateDirPath string
    IgnoreCopyFiles []string   // .DS_Store, Thumbs.db, …
}
```

`TemplatePath(name)` returns the absolute registry path for a given template name.
`IsTemplateDirInitialized()` checks whether the registry directory exists.
`init()` loads optional `~/.config/boilr/config.json` on startup.

**`errors.go`**

`ErrTemplateAlreadyExists` — sentinel returned when a name collision occurs without `--force`.

---

## `pkg/prompt` — Interactive Prompts

```go
type Interface interface {
    PromptMessage(fieldName string) string
    EvaluateChoice(input string) (interface{}, error)
}
```

| Type | Input | Output |
|------|-------|--------|
| `strPrompt` | Free text | `string` |
| `boolPrompt` | `y/yes/true` or `n/no/false` | `bool` |
| `multipleChoicePrompt` | Numeric index | chosen `string` |

**`prompt.Func(defaultValue)`** — returns the right prompt type for a given default.

**`prompt.New(fieldName, defaultValue)`** — returns a *closure* that:
- Shows the prompt once on first call.
- Caches the answer and returns it on subsequent calls.
  This ensures a variable used in multiple filenames/file contents is asked only once.

---

## `pkg/host` — GitHub URL Helpers

```go
// "tmrts/boilr-license" → "https://github.com/tmrts/boilr-license"
// "https://github.com/tmrts/boilr-license" → unchanged
func URL(repo string) string

// "tmrts/boilr-license:v1.0" → "https://codeload.github.com/tmrts/boilr-license/zip/v1.0"
func ZipURL(repo string) string
```

`ZipURL` supports an optional `:<branch-or-tag>` suffix for version-pinned downloads.

---

## `pkg/util/git`

Thin wrapper over `go-git`:

```go
type CloneOptions struct{ *ggit.CloneOptions }

func Clone(dir string, opts CloneOptions) error
```

---

## `pkg/util/osutil`

```go
func FileExists(path string) bool
func DirExists(path string) bool
func CreateDirs(paths ...string) error
func CopyRecursively(src, dst string) error
func GetUserHomeDir() string
```

---

## `pkg/util/exec`

```go
func Cmd(executable string, args ...string) (string, error)
```

Runs a subprocess, captures combined stdout/stderr.
Returns `(stdout, nil)` on success or `("", err)` on failure with stderr in the error message.

---

## `pkg/util/exit`

```go
const (
    CodeOK    = 0
    CodeError = 1
    CodeFatal = 2
)

func Fatal(err error)                      // exits 2
func Error(err error)                      // exits 1
func GoodEnough(format string, a ...any)   // exits 0, prints info
func OK(format string, a ...any)           // exits 0, prints success
```

---

## `pkg/util/tlog` — Terminal Logger

Log levels are bit-flags; `Set(level)` enables all levels up to and including that level.

| Level | Symbol | Colour |
|-------|--------|--------|
| `LevelDebug` | `☹` | Magenta |
| `LevelInfo` | `i` | Cyan |
| `LevelSuccess` | `✔` | Green |
| `LevelWarn` | `!` | Yellow |
| `LevelError` | `✘` | Red |
| `LevelFatal` | `✘` | Red |

```go
func Debug(msg string)
func Info(msg string)
func Success(msg string)
func Warn(msg string)
func Error(msg string)
func Fatal(msg string)
func Prompt(msg, defaultValue string)   // shows "?" prefix
```

Default log level is `LevelError` (only errors and fatals shown).

---

## `pkg/util/tabular`

```go
func Print(header []string, data [][]string)
```

Renders a coloured table using `olekukonko/tablewriter`.
Green separator lines, red name column, blue/yellow repository column.

---

## `pkg/util/validate`

### Patterns (`pattern.go`)

| Name | Regex intent |
|------|-------------|
| `Alpha` | Letters only |
| `Alphanumeric` | Letters + digits |
| `AlphanumericExt` | Letters + digits + `-` + `_` |
| `Integer` | Whole numbers |
| `Numeric` | Decimal numbers |
| `UnixPath` | File system paths |
| `URL` | HTTP(S) URLs |
| `Email` | Email addresses |

### String Validators (`string.go`)

```go
type String func(string) error   // validator function type

Integer()
URL()
UnixPath()
Alphanumeric()
AlphanumericExt()
```

### Argument Validator (`argument.go`)

```go
type Argument struct {
    Name      string
    Validator validate.String
}
```

Used by `MustValidateArgs` to pair each positional argument with a validator.

---

## `pkg/util/stringutil`

```go
type String interface {
    io.ReadWriter
    String() string
}

func NewString(contents string) String
```

A `bytes.Buffer`-backed `io.ReadWriter` used internally when rendering template content
into a string before writing to disk.
