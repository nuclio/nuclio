package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/nuclio/nuclio/cmd/controller/app"
)

func run() error {
	configPath := flag.String("config", "", "Path of configuration file")
	flag.Parse()

	// get namespace from within the pod if applicable
	namespace, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		namespace = []byte("undefined")
	}

	controller, err := app.NewController(string(namespace), *configPath)
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
