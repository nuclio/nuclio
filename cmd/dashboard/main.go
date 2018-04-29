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

	"github.com/nuclio/nuclio/cmd/dashboard/app"
	_ "github.com/nuclio/nuclio/pkg/dashboard/resource"
	"github.com/nuclio/nuclio/pkg/errors"
)

func main() {
	defaultNoPullBaseImages := os.Getenv("NUCLIO_DASHBOARD_NO_PULL_BASE_IMAGES") == "true"

	externalIPAddressesDefault := os.Getenv("NUCLIO_DASHBOARD_EXTERNAL_IP_ADDRESSES")
	if externalIPAddressesDefault == "" {
		externalIPAddressesDefault = "172.17.0.1"
	}

	listenAddress := flag.String("listen-addr", ":8070", "IP/port on which the playground listens")
	dockerKeyDir := flag.String("docker-key-dir", "", "Directory to look for docker keys for secure registries")
	platformType := flag.String("platform", "auto", "One of kube/local/auto")
	defaultRegistryURL := flag.String("registry", os.Getenv("NUCLIO_DASHBOARD_REGISTRY_URL"), "Default registry URL")
	defaultRunRegistryURL := flag.String("run-registry", os.Getenv("NUCLIO_DASHBOARD_RUN_REGISTRY_URL"), "Default run registry URL")
	noPullBaseImages := flag.Bool("no-pull", defaultNoPullBaseImages, "Default run registry URL")
	credsRefreshInterval := flag.String("creds-refresh-interval", os.Getenv("NUCLIO_DASHBOARD_CREDS_REFRESH_INTERVAL"), "Default credential refresh interval, or 'none' (12h by default)")
	externalIPAddresses := flag.String("external-ip-addresses", externalIPAddressesDefault, "Comma delimited list of external IP addresses")

	flag.Parse()

	if err := app.Run(*listenAddress,
		*dockerKeyDir,
		*defaultRegistryURL,
		*defaultRunRegistryURL,
		*platformType,
		*noPullBaseImages,
		*credsRefreshInterval,
		*externalIPAddresses); err != nil {

		errors.PrintErrorStack(os.Stderr, err, 5)

		os.Exit(1)
	}

	os.Exit(0)
}
