package main

import (
	"os"

	"github.com/nuclio/nuclio/cmd/nuclio-deploy/app"
)

func main() {
	if err := app.Run(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
