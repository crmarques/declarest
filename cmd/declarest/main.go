package main

import (
	"os"

	"github.com/crmarques/declarest/internal/app"
	"github.com/crmarques/declarest/internal/cli"
)

func main() {
	container := app.NewContainer()
	if err := cli.Execute(container.CommandWiring()); err != nil {
		os.Exit(1)
	}
}
