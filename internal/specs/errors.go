package specs

import "errors"

var (
	// ErrTemplateNotFound is returned when a name is given that has no matching directory.
	ErrTemplateNotFound = errors.New("template not found")

	// ErrTemplateAlreadyExists is returned when a template name is already in use and
	// the operation does not allow overwriting.
	ErrTemplateAlreadyExists = errors.New("template already exists")

	// ErrTemplateDirMissing is returned when the template root exists but has no template/ subdir.
	ErrTemplateDirMissing = errors.New("template directory is missing a 'template/' subdirectory")

	// ErrBothHookSources is returned when project.yaml contains inline hooks AND a hooks/
	// directory also exists. Only one source is allowed.
	ErrBothHookSources = errors.New("conflicting hook sources: found both inline hooks in project.yaml and a hooks/ directory")

	// ErrAmbiguousProjectFile is returned when both project.yaml and project.yml exist in the
	// template root. Only one is allowed.
	ErrAmbiguousProjectFile = errors.New("ambiguous project file: both project.yaml and project.yml exist — remove one")

	// ErrInvalidDelimiters is returned when __delimiters in project.yaml is present but
	// malformed or has empty left/right values.
	ErrInvalidDelimiters = errors.New(`"__delimiters" must be a mapping with non-empty "left" and "right" string values`)

	// ErrProjectFileMissing is returned when no project.yaml, project.yml, or project.json
	// is found in the template root.
	ErrProjectFileMissing = errors.New("no project file found")

	// ErrLocalSource is returned when a local path is given to a command that requires a
	// remote URL (e.g. specs template download).
	ErrLocalSource = errors.New("source is a local path — use 'specs template save' to register a local template")

	// ErrInvalidComputedDef is returned when a "computed" entry in project.yaml is malformed:
	// wrong type, value type mismatch, or conflict with a user input key.
	ErrInvalidComputedDef = errors.New("invalid computed definition")

	// ErrCyclicDependency is returned when a cycle is detected among computed or
	// referenced-default keys in project.yaml.
	ErrCyclicDependency = errors.New("cyclic dependency detected among keys")

	// ErrInvalidSpecsVersion is returned when the reserved "__specs__version" key in
	// project.yaml is present but is not a string or is not a parseable semver
	// constraint string.
	ErrInvalidSpecsVersion = errors.New(`"__specs__version" must be a valid semver constraint string`)

	// ErrSpecsVersionUnsatisfied is returned when the running CLI version does not
	// satisfy the "__specs__version" constraint declared by the template.
	ErrSpecsVersionUnsatisfied = errors.New("specs version constraint not satisfied")

	// ErrReservedVariableName is returned when a template defines a variable
	// (top-level key or computed value) whose name starts with the reserved "__"
	// prefix. That namespace is reserved for specs configuration keys such as
	// __delimiters.
	ErrReservedVariableName = errors.New(`variable names starting with "__" are reserved for specs configuration`)
)

// KindOf returns a stable, machine-readable string identifier for the sentinel
// wrapped in err, or "" when err wraps no known sentinel.
// The returned strings are safe to embed in JSON output as an "error_kind" field.
func KindOf(err error) string {
	switch {
	case errors.Is(err, ErrTemplateNotFound):
		return "template_not_found"
	case errors.Is(err, ErrTemplateAlreadyExists):
		return "template_already_exists"
	case errors.Is(err, ErrTemplateDirMissing):
		return "template_dir_missing"
	case errors.Is(err, ErrBothHookSources):
		return "both_hook_sources"
	case errors.Is(err, ErrAmbiguousProjectFile):
		return "ambiguous_project_file"
	case errors.Is(err, ErrInvalidDelimiters):
		return "invalid_delimiters"
	case errors.Is(err, ErrProjectFileMissing):
		return "project_file_missing"
	case errors.Is(err, ErrLocalSource):
		return "local_source"
	case errors.Is(err, ErrInvalidComputedDef):
		return "invalid_computed_def"
	case errors.Is(err, ErrCyclicDependency):
		return "cyclic_dependency"
	case errors.Is(err, ErrInvalidSpecsVersion):
		return "invalid_specs_version"
	case errors.Is(err, ErrSpecsVersionUnsatisfied):
		return "specs_version_unsatisfied"
	case errors.Is(err, ErrReservedVariableName):
		return "reserved_variable_name"
	default:
		return ""
	}
}
