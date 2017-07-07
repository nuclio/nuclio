package functioncr

import (
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/logger"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
	v1b1e "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
)

type Client struct {
	logger     logger.Logger
	restClient *rest.RESTClient
	clientSet  *kubernetes.Clientset
}

func NewClient(parentLogger logger.Logger,
	restConfig *rest.Config,
	clientSet *kubernetes.Clientset) (*Client, error) {
	var err error

	newClient := &Client{
		logger:    parentLogger.GetChild("functioncr").(logger.Logger),
		clientSet: clientSet,
	}

	newClient.restClient, err = newClient.createRESTClient(restConfig, clientSet)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST client")
	}

	return newClient, nil
}

// registers the "class" into k8s
func (c *Client) CreateResource() error {
	c.logger.DebugWith("Creating resource", "name", c.getFullyQualifiedName())

	thirdPartyResource := &v1b1e.ThirdPartyResource{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: c.getFullyQualifiedName(),
		},
		Versions: []v1b1e.APIVersion{
			{
				Name: c.getVersion(),
			},
		},
		Description: c.getDescription(),
	}

	_, err := c.clientSet.Extensions().ThirdPartyResources().Create(thirdPartyResource)

	// if it already exists, we're good
	if err == nil {
		c.logger.Debug("Created resource")

		// wait for the resource to be ready
		return c.WaitForResource()

	} else if err != nil && apierrors.IsAlreadyExists(err) {
		c.logger.Debug("Resource already existed, skipping creation")

		// we're done
		return nil
	} else {
		return errors.Wrap(err, "Failed to create third part resource")
	}
}

func (c *Client) DeleteResource() error {
	return c.clientSet.Extensions().ThirdPartyResources().Delete(c.getFullyQualifiedName(), nil)
}

func (c *Client) WaitForResource() error {
	c.logger.Debug("Waiting for resource to be ready")

	return wait.Poll(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		_, err := c.restClient.Get().Namespace(v1.NamespaceDefault).Resource(c.getNamePlural()).DoRaw()
		if err != nil {

			// if the error is that it's not found, don't stop
			if apierrors.IsNotFound(err) {
				return false, nil
			}

			// something went wrong - stop and return error
			return true, err

		} else {
			c.logger.Debug("Resource is ready")

			// we're done
			return true, nil
		}
	})
}

func (c *Client) WatchForChanges(changeChan chan Change) (*Watcher, error) {
	return newWatcher(c, changeChan)
}

func (c *Client) Create(obj *Function) (*Function,error) {
	var result Function
	err := c.restClient.Post().
		Namespace(obj.ObjectMeta.Namespace).Resource(c.getNamePlural()).
		Body(obj).Do().Into(&result)
	return &result, err
}

func (c *Client) Update(obj *Function) (*Function,error) {
	var result Function
	err := c.restClient.Put().
		Namespace(obj.ObjectMeta.Namespace).Name(obj.ObjectMeta.Name).Resource(c.getNamePlural()).
		Body(obj).Do().Into(&result)
	return &result, err
}

func (c *Client) Delete(namespace, name string, options *meta_v1.DeleteOptions) error {
	return c.restClient.Delete().
		Namespace(namespace).Resource(c.getNamePlural()).
		Name(name).Body(options).Do().
		Error()
}

func (c *Client) Get(namespace, name string) (*Function,error) {
	var result Function
	err := c.restClient.Get().
		Namespace(namespace).Resource(c.getNamePlural()).
		Name(name).Do().Into(&result)
	return &result, err
}

func (c *Client) List(namespace string) (*FunctionList,error) {
	var result FunctionList
	err := c.restClient.Get().
		Namespace(namespace).Resource(c.getNamePlural()).
		Do().Into(&result)
	return &result, err
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
