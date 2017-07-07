package function

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

type CustomResource struct {
	logger     logger.Logger
	restClient *rest.RESTClient
	clientSet  *kubernetes.Clientset
}

func NewCustomResource(parentLogger logger.Logger,
	restConfig *rest.Config,
	clientSet *kubernetes.Clientset) (*CustomResource, error) {
	var err error

	newCustomResource := CustomResource{
		logger:    parentLogger.GetChild("function_cr").(logger.Logger),
		clientSet: clientSet,
	}

	newCustomResource.restClient, err = newCustomResource.createRESTClient(restConfig, clientSet)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create REST client")
	}

	return &newCustomResource, nil
}

// registers the "class" into k8s
func (cr *CustomResource) CreateResource() error {
	cr.logger.DebugWith("Creating resource", "name", cr.getFullyQualifiedName())

	thirdPartyResource := &v1b1e.ThirdPartyResource{
		ObjectMeta: meta_v1.ObjectMeta{
			Name: cr.getFullyQualifiedName(),
		},
		Versions: []v1b1e.APIVersion{
			{
				Name: cr.getVersion(),
			},
		},
		Description: cr.getDescription(),
	}

	_, err := cr.clientSet.Extensions().ThirdPartyResources().Create(thirdPartyResource)

	// if it already exists, we're good
	if err == nil {
		cr.logger.Debug("Created resource")

		// wait for the resource to be ready
		return cr.WaitForResource()

	} else if err != nil && apierrors.IsAlreadyExists(err) {
		cr.logger.Debug("Resource already existed, skipping creation")

		// we're done
		return nil
	} else {
		return errors.Wrap(err, "Failed to create third part resource")
	}
}

func (cr *CustomResource) DeleteResource() error {
	return cr.clientSet.Extensions().ThirdPartyResources().Delete(cr.getFullyQualifiedName(), nil)
}

func (cr *CustomResource) WaitForResource() error {
	cr.logger.Debug("Waiting for resource to be ready")

	return wait.Poll(100*time.Millisecond, 60*time.Second, func() (bool, error) {
		_, err := cr.restClient.Get().Namespace(v1.NamespaceDefault).Resource(cr.getNamePlural()).DoRaw()
		if err != nil {

			// if the error is that it's not found, don't stop
			if apierrors.IsNotFound(err) {
				return false, nil
			}

			// something went wrong - stop and return error
			return true, err

		} else {
			cr.logger.Debug("Resource is ready")

			// we're done
			return true, nil
		}
	})
}

func (cr *CustomResource) createRESTClient(restConfig *rest.Config,
	clientSet *kubernetes.Clientset) (*rest.RESTClient, error) {
	cr.logger.Debug("Creating REST client")

	scheme := runtime.NewScheme()
	schemeBuilder := runtime.NewSchemeBuilder(cr.getKnownType)
	schemeGroupVersion := cr.getGroupVersion()

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

func (cr *CustomResource) getKnownType(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(cr.getGroupVersion(),
		&Function{},
		&FunctionList{},
	)
	meta_v1.AddToGroupVersion(scheme, cr.getGroupVersion())
	return nil
}

func (cr *CustomResource) getGroupVersion() schema.GroupVersion {
	return schema.GroupVersion{
		Group:   cr.getGroupName(),
		Version: cr.getVersion(),
	}
}

func (cr *CustomResource) getFullyQualifiedName() string {
	return fmt.Sprintf("%s.%s", cr.getName(), cr.getGroupName())
}

func (cr *CustomResource) getName() string {
	return "function"
}

func (cr *CustomResource) getNamePlural() string {
	return "functions"
}

func (cr *CustomResource) getGroupName() string {
	return "nuclio.io"
}

func (cr *CustomResource) getVersion() string {
	return "v1"
}

func (cr *CustomResource) getDescription() string {
	return "Serverless function"
}
