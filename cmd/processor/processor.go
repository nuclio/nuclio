package main

import (
	"fmt"
	"os"

	"github.com/nuclio/nuclio/cmd/processor/app"
)

func run() error {

	processor, err := app.NewProcessor("test/e2e/config/nuclio.yaml")
	if err != nil {
		return err
	}

	return processor.Start()
}

func main() {

	if err := run(); err != nil {
		fmt.Printf("Failed to run processor: %s", err)

		os.Exit(1)
	}

	os.Exit(0)
}
