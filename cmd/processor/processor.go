package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/nuclio/nuclio/cmd/processor/app"
)

func run() error {
	configPath := flag.String("config", "", "Path of configuration file")
	flag.Parse()

	processor, err := app.NewProcessor(*configPath)
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
