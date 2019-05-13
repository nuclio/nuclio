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

package kube

import (
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
	// enable OIDC plugin
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type consumer struct {
	kubeClientSet   kubernetes.Interface
	nuclioClientSet nuclioio_client.Interface
	kubeHost        string
	kubeconfigPath  string
}

func newConsumer(logger logger.Logger, kubeconfigPath string) (*consumer, error) {
	logger.DebugWith("Using kubeconfig", "kubeconfigPath", kubeconfigPath)

	newConsumer := consumer{
		kubeconfigPath: kubeconfigPath,
	}

	// create REST config
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST config")
	}

	// set kube host
	newConsumer.kubeHost = restConfig.Host

	// create kubeClientSet
	newConsumer.kubeClientSet, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create client set")
	}

	// create a client for function custom resources
	newConsumer.nuclioClientSet, err = nuclioio_client.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function custom resource client")
	}

	return &newConsumer, nil
}

func (c *consumer) getNuclioClientSet(authConfig *platform.AuthConfig) (nuclioio_client.Interface, error) {

	// if no authentication was passed, can use the generic client. otherwise must create
	if authConfig == nil {
		return c.nuclioClientSet, nil
	}

	// create REST config
	restConfig, err := clientcmd.BuildConfigFromFlags("", c.kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST config")
	}

	// set the auth provider config
	restConfig.AuthProvider = &clientcmdapi.AuthProviderConfig{
		Name:   authConfig.Name,
		Config: authConfig.Config,
	}

	return nuclioio_client.NewForConfig(restConfig)
}
