package template

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/specsnl/specs-cli/pkg/specs"
)

// Sentinel errors identifying each kind of validation issue.
// Use errors.Is to test a ValidationIssue against these values.
var (
	ErrUnknownVariable = errors.New("unknown_variable")
	ErrUnusedVariable  = errors.New("unused_variable")
	ErrUnusedComputed  = errors.New("unused_computed")
)

// ValidationIssue is a single finding from Validate.
type ValidationIssue struct {
	Kind error  // one of ErrUnknownVariable, ErrUnusedVariable, ErrUnusedComputed
	Name string // variable or computed value name
	File string // non-empty for unknown_variable: path relative to template root
}

func (v ValidationIssue) Error() string {
	if v.File != "" {
		return fmt.Sprintf("%s: %q in %s", v.Kind, v.Name, v.File)
	}
	return fmt.Sprintf("%s: %q", v.Kind, v.Name)
}

// Unwrap lets errors.Is match against the kind sentinel.
func (v ValidationIssue) Unwrap() error { return v.Kind }

// HasUnknown reports whether any issue is an unknown-variable reference.
func HasUnknown(issues []ValidationIssue) bool {
	for _, iss := range issues {
		if errors.Is(iss, ErrUnknownVariable) {
			return true
		}
	}
	return false
}

// HasUnused reports whether any issue is an unused variable or computed value.
func HasUnused(issues []ValidationIssue) bool {
	for _, iss := range issues {
		if errors.Is(iss, ErrUnusedVariable) || errors.Is(iss, ErrUnusedComputed) {
			return true
		}
	}
	return false
}

// Validate inspects the template for two categories of issues:
//   - unknown_variable: a name used in a template file or path that is not
//     defined in project.yaml (neither as a variable nor as a computed value).
//   - unused_variable / unused_computed: a name defined in project.yaml that
//     is never referenced in any template file, path expression, or computed
//     expression.
func (t *Template) Validate() ([]ValidationIssue, error) {
	allDefined := make(map[string]bool, len(t.Context)+len(t.ComputedDefs))
	for k := range t.Context {
		allDefined[k] = true
	}
	for k := range t.ComputedDefs {
		allDefined[k] = true
	}

	type issueKey struct {
		kind       error
		name, file string
	}
	seen := make(map[issueKey]bool)
	var issues []ValidationIssue

	addIssue := func(kind error, name, file string) {
		k := issueKey{kind, name, file}
		if !seen[k] {
			seen[k] = true
			issues = append(issues, ValidationIssue{Kind: kind, Name: name, File: file})
		}
	}

	// Scan template files and path expressions for unknown variable references.
	srcRoot := filepath.Join(t.Root, specs.TemplateDirFile)
	err := filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relFromRoot, _ := filepath.Rel(t.Root, path)
		if relFromRoot == "." {
			return nil
		}
		reportPath := filepath.ToSlash(relFromRoot)

		// Check the entry's name as a path template expression.
		for _, k := range extractRefs(d.Name(), t.funcMap) {
			if !allDefined[k] {
				addIssue(ErrUnknownVariable, k, reportPath)
			}
		}

		if d.IsDir() {
			return nil
		}

		// Check file content.
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, k := range extractRefs(string(data), t.funcMap) {
			if !allDefined[k] {
				addIssue(ErrUnknownVariable, k, reportPath)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Unused variables: defined in project.yaml but not referenced anywhere.
	// Template.Referenced already includes variables used in computed expressions.
	for k := range t.Context {
		if !t.Referenced[k] {
			addIssue(ErrUnusedVariable, k, "")
		}
	}

	// Unused computed: defined under "computed" but not referenced anywhere.
	for k := range t.ComputedDefs {
		if !t.Referenced[k] {
			addIssue(ErrUnusedComputed, k, "")
		}
	}

	kindRank := map[error]int{ErrUnknownVariable: 0, ErrUnusedVariable: 1, ErrUnusedComputed: 2}
	sort.Slice(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		if kindRank[a.Kind] != kindRank[b.Kind] {
			return kindRank[a.Kind] < kindRank[b.Kind]
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.File < b.File
	})

	return issues, nil
}
