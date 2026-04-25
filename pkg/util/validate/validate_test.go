package validate_test

import (
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/validate"
)

func TestTag_Valid(t *testing.T) {
	for _, tag := range []string{"my-template", "template_1", "UPPER", "a", "mix-123_ABC"} {
		if err := validate.Tag(tag); err != nil {
			t.Errorf("Tag(%q) = %v, want nil", tag, err)
		}
	}
}

func TestTag_Empty(t *testing.T) {
	if err := validate.Tag(""); err == nil {
		t.Error("Tag(\"\") = nil, want error")
	}
}

func TestTag_InvalidChars(t *testing.T) {
	for _, tag := range []string{"foo bar", "foo/bar", "foo.bar", "foo@bar", "foo:bar"} {
		if err := validate.Tag(tag); err == nil {
			t.Errorf("Tag(%q) = nil, want error", tag)
		}
	}
}
