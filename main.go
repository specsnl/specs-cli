package main

import (
	"errors"
	"os"

	"github.com/specsnl/specs-cli/pkg/cmd"
	"github.com/specsnl/specs-cli/pkg/util/exit"
)

func main() {
	app := cmd.NewApp()
	if err := cmd.Execute(app); err != nil {
		var exitErr *exit.ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		app.Output.Error("%v", err)
		os.Exit(exit.Error)
	}
}
