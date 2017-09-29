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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/functiondep"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type KubeConsumer struct {
	Clientset         *kubernetes.Clientset
	FunctioncrClient  *functioncr.Client
	FunctiondepClient *functiondep.Client
	KubeHost          string
}

func NewKubeConsumer(logger nuclio.Logger, kubeconfigPath string) (*KubeConsumer, error) {
	logger.DebugWith("Using kubeconfig", "kubeconfigPath", kubeconfigPath)

	newKubeConsumer := KubeConsumer{}

	// create REST config
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST config")
	}

	// set kube host
	newKubeConsumer.KubeHost = restConfig.Host

	// create clientset
	newKubeConsumer.Clientset, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create client set")
	}

	// create a client for function custom resources
	newKubeConsumer.FunctioncrClient, err = functioncr.NewClient(logger, restConfig, newKubeConsumer.Clientset)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function custom resource client")
	}

	// create a client for function deployments
	newKubeConsumer.FunctiondepClient, err = functiondep.NewClient(logger, newKubeConsumer.Clientset)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function deployment client")
	}

	return &newKubeConsumer, nil
}
