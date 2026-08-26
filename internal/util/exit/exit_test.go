package exit_test

import (
	"errors"
	"testing"

	"github.com/specsnl/specs-cli/internal/util/exit"
)

func TestExitError_Error(t *testing.T) {
	tests := []struct {
		name string
		code int
		want string
	}{
		{name: "generic error", code: exit.Error, want: "exit status 1"},
		{name: "ok", code: exit.OK, want: "exit status 0"},
		{name: "combined validate bits", code: exit.ValidateUnknown | exit.ValidateRender, want: "exit status 6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &exit.ExitError{Code: tt.code}
			if got := err.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// main.go reaches for the code through errors.As, so an ExitError has to survive
// wrapping.
func TestExitError_UnwrapsFromWrappedError(t *testing.T) {
	wrapped := errors.Join(errors.New("context"), &exit.ExitError{Code: exit.ValidateUnused})

	var target *exit.ExitError
	if !errors.As(wrapped, &target) {
		t.Fatalf("errors.As did not find an *ExitError in %v", wrapped)
	}

	if target.Code != exit.ValidateUnused {
		t.Errorf("Code = %d, want %d", target.Code, exit.ValidateUnused)
	}
}

// The validate codes are a bitmask: each condition owns one bit so several can
// be reported in a single exit status.
func TestValidateCodesAreDistinctBits(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{name: "unused", code: exit.ValidateUnused, want: 1},
		{name: "unknown", code: exit.ValidateUnknown, want: 2},
		{name: "render", code: exit.ValidateRender, want: 4},
		{name: "unused and unknown", code: exit.ValidateUnused | exit.ValidateUnknown, want: 3},
		{
			name: "all three",
			code: exit.ValidateUnused | exit.ValidateUnknown | exit.ValidateRender,
			want: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code != tt.want {
				t.Errorf("code = %d, want %d", tt.code, tt.want)
			}
		})
	}
}
