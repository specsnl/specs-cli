package specs

import "errors"

var (
	// ErrTemplateNotFound is returned when a name is given that has no matching directory.
	ErrTemplateNotFound = errors.New("template not found")

	// ErrTemplateAlreadyExists is returned on save/download when the name is already in use
	// and --force was not passed.
	ErrTemplateAlreadyExists = errors.New("template already exists — use --force to overwrite")

	// ErrTemplateDirMissing is returned when the template root exists but has no template/ subdir.
	ErrTemplateDirMissing = errors.New("template directory is missing a 'template/' subdirectory")

	// ErrBothHookSources is returned when project.yaml contains inline hooks AND a hooks/
	// directory also exists. Only one source is allowed.
	ErrBothHookSources = errors.New("conflicting hook sources: found both inline hooks in project.yaml and a hooks/ directory")

	// ErrAmbiguousProjectFile is returned when both project.yaml and project.yml exist in the
	// template root. Only one is allowed.
	ErrAmbiguousProjectFile = errors.New("ambiguous project file: both project.yaml and project.yml exist — remove one")
)
