package specs

// Delimiters holds the left and right action delimiters used when parsing and
// executing templates. Storing them as a named type (rather than bare string
// constants) lets the value be carried on Config and threaded through every
// call site, making it straightforward to read custom delimiters from
// project.yaml in the future without further restructuring.
type Delimiters struct {
	Left  string
	Right string
}

// DefaultDelimiters are the standard specs template delimiters.
// They match Go's native text/template syntax so template authors can use
// familiar tooling. Templates that need to output literal {{ }} (e.g. GitHub
// Actions YAML) can override this with __delimiters in project.yaml.
var DefaultDelimiters = Delimiters{Left: "{{", Right: "}}"}
