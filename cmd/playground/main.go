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
	"os"

	"github.com/nuclio/nuclio/cmd/playground/app"
	_ "github.com/nuclio/nuclio/pkg/playground/resource"
)

func main() {

	defaultRegistry := os.Getenv("NUCLIO_PLAYGROUND_REGISTRY_URL")
	if defaultRegistry == "" {
		defaultRegistry = "127.0.0.1:5000"
	}

	defaultNoPullBaseImages := os.Getenv("NUCLIO_PLAYGROUND_NO_PULL_BASE_IMAGES") == "true"

	listenAddress := flag.String("listen-addr", ":8070", "Path of configuration file")
	assetsDir := flag.String("assets-dir", "", "Path of configuration file")
	sourcesDir := flag.String("sources-dir", "", "Directory to save sources")
	dockerKeyDir := flag.String("docker-key-dir", "", "Directory to look for docker keys for secure registries")
	platformType := flag.String("platform", "auto", "One of kube/local/auto")
	defaultRegistryURL := flag.String("registry", defaultRegistry, "Default registry URL")
	defaultRunRegistryURL := flag.String("run-registry", os.Getenv("NUCLIO_PLAYGROUND_RUN_REGISTRY_URL"), "Default run registry URL")
	noPullBaseImages := flag.Bool("no-pull", defaultNoPullBaseImages, "Default run registry URL")

	flag.Parse()

	if err := app.Run(*listenAddress,
		*assetsDir,
		*sourcesDir,
		*dockerKeyDir,
		*defaultRegistryURL,
		*defaultRunRegistryURL,
		*platformType,
		*noPullBaseImages); err != nil {
		os.Exit(1)
	}

	os.Exit(0)
}
