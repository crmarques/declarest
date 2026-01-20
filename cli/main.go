package main

import (
	"fmt"
	"os"

	"github.com/crmarques/declarest/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		cmd.ReportDebug(err, os.Stderr)
		if !cmd.IsHandledError(err) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
	cmd.ReportDebug(nil, os.Stdout)
}
