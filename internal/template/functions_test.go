package template_test

import (
	"bytes"
	"testing"
	texttemplate "text/template"

	pkgtemplate "github.com/specsnl/specs-cli/internal/template"
)

// --- FuncMap presence ---

func TestFuncMap_ContainsCustomFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{})

	expected := []string{"hostname", "username", "toBinary", "formatFilesize", "password"}
	for _, name := range expected {
		if _, ok := m[name]; !ok {
			t.Errorf("custom function %q missing from FuncMap", name)
		}
	}
}

func TestFuncMap_ContainsSproutFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{})

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
		{"regex", "regexMatch"},
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

// --- Regex registry selection ---

// The regex and regexp registries expose identical function names, so presence
// alone cannot tell them apart. These are the four functions whose argument
// order differs between them: rendering them pins that FuncMap registers regex
// (subject last, pipeable) and not the deprecated regexp.
func TestFuncMap_RegistersPipeableRegexRegistry(t *testing.T) {
	cases := []struct {
		name string
		tmpl string
		want string
	}{
		{"regexFindAll", `{{ "abac" | regexFindAll "a." -1 }}`, "[ab ac]"},
		{"regexSplit", `{{ "a,b,c" | regexSplit "," -1 }}`, "[a b c]"},
		{"regexReplaceAll", `{{ "abca" | regexReplaceAll "a" "X" }}`, "XbcX"},
		{"regexReplaceAllLiteral", `{{ "abca" | regexReplaceAllLiteral "a" "$1" }}`, "$1bc$1"},
	}

	funcs := pkgtemplate.FuncMap(pkgtemplate.Config{})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := texttemplate.New(tc.name).Funcs(funcs).Parse(tc.tmpl)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			var buf bytes.Buffer
			if err := tpl.Execute(&buf, nil); err != nil {
				t.Fatalf("execute: %v", err)
			}

			if got := buf.String(); got != tc.want {
				t.Errorf("%s = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

// --- Safe mode ---

func TestFuncMap_SafeMode_ExcludesEnvFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: true})

	for _, name := range []string{"env", "expandEnv"} {
		if _, ok := m[name]; ok {
			t.Errorf("function %q should not be in FuncMap when SafeMode is true", name)
		}
	}
}

func TestFuncMap_SafeMode_ExcludesFilesystemFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: true})

	for _, name := range []string{"pathBase", "pathDir", "osBase", "osDir"} {
		if _, ok := m[name]; ok {
			t.Errorf("function %q should not be in FuncMap when SafeMode is true", name)
		}
	}
}

// --- Default mode ---

func TestFuncMap_DefaultMode_IncludesEnvFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: false})

	for _, name := range []string{"env", "expandEnv"} {
		if _, ok := m[name]; !ok {
			t.Errorf("function %q should be in FuncMap when SafeMode is false", name)
		}
	}
}

func TestFuncMap_DefaultMode_IncludesFilesystemFunctions(t *testing.T) {
	m := pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: false})

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

	pkgtemplate.FuncMap(pkgtemplate.Config{})
	pkgtemplate.FuncMap(pkgtemplate.Config{SafeMode: true})
}
