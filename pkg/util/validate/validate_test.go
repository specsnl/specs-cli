package validate_test

import (
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func TestName_Valid(t *testing.T) {
	for _, name := range []string{"my-template", "template_1", "UPPER", "a", "mix-123_ABC"} {
		if err := validate.Name(name); err != nil {
			t.Errorf("Name(%q) = %v, want nil", name, err)
		}
	}
}

func TestName_Empty(t *testing.T) {
	if err := validate.Name(""); err == nil {
		t.Error("Name(\"\") = nil, want error")
	}
}

func TestName_InvalidChars(t *testing.T) {
	for _, name := range []string{"foo bar", "foo/bar", "foo.bar", "foo@bar", "foo:bar"} {
		if err := validate.Name(name); err == nil {
			t.Errorf("Name(%q) = nil, want error", name)
		}
	}
}
