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

package functioncr

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/nuclio/nuclio-sdk"
	apiex_v1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiex_client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	logger         nuclio.Logger
	restClient     *rest.RESTClient
	clientSet      *kubernetes.Clientset
	apiexClientSet *apiex_client.Clientset
	parameterCodec runtime.ParameterCodec
}

func NewClient(parentLogger nuclio.Logger,
	restConfig *rest.Config,
	clientSet *kubernetes.Clientset) (*Client, error) {
	var err error

	newClient := &Client{
		logger:    parentLogger.GetChild("functioncr"),
		clientSet: clientSet,
	}

	newClient.restClient, err = newClient.createRESTClient(restConfig, clientSet)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST client")
	}

	newClient.apiexClientSet, err = apiex_client.NewForConfig(restConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create apiextensions client set")
	}

	return newClient, nil
}

// registers the "class" into k8s (CRDs are not namespaced)
func (c *Client) CreateResource() error {
	c.logger.DebugWith("Creating resource", "name", c.getFullyQualifiedName())

	customResource := apiex_v1beta1.CustomResourceDefinition{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", c.getNamePlural(), c.getGroupName()),
		},
		Spec: apiex_v1beta1.CustomResourceDefinitionSpec{
			Version: c.getVersion(),
			Scope:   apiex_v1beta1.NamespaceScoped,
			Group:   c.getGroupName(),
			Names: apiex_v1beta1.CustomResourceDefinitionNames{
				Singular: c.getName(),
				Plural:   c.getNamePlural(),
				Kind:     "Function",
				ListKind: "FunctionList",
			},
		},
	}

	_, err := c.apiexClientSet.ApiextensionsV1beta1Client.CustomResourceDefinitions().Create(&customResource)
	if err != nil {

		// if it already existed, there's no err
		if !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "Failed to create custom resource")
		}

		c.logger.Debug("Resource already existed, skipping creation")

	} else {
		c.logger.Debug("Created resource")
	}

	// wait for the resource to be ready
	return c.WaitForResource()
}

func (c *Client) DeleteResource() error {
	return c.apiexClientSet.ApiextensionsV1beta1Client.CustomResourceDefinitions().Delete(c.getFullyQualifiedName(), nil)
}

func (c *Client) WaitForResource() error {
	c.logger.Debug("Waiting for resource to be ready")

	return wait.Poll(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		_, err := c.restClient.Get().Resource(c.getNamePlural()).DoRaw()
		if err != nil {

			// if the error is that it's not found, don't stop
			if apierrors.IsNotFound(err) {
				return false, nil
			}

			// something went wrong - stop and return error
			return true, err

		}

		c.logger.Debug("Resource is ready")

		// we're done
		return true, nil
	})
}

func (c *Client) WatchForChanges(namespace string, changeChan chan Change) (*Watcher, error) {
	return newWatcher(c, namespace, changeChan)
}

func (c *Client) Create(function *Function) (*Function, error) {
	var result Function
	err := c.restClient.Post().
		Namespace(function.ObjectMeta.Namespace).Resource(c.getNamePlural()).
		Body(function).Do().Into(&result)

	result.Sanitize()
	return &result, err
}

func (c *Client) Update(function *Function) (*Function, error) {
	var result Function
	err := c.restClient.Put().
		Namespace(function.ObjectMeta.Namespace).Name(function.ObjectMeta.Name).Resource(c.getNamePlural()).
		Body(function).Do().Into(&result)

	result.Sanitize()
	return &result, err
}

func (c *Client) Delete(namespace, name string, options *meta_v1.DeleteOptions) error {
	return c.restClient.Delete().
		Namespace(namespace).Resource(c.getNamePlural()).
		Name(name).Body(options).Do().
		Error()
}

func (c *Client) Get(namespace, name string) (*Function, error) {
	var result Function
	err := c.restClient.Get().
		Namespace(namespace).Resource(c.getNamePlural()).
		Name(name).Do().Into(&result)

	result.Sanitize()
	return &result, err
}

func (c *Client) List(namespace string, options *meta_v1.ListOptions) (*FunctionList, error) {
	var result FunctionList
	err := c.restClient.Get().
		Namespace(namespace).Resource(c.getNamePlural()).
		VersionedParams(options, c.parameterCodec).
		Do().Into(&result)

	for _, function := range result.Items {
		function.Sanitize()
	}

	return &result, err
}

func (c *Client) WaitUntilCondition(namespace, name string, condition func(*Function) (bool, error), timeout time.Duration) error {
	return wait.Poll(250*time.Millisecond, timeout, func() (bool, error) {

		// get the appropriate function CR
		functioncrInstance, err := c.Get(namespace, name)
		if err != nil {
			return true, err
		}

		// call the callback
		return condition(functioncrInstance)
	})
}

func (c *Client) createRESTClient(restConfig *rest.Config,
	clientSet *kubernetes.Clientset) (*rest.RESTClient, error) {
	c.logger.Debug("Creating REST client")

	scheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(c.getKnownType)
	schemeGroupVersion := c.getGroupVersion()

	if err := schemeBuilder.AddToScheme(scheme); err != nil {
		return nil, errors.Wrap(err, "Failed to add scheme to builder")
	}

	// create parameter codec
	c.parameterCodec = runtime.NewParameterCodec(scheme)

	restConfigCopy := *restConfig
	restConfigCopy.GroupVersion = &schemeGroupVersion
	restConfigCopy.APIPath = "/apis"
	restConfigCopy.ContentType = runtime.ContentTypeJSON
	restConfigCopy.NegotiatedSerializer = serializer.DirectCodecFactory{
		CodecFactory: serializer.NewCodecFactory(scheme),
	}

	return rest.RESTClientFor(&restConfigCopy)
}

func (c *Client) getKnownType(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(c.getGroupVersion(),
		&Function{},
		&FunctionList{},
	)
	meta_v1.AddToGroupVersion(scheme, c.getGroupVersion())
	return nil
}

func (c *Client) getGroupVersion() schema.GroupVersion {
	return schema.GroupVersion{
		Group:   c.getGroupName(),
		Version: c.getVersion(),
	}
}

func (c *Client) getFullyQualifiedName() string {
	return fmt.Sprintf("%s.%s", c.getName(), c.getGroupName())
}

func (c *Client) getName() string {
	return "function"
}

func (c *Client) getNamePlural() string {
	return "functions"
}

func (c *Client) getGroupName() string {
	return "nuclio.io"
}

func (c *Client) getVersion() string {
	return "v1"
}

func (c *Client) getDescription() string {
	return "Serverless function"
}
