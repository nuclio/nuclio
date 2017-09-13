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

package nuctl

import (
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/functiondep"
	"github.com/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeConsumer struct {
	Clientset         *kubernetes.Clientset
	FunctioncrClient  *functioncr.Client
	FunctiondepClient *functiondep.Client
}

func (kc *KubeConsumer) GetClients(logger nuclio.Logger, kubeconfigPath string) (kubeHost string, clientsErr error) {
	logger.DebugWith("Using kubeconfig", "kubeconfigPath", kubeconfigPath)

	// create REST config
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		clientsErr = errors.Wrap(err, "Failed to create REST config")
		return
	}

	// set kube host
	kubeHost = restConfig.Host

	// create clientset
	kc.Clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		clientsErr = errors.Wrap(err, "Failed to create client set")
		return
	}

	// create a client for function custom resources
	kc.FunctioncrClient, err = functioncr.NewClient(logger, restConfig, kc.Clientset)
	if err != nil {
		clientsErr = errors.Wrap(err, "Failed to create function custom resource client")
		return
	}

	// create a client for function deployments
	kc.FunctiondepClient, err = functiondep.NewClient(logger, kc.Clientset)
	if err != nil {
		clientsErr = errors.Wrap(err, "Failed to create function deployment client")
		return
	}

	return
}
