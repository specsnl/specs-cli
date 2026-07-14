package specs_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/specsnl/specs-cli/internal/specs"
)

func TestKindOf_KnownSentinels(t *testing.T) {
	cases := []struct {
		err      error
		wantKind string
	}{
		{specs.ErrTemplateNotFound, "template_not_found"},
		{fmt.Errorf("%w: mytemplate", specs.ErrTemplateNotFound), "template_not_found"},
		{specs.ErrTemplateAlreadyExists, "template_already_exists"},
		{fmt.Errorf("%w: foo — use --force to overwrite", specs.ErrTemplateAlreadyExists), "template_already_exists"},
		{specs.ErrTemplateDirMissing, "template_dir_missing"},
		{specs.ErrBothHookSources, "both_hook_sources"},
		{specs.ErrAmbiguousProjectFile, "ambiguous_project_file"},
		{specs.ErrInvalidDelimiters, "invalid_delimiters"},
		{specs.ErrProjectFileMissing, "project_file_missing"},
		{fmt.Errorf("%w in /tmp/tmpl", specs.ErrProjectFileMissing), "project_file_missing"},
		{specs.ErrLocalSource, "local_source"},
		{specs.ErrInvalidComputedDef, "invalid_computed_def"},
		{fmt.Errorf("%w: key %q conflicts with a user input key", specs.ErrInvalidComputedDef, "foo"), "invalid_computed_def"},
		{specs.ErrCyclicDependency, "cyclic_dependency"},
		{fmt.Errorf("%w: a, b", specs.ErrCyclicDependency), "cyclic_dependency"},
		{specs.ErrInvalidSpecsVersion, "invalid_specs_version"},
		{fmt.Errorf("%w: got int", specs.ErrInvalidSpecsVersion), "invalid_specs_version"},
		{specs.ErrSpecsVersionUnsatisfied, "specs_version_unsatisfied"},
		{fmt.Errorf("%w: template requires specs %s, but this binary is %s", specs.ErrSpecsVersionUnsatisfied, "^0.2.0", "0.1.0"), "specs_version_unsatisfied"},
		{specs.ErrReservedVariableName, "reserved_variable_name"},
		{fmt.Errorf("%w: __foo", specs.ErrReservedVariableName), "reserved_variable_name"},
	}

	for _, tc := range cases {
		got := specs.KindOf(tc.err)
		if got != tc.wantKind {
			t.Errorf("KindOf(%v) = %q, want %q", tc.err, got, tc.wantKind)
		}
	}
}

func TestKindOf_UnknownError(t *testing.T) {
	if got := specs.KindOf(errors.New("some unknown error")); got != "" {
		t.Errorf("KindOf(unknown) = %q, want %q", got, "")
	}
}

func TestKindOf_NilPanics(t *testing.T) {
	// KindOf(nil) should return "" without panicking — errors.Is handles nil gracefully.
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("KindOf(nil) panicked: %v", r)
		}
	}()
	if got := specs.KindOf(nil); got != "" {
		t.Errorf("KindOf(nil) = %q, want %q", got, "")
	}
}

func TestErrorsIs_AllSentinelsWrapCorrectly(t *testing.T) {
	sentinels := []error{
		specs.ErrTemplateNotFound,
		specs.ErrTemplateAlreadyExists,
		specs.ErrTemplateDirMissing,
		specs.ErrBothHookSources,
		specs.ErrAmbiguousProjectFile,
		specs.ErrInvalidDelimiters,
		specs.ErrProjectFileMissing,
		specs.ErrLocalSource,
		specs.ErrInvalidComputedDef,
		specs.ErrCyclicDependency,
		specs.ErrInvalidSpecsVersion,
		specs.ErrSpecsVersionUnsatisfied,
		specs.ErrReservedVariableName,
	}

	for _, sentinel := range sentinels {
		wrapped := fmt.Errorf("wrapping: %w", sentinel)
		if !errors.Is(wrapped, sentinel) {
			t.Errorf("errors.Is(%v, %v) = false, want true", wrapped, sentinel)
		}
	}
}
