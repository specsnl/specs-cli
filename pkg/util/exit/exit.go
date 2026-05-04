package exit

import "fmt"

const (
	OK    = 0
	Error = 1

	// Bitmask exit codes for validate. Multiple conditions combine additively.
	ValidateUnused  = 1 // bit 0: unused variable/computed value (--strict only)
	ValidateUnknown = 2 // bit 1: unknown variable referenced in a template file
)

// ExitError is a silent error that carries a specific exit code. main.go uses
// os.Exit with Code directly and does not print an error message.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}
