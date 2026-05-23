package template

import (
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	texttemplate "text/template"
	"unicode/utf8"

	"github.com/specsnl/specs-cli/internal/specs"
)

// Config holds options that control template loading and execution behaviour.
type Config struct {
	// SafeMode disables the env and filesystem sprout registries and must be
	// combined with skipping hooks at the command layer. Use this when executing
	// templates from untrusted sources.
	SafeMode bool

	// Delims overrides the default [[ ]] template delimiters. The zero value
	// resolves to specs.DefaultDelimiters. Set this field to support per-project
	// custom delimiters read from project.yaml.
	Delims specs.Delimiters

	// ContinueOnRenderError switches from the default fail-fast behaviour to the
	// legacy warn-and-copy mode: parse or execution errors are recorded as
	// RenderWarnings and the file is copied verbatim instead of aborting Execute.
	// Enable this via --continue-on-error when the template has intentionally
	// unrenderable files that cannot use .specsverbatim.
	ContinueOnRenderError bool
}

// delims returns the configured delimiters, falling back to specs.DefaultDelimiters
// when the zero value was left unset.
func (c Config) delims() specs.Delimiters {
	if c.Delims.Left == "" {
		return specs.DefaultDelimiters
	}
	return c.Delims
}

// ignoredFiles are always skipped — they are OS/editor metadata, not template content.
var ignoredFiles = map[string]bool{
	".DS_Store": true,
	"Thumbs.db": true,
}

// RenderWarning records a non-fatal issue encountered while rendering a template file.
// The file is still written to the destination as a verbatim copy.
// Only produced when Config.ContinueOnRenderError is true.
type RenderWarning struct {
	Path    string // template-relative path of the affected file
	Err     error
	Preview string // first ~80 bytes of the unrendered source content; helps grep for stragglers
}

// Template holds everything needed to execute a boilr template.
type Template struct {
	Root         string                 // path to the template root (contains project.yaml + template/)
	Context      map[string]any         // user input map with referenced defaults resolved
	ComputedDefs map[string]string      // raw computed definitions; caller must apply via ApplyComputed before Execute
	Conditionals Conditionals           // varName → Cond; absent means always prompt
	Referenced   map[string]bool        // schema variables referenced in template files or computed expressions
	Metadata     *Metadata              // nil if __metadata.json is absent
	Warnings     []RenderWarning        // non-fatal render issues collected during Execute
	cfg          Config
	funcMap      texttemplate.FuncMap
	verbatim     *VerbatimRules
}

// FuncMap returns the template's function map. Used by callers that need to pass
// the same FuncMap to hooks or ApplyComputed.
func (t *Template) FuncMap() texttemplate.FuncMap {
	return t.funcMap
}

// Delims returns the delimiter pair in use for this template. Callers such as
// hooks.Run need this to render [[ ]]-style expressions with the same delimiters.
func (t *Template) Delims() specs.Delimiters {
	return t.cfg.delims()
}

// Get loads a template from templateRoot. The root must contain either project.yaml or
// project.json, and a template/ subdirectory.
func Get(templateRoot string, cfg Config) (*Template, error) {
	slog.Debug("loading template", "template", templateRoot)

	funcMap := FuncMap(cfg)

	// Resolve delimiters: __delimiters in project.yaml > cfg.Delims > DefaultDelimiters.
	// Write the resolved value back into cfg so methods on the returned Template
	// (renderName, renderFile, Delims, …) all see the same delimiters.
	delims, err := ExtractProjectDelimiters(templateRoot, cfg.delims())
	if err != nil {
		return nil, fmt.Errorf("reading delimiters from project file: %w", err)
	}
	cfg.Delims = delims

	userCtx, computedDefs, err := LoadUserContext(templateRoot, funcMap, delims)
	if err != nil {
		return nil, err
	}

	conds, referenced, err := AnalyzeConditionals(templateRoot, userCtx, funcMap, delims)
	if err != nil {
		return nil, err
	}

	// Also count variables that only appear in computed expressions as referenced.
	for _, expr := range computedDefs {
		for _, key := range extractRefs(expr, funcMap, delims) {
			if _, inSchema := userCtx[key]; inSchema {
				referenced[key] = true
			}
		}
	}

	verbatim, err := LoadVerbatim(templateRoot)
	if err != nil {
		return nil, err
	}

	meta, err := LoadMetadata(templateRoot) // missing metadata is not an error
	if err != nil {
		slog.Debug("failed to parse template metadata", "template", templateRoot, "error", err)
	}

	slog.Debug("template loaded", "template", templateRoot, "keys", len(userCtx), "computed", len(computedDefs))

	return &Template{
		Root:         templateRoot,
		Context:      userCtx,
		ComputedDefs: computedDefs,
		Conditionals: conds,
		Referenced:   referenced,
		Metadata:     meta,
		cfg:          cfg,
		funcMap:      funcMap,
		verbatim:     verbatim,
	}, nil
}

// Execute renders the template/ subdirectory into targetDir, which must already exist.
// Context must already contain all resolved values (user inputs and computed); call
// ApplyComputed before Execute when ComputedDefs are present.
func (t *Template) Execute(targetDir string) error {
	ctx := t.Context

	srcRoot := filepath.Join(t.Root, specs.TemplateDirFile)

	slog.Debug("starting template execution", "template", t.Root, "dest", targetDir)

	var rendered, skipped, verbatim int

	walkErr := filepath.WalkDir(srcRoot, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(srcRoot, srcPath)
		if rel == "." {
			return nil // skip the root itself
		}

		// 1. Skip OS/editor metadata files.
		if ignoredFiles[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 2. Render the relative path as a template to get the destination path.
		destRel, err := t.renderName(rel, ctx)
		if err != nil || strings.TrimSpace(destRel) == "" {
			slog.Debug("skipping path", "path", rel, "error", err)
			if !d.IsDir() {
				skipped++
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 3. Skip if any path segment is empty (conditional directory exclusion).
		if hasEmptySegment(destRel) {
			if !d.IsDir() {
				skipped++
			}
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		destPath := filepath.Join(targetDir, destRel)

		// 4. Directory: create it.
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}

		// 5. File: determine copy strategy.
		relForward := filepath.ToSlash(rel)
		if t.verbatim.Matches(relForward) || isBinary(srcPath) {
			slog.Debug("file decision", "path", rel, "dest", destPath, "action", "verbatim")
			verbatim++
			return copyFile(srcPath, destPath)
		}
		slog.Debug("file decision", "path", rel, "dest", destPath, "action", "render")
		rendered++
		return t.renderFile(srcPath, destPath, relForward, ctx)
	})

	if walkErr != nil {
		return walkErr
	}

	slog.Info("template execution complete",
		"template", t.Root,
		"dest", targetDir,
		"rendered", rendered,
		"verbatim", verbatim,
		"skipped", skipped,
	)
	return nil
}

// renderName renders a file/directory path template using the configured delimiters.
func (t *Template) renderName(name string, ctx map[string]any) (string, error) {
	d := t.cfg.delims()
	tmpl, err := texttemplate.New("").Delims(d.Left, d.Right).Funcs(t.funcMap).Parse(name)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderFile renders a text file's content using the configured delimiters.
// If the rendered content is whitespace-only, the destination file is not created.
// By default a parse or execution error aborts Execute with a wrapped error.
// When Config.ContinueOnRenderError is true the error is recorded as a RenderWarning
// and the file is copied verbatim instead.
func (t *Template) renderFile(srcPath, destPath, rel string, ctx map[string]any) error {
	info, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	d := t.cfg.delims()
	tmpl, err := texttemplate.New("").
		Delims(d.Left, d.Right).
		Funcs(t.funcMap).
		Option("missingkey=error").
		Parse(string(data))
	if err != nil {
		if t.cfg.ContinueOnRenderError {
			t.Warnings = append(t.Warnings, RenderWarning{Path: rel, Err: err, Preview: contentPreview(data)})
			return copyFile(srcPath, destPath)
		}
		return fmt.Errorf("parsing template %s: %w", rel, err)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		if t.cfg.ContinueOnRenderError {
			t.Warnings = append(t.Warnings, RenderWarning{Path: rel, Err: err, Preview: contentPreview(data)})
			return copyFile(srcPath, destPath)
		}
		return fmt.Errorf("rendering template %s: %w", rel, err)
	}

	result := buf.String()
	if strings.TrimSpace(result) == "" {
		return nil // whitespace-only: skip
	}

	return writeFile(destPath, []byte(result), info.Mode())
}

// contentPreview returns the first ~80 characters of data for display in warnings.
func contentPreview(data []byte) string {
	const max = 80
	s := string(data)
	if len(s) > max {
		return s[:max]
	}
	return s
}

// isBinary returns true if the file contains a null byte or invalid UTF-8.
// Only the first 512 bytes are examined for performance.
func isBinary(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	buf = buf[:n]

	if slices.Contains(buf, 0) {
		return true
	}
	return !utf8.Valid(buf)
}

// hasEmptySegment returns true if any path segment is empty or whitespace-only.
func hasEmptySegment(path string) bool {
	for seg := range strings.SplitSeq(path, string(filepath.Separator)) {
		if strings.TrimSpace(seg) == "" {
			return true
		}
	}
	return false
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return os.Chmod(dst, info.Mode())
}

func writeFile(path string, data []byte, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}
