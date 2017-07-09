package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nuclio/nuclio/cmd/controller/app"
)

func run() error {
	configPath := flag.String("config", "", "Path of configuration file")
	flag.Parse()

	controller, err := app.NewController(*configPath)
	if err != nil {
		return err
	}

	return controller.Start()
}

func main() {

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to run controller: %s", err)

		os.Exit(1)
	}
}
