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
	"io/ioutil"
	"os"

	"github.com/nuclio/nuclio/cmd/controller/app"
	"github.com/nuclio/nuclio/pkg/errors"
)

func getNamespace(namespaceArgument string) string {

	// if the namespace was passed in the arguments, use that
	if namespaceArgument != "" {
		return namespaceArgument
	}

	// if the namespace exists in env, use that
	if namespaceEnv := os.Getenv("NUCLIO_CONTROLLER_NAMESPACE"); namespaceEnv != "" {
		return namespaceEnv
	}

	// if nothing was passed, listen on all namespaces
	return "*"
}

func main() {
	kubeconfigPath := flag.String("kubeconfig-path", "", "Path of kubeconfig file")
	namespace := flag.String("namespace", "", "Namespace to listen on, or * for all")
	imagePullSecrets := flag.String("image-pull-secrets", os.Getenv("NUCLIO_CONTROLLER_IMAGE_PULL_SECRETS"), "Optional secret name to use for pull")
	flag.Parse()

	// get the namespace from args -> env -> default (*)
	resolvedNamespace := getNamespace(*namespace)

	// if the namespace is set to @nuclio.selfNamespace, use the namespace we're in right now
	if resolvedNamespace == "@nuclio.selfNamespace" {

		// get namespace from within the pod. if found, return that
		if namespacePod, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			resolvedNamespace = string(namespacePod)
		}
	}

	if err := app.Run(*kubeconfigPath, resolvedNamespace, *imagePullSecrets); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 5)

		os.Exit(1)
	}
}
