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

package client

import (
	"context"
	"os"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioioclient "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
	// enable OIDC plugin
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type Consumer struct {
	NuclioClientSet nuclioioclient.Interface
	KubeClientSet   kubernetes.Interface
	KubeHost        string
	kubeconfigPath  string
}

func NewConsumer(ctx context.Context, logger logger.Logger, kubeconfigPath string) (*Consumer, error) {
	logger.DebugWithCtx(ctx, "Using kubeconfig", "kubeconfigPath", kubeconfigPath)

	newConsumer := Consumer{
		kubeconfigPath: kubeconfigPath,
	}

	// create REST config
	restConfig, err := common.GetClientConfig(kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST config")
	}

	// add bearer token if specified in environment
	token := os.Getenv("NUCLIO_KUBE_CONSUMER_BEARER_TOKEN")
	if token != "" {
		restConfig.BearerToken = token
	}

	// set kube host
	newConsumer.KubeHost = restConfig.Host

	// create KubeClientSet
	newConsumer.KubeClientSet, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create client set")
	}

	// create a client for function custom resources
	newConsumer.NuclioClientSet, err = nuclioioclient.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function custom resource client")
	}

	return &newConsumer, nil
}

func (c *Consumer) getNuclioClientSet(authConfig *platform.AuthConfig) (nuclioioclient.Interface, error) {

	// if no authentication was passed, can use the generic client. otherwise must create
	if authConfig == nil {
		return c.NuclioClientSet, nil
	}

	// create REST config
	restConfig, err := common.GetClientConfig(c.kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST config")
	}

	// set the auth provider config
	restConfig.BearerToken = authConfig.Token

	return nuclioioclient.NewForConfig(restConfig)
}
