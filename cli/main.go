package main

import (
	"fmt"
	"os"

	"declarest/cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if !cmd.IsHandledError(err) {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}
