/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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
		namespace = []byte("default")
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
