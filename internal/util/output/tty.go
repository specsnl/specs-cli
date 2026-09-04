package output

import "github.com/charmbracelet/x/term"

// fileDescriptor is the only thing a terminal check actually needs.
type fileDescriptor interface {
	Fd() uintptr
}

// IsTTY reports whether stream is an interactive terminal.
//
// The parameter is any rather than io.Writer deliberately: the question that
// matters most here is about stdin, because the failure being prevented is a
// read with nobody to answer it. A job with a terminal on stderr and its stdin
// closed must still refuse to prompt, and cmd.InOrStdin() hands back an
// io.Reader.
//
// Anything without a file descriptor — a bytes.Buffer in a test, an os.Pipe read
// end — is not a terminal.
func IsTTY(stream any) bool {
	f, ok := stream.(fileDescriptor)
	if !ok {
		return false
	}

	return term.IsTerminal(f.Fd())
}
