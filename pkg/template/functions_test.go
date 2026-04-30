package template_test

import (
	"testing"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

// --- FuncMap presence ---

func TestFuncMap_ContainsCustomFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger())

	expected := []string{"hostname", "username", "toBinary", "formatFilesize", "password"}
	for _, name := range expected {
		if _, ok := m[name]; !ok {
			t.Errorf("custom function %q missing from FuncMap", name)
		}
	}
}

func TestFuncMap_ContainsSproutFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger())

	// Spot-check one function from each sprout registry group.
	spotCheck := []struct {
		registry string
		fn       string
	}{
		{"strings", "toKebabCase"},
		{"strings", "toSnakeCase"},
		{"checksum", "sha256Sum"},
		{"time", "now"},
		{"semver", "semver"},
		{"uniqueid", "uuidv4"},
		{"network", "parseIP"},
		{"crypto", "bcrypt"},
		{"reflect", "typeOf"},
		{"encoding", "base64Encode"},
		{"regexp", "regexMatch"},
		{"slices", "list"},
		{"maps", "dict"},
		{"numeric", "add"},
		{"conversion", "toInt"},
		{"random", "randAlpha"},
	}

	for _, tc := range spotCheck {
		if _, ok := m[tc.fn]; !ok {
			t.Errorf("sprout function %q (from %s registry) missing from FuncMap", tc.fn, tc.registry)
		}
	}
}

// --- Safe mode ---

func TestFuncMap_SafeMode_ExcludesEnvFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: true}, discardLogger())

	for _, name := range []string{"env", "expandEnv"} {
		if _, ok := m[name]; ok {
			t.Errorf("function %q should not be in FuncMap when SafeMode is true", name)
		}
	}
}

func TestFuncMap_SafeMode_ExcludesFilesystemFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: true}, discardLogger())

	for _, name := range []string{"pathBase", "pathDir", "osBase", "osDir"} {
		if _, ok := m[name]; ok {
			t.Errorf("function %q should not be in FuncMap when SafeMode is true", name)
		}
	}
}

// --- Default mode ---

func TestFuncMap_DefaultMode_IncludesEnvFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: false}, discardLogger())

	for _, name := range []string{"env", "expandEnv"} {
		if _, ok := m[name]; !ok {
			t.Errorf("function %q should be in FuncMap when SafeMode is false", name)
		}
	}
}

func TestFuncMap_DefaultMode_IncludesFilesystemFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: false}, discardLogger())

	for _, name := range []string{"pathBase", "pathDir", "osBase", "osDir"} {
		if _, ok := m[name]; !ok {
			t.Errorf("function %q should be in FuncMap when SafeMode is false", name)
		}
	}
}

// --- No panic ---

func TestFuncMap_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("FuncMap() panicked: %v", r)
		}
	}()
	pkgtemplate.FuncMap(pkgtemplate.Config{}, discardLogger())
	pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: true}, discardLogger())
}
