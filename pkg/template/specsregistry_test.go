package template_test

import (
	"testing"
	texttemplate "text/template"

	pkgtemplate "github.com/specsnl/specs-cli/pkg/template"
)

func newRegistry() *pkgtemplate.SpecsRegistry {
	return pkgtemplate.NewSpecsRegistry()
}

// --- Registry interface ---

func TestSpecsRegistry_UID(t *testing.T) {
	if got := newRegistry().UID(); got != "specsnl/specs-cli.specs" {
		t.Errorf("UID() = %q, want %q", got, "specsnl/specs-cli.specs")
	}
}

func TestSpecsRegistry_LinkHandler_ReturnsNoError(t *testing.T) {
	if err := newRegistry().LinkHandler(nil); err != nil {
		t.Errorf("LinkHandler(nil) returned error: %v", err)
	}
}

func TestSpecsRegistry_RegisterFunctions_RegistersAllNames(t *testing.T) {
	m := make(texttemplate.FuncMap)
	if err := newRegistry().RegisterFunctions(m); err != nil {
		t.Fatalf("RegisterFunctions returned error: %v", err)
	}

	expected := []string{"hostname", "username", "toBinary", "formatFilesize", "password"}
	for _, name := range expected {
		if _, ok := m[name]; !ok {
			t.Errorf("function %q not registered", name)
		}
	}
}

// --- Hostname ---

func TestSpecsRegistry_Hostname_NonEmpty(t *testing.T) {
	h := newRegistry().Hostname()
	if h == "" {
		t.Error("Hostname() returned empty string")
	}
}

// --- Username ---

func TestSpecsRegistry_Username_NonEmpty(t *testing.T) {
	u := newRegistry().Username()
	if u == "" {
		t.Error("Username() returned empty string")
	}
}

// --- ToBinary ---

func TestSpecsRegistry_ToBinary(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{5, "101"},
		{10, "1010"},
		{255, "11111111"},
		{-1, "-1"},
	}
	r := newRegistry()
	for _, tt := range tests {
		if got := r.ToBinary(tt.input); got != tt.want {
			t.Errorf("ToBinary(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- FormatFilesize ---

func TestSpecsRegistry_FormatFilesize(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "0B"},
		{1000, "1kB"},
		{1000 * 1000, "1MB"},
		{1000 * 1000 * 1000, "1GB"},
	}
	r := newRegistry()
	for _, tt := range tests {
		if got := r.FormatFilesize(tt.input); got != tt.want {
			t.Errorf("FormatFilesize(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Password ---

func TestSpecsRegistry_Password_CorrectLength(t *testing.T) {
	r := newRegistry()
	for _, length := range []int{8, 12, 20} {
		p := r.Password(length, 2, 2, false, false)
		if len(p) != length {
			t.Errorf("Password(%d,...) has length %d, want %d", length, len(p), length)
		}
	}
}

func TestSpecsRegistry_Password_NonEmpty(t *testing.T) {
	if p := newRegistry().Password(16, 2, 2, false, false); p == "" {
		t.Error("Password() returned empty string")
	}
}
