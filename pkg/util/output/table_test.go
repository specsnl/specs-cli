package output_test

import (
	"strings"
	"testing"

	"github.com/specsnl/specs-cli/pkg/util/output"
)

func TestRenderTable_ContainsHeaders(t *testing.T) {
	out := output.RenderTable(
		[]string{"Tag", "Repository", "Created"},
		[][]string{{"my-tag", "user/repo", "2 days ago"}},
	)
	if !strings.Contains(out, "Tag") {
		t.Error("table output does not contain header 'Tag'")
	}
	if !strings.Contains(out, "my-tag") {
		t.Error("table output does not contain row value 'my-tag'")
	}
}

func TestRenderTable_MultipleRows(t *testing.T) {
	out := output.RenderTable(
		[]string{"Name", "Value"},
		[][]string{
			{"alpha", "1"},
			{"beta", "2"},
		},
	)
	if !strings.Contains(out, "alpha") {
		t.Error("table output does not contain 'alpha'")
	}
	if !strings.Contains(out, "beta") {
		t.Error("table output does not contain 'beta'")
	}
}
