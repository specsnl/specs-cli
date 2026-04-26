package template

import (
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
	"unicode/utf8"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// Config holds options that control template loading and execution behaviour.
type Config struct {
	// SafeMode disables the env and filesystem sprout registries and must be
	// combined with skipping hooks at the command layer. Use this when executing
	// templates from untrusted sources.
	SafeMode bool
}

// ignoredFiles are always skipped — they are OS/editor metadata, not template content.
var ignoredFiles = map[string]bool{
	".DS_Store": true,
	"Thumbs.db": true,
}

// Template holds everything needed to execute a boilr template.
type Template struct {
	Root         string                 // path to the template root (contains project.yaml + template/)
	Context      map[string]any         // user input map with referenced defaults resolved
	ComputedDefs map[string]string      // raw computed definitions; resolved by ApplyComputed post-prompt
	Conditionals Conditionals           // varName → Cond; absent means always prompt
	Referenced   map[string]bool        // schema variables referenced in template files or computed expressions
	Metadata     *Metadata              // nil if __metadata.json is absent
	cfg          Config
	logger       *slog.Logger
	funcMap      texttemplate.FuncMap
	verbatim     *VerbatimRules
}

// FuncMap returns the template's function map. Used by callers that need to pass
// the same FuncMap to hooks or ApplyComputed.
func (t *Template) FuncMap() texttemplate.FuncMap {
	return t.funcMap
}

// Get loads a template from templateRoot. The root must contain either project.yaml or
// project.json, and a template/ subdirectory.
func Get(templateRoot string, cfg Config, logger *slog.Logger) (*Template, error) {
	funcMap := FuncMap(cfg, logger)

	userCtx, computedDefs, err := LoadUserContext(templateRoot, funcMap)
	if err != nil {
		return nil, err
	}

	conds, referenced, err := AnalyzeConditionals(templateRoot, userCtx, funcMap)
	if err != nil {
		return nil, err
	}

	// Also count variables that only appear in computed expressions as referenced.
	for _, expr := range computedDefs {
		for _, key := range extractRefs(expr, funcMap) {
			if _, inSchema := userCtx[key]; inSchema {
				referenced[key] = true
			}
		}
	}

	verbatim, err := LoadVerbatim(templateRoot)
	if err != nil {
		return nil, err
	}

	meta, _ := loadMetadata(templateRoot) // missing metadata is not an error

	return &Template{
		Root:         templateRoot,
		Context:      userCtx,
		ComputedDefs: computedDefs,
		Conditionals: conds,
		Referenced:   referenced,
		Metadata:     meta,
		cfg:          cfg,
		logger:       logger,
		funcMap:      funcMap,
		verbatim:     verbatim,
	}, nil
}

// Execute renders the template/ subdirectory into targetDir, which must already exist.
// If ComputedDefs is non-empty, computed values are resolved and merged into the context
// before the walk begins.
func (t *Template) Execute(targetDir string) error {
	ctx := t.Context
	if len(t.ComputedDefs) > 0 {
		var err error
		ctx, err = ApplyComputed(t.Context, t.ComputedDefs, t.funcMap)
		if err != nil {
			return err
		}
	}

	srcRoot := filepath.Join(t.Root, specs.TemplateDirFile)

	return filepath.WalkDir(srcRoot, func(srcPath string, d fs.DirEntry, err error) error {
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
			t.logger.Debug("skipping path", "path", rel, "error", err)
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 3. Skip if any path segment is empty (conditional directory exclusion).
		if hasEmptySegment(destRel) {
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
			return copyFile(srcPath, destPath)
		}
		return t.renderFile(srcPath, destPath, ctx)
	})
}

// renderName renders a file/directory path template using [[ ]] delimiters.
func (t *Template) renderName(name string, ctx map[string]any) (string, error) {
	tmpl, err := texttemplate.New("").Delims("[[", "]]").Funcs(t.funcMap).Parse(name)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// renderFile renders a text file's content using [[ ]]  delimiters.
// If the rendered content is whitespace-only, the destination file is not created.
func (t *Template) renderFile(srcPath, destPath string, ctx map[string]any) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	tmpl, err := texttemplate.New("").
		Delims("[[", "]]").
		Funcs(t.funcMap).
		Option("missingkey=error").
		Parse(string(data))
	if err != nil {
		t.logger.Debug("template parse error, copying verbatim", "path", srcPath, "error", err)
		return copyFile(srcPath, destPath)
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		t.logger.Debug("template execute error, copying verbatim", "path", srcPath, "error", err)
		return copyFile(srcPath, destPath)
	}

	result := buf.String()
	if strings.TrimSpace(result) == "" {
		return nil // whitespace-only: skip
	}

	return writeFile(destPath, []byte(result))
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

	for _, b := range buf {
		if b == 0 {
			return true
		}
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
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
