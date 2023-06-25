/*
Copyright 2023 The Nuclio Authors.

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

	"github.com/nuclio/nuclio/cmd/dlx/app"
	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
)

func main() {
	kubeconfigPath := flag.String("kubeconfig-path", os.Getenv("KUBECONFIG"), "Path of kubeconfig file")
	namespace := flag.String("namespace", "", "Namespace to listen on, or * for all")
	platformConfigurationPath := flag.String("platform-config", "/etc/nuclio/config/platform/platform.yaml", "Path of platform configuration file")
	functionReadinessVerificationEnabled := flag.Bool("function-readiness-verification-enabled", common.GetEnvOrDefaultBool("NUCLIO_RESOURCESCALER_FUNCTION_READINESS_VERIFICATION_ENABLED", true), "Whether to verify function readiness")
	flag.Parse()

	*namespace = getNamespace(*namespace)

	if err := app.Run(*platformConfigurationPath,
		*namespace,
		*kubeconfigPath,
		*functionReadinessVerificationEnabled); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 5)
		os.Exit(1)
	}
}

func getNamespace(namespaceArgument string) string {

	// if the namespace was passed in the arguments, use that
	if namespaceArgument != "" {
		return namespaceArgument
	}

	// if the namespace exists in env, use that
	if namespaceEnv := os.Getenv("NUCLIO_SCALER_NAMESPACE"); namespaceEnv != "" {
		return namespaceEnv
	}

	if namespacePod, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		return string(namespacePod)
	}

	return "default"
}
