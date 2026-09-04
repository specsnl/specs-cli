package output_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/specsnl/specs-cli/internal/util/output"
)

func TestIsTTY(t *testing.T) {
	// A pipe has a file descriptor but is not a terminal, which is exactly the
	// case that matters: a CI runner's stdin.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = devNull.Close() })

	tests := []struct {
		name   string
		stream any
		want   bool
	}{
		{"a buffer has no file descriptor", &bytes.Buffer{}, false},
		{"nil", nil, false},
		{"the read end of a pipe", r, false},
		{"the write end of a pipe", w, false},
		{"/dev/null", devNull, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := output.IsTTY(tt.stream); got != tt.want {
				t.Errorf("IsTTY(%T) = %v, want %v", tt.stream, got, tt.want)
			}
		})
	}
}
