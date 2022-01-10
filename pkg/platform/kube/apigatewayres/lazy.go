package apigatewayres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platform/kube"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//
// Client
//

type lazyClient struct {
	logger          logger.Logger
	kubeClientSet   kubernetes.Interface
	nuclioClientSet nuclioio_client.Interface
	ingressManager  *ingress.Manager
}

func NewLazyClient(loggerInstance logger.Logger,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioio_client.Interface,
	ingressManager *ingress.Manager) (Client, error) {

	newClient := lazyClient{
		logger:          loggerInstance.GetChild("apigatewayres"),
		kubeClientSet:   kubeClientSet,
		nuclioClientSet: nuclioClientSet,
		ingressManager:  ingressManager,
	}

	return &newClient, nil
}

func (lc *lazyClient) List(ctx context.Context, namespace string) ([]Resources, error) {
	return nil, errors.New("Method not implemented")
}

func (lc *lazyClient) Get(ctx context.Context, namespace string, name string) (Resources, error) {
	return nil, errors.New("Method not implemented")
}

func (lc *lazyClient) CreateOrUpdate(ctx context.Context, apiGateway *nuclioio.NuclioAPIGateway) (Resources, error) {
	apiGateway.Status.Name = apiGateway.Spec.Name

	if err := lc.validateSpec(ctx, apiGateway); err != nil {
		return nil, errors.Wrap(err, "Api gateway spec validation failed")
	}

	// always try to remove previous canary ingress first, because
	// nginx returns 503 on all requests if primary service == secondary service. (happens on every promotion)
	// so during promotion all requests will be sent to the primary ingress
	lc.tryRemovePreviousCanaryIngress(ctx, apiGateway)

	// generate an ingress for each upstream
	ingressesResources := map[string]*ingress.Resources{}
	var ingressesToCreate []*ingress.Resources

	primaryUpstream, canaryUpstream, err := lc.resolveBaseAndCanaryUpstreamsFromSpec(apiGateway.Spec.Upstreams)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve base and canary upstreams")
	}

	// create primary ingress
	primaryIngressResources, err := lc.generateNginxIngress(ctx, apiGateway, primaryUpstream)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to generate primary nginx ingress")
	}

	lc.enrichPrimaryIngressResources(primaryIngressResources, primaryUpstream, canaryUpstream)

	ingressesResources[primaryIngressResources.Ingress.Name] = primaryIngressResources
	ingressesToCreate = append(ingressesToCreate, primaryIngressResources)

	// add the canary ingress
	if canaryUpstream != nil {

		// percentage range
		if canaryUpstream.Percentage > 100 || canaryUpstream.Percentage < 1 {
			return nil, errors.New("The canary upstream percentage must be between 1 and 100")
		}

		canaryIngressResources, err := lc.generateNginxIngress(ctx, apiGateway, canaryUpstream)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to generate the canary nginx ingress")
		}
		ingressesResources[canaryIngressResources.Ingress.Name] = canaryIngressResources
		ingressesToCreate = append(ingressesToCreate, canaryIngressResources)
	}

	// create ingresses
	// must be done synchronously, first primary and then canary
	// otherwise, when there is only canary ingress, the endpoint will not work (nginx behavior)
	for _, ingressResources := range ingressesToCreate {
		if _, _, err := lc.ingressManager.CreateOrUpdateResources(ctx, ingressResources); err != nil {
			lc.logger.WarnWithCtx(ctx, "Failed to create/update api gateway ingress resources",
				"err", errors.Cause(err),
				"ingressName", ingressResources.Ingress.Name)
			return nil, errors.New("Failed to create/update api gateway ingress resources")
		}
	}

	return &lazyResources{
		ingressResourcesMap: ingressesResources,
	}, nil
}

func (lc *lazyClient) WaitAvailable(ctx context.Context, namespace string, name string) {
	lc.logger.DebugWithCtx(ctx, "Sleeping for 4 seconds so nginx controller will stabilize")

	// sleep 4 seconds as a safety, so nginx will finish updating the ingresses properly (it takes time)
	time.Sleep(4 * time.Second)
}

func (lc *lazyClient) Delete(ctx context.Context, namespace string, name string) {
	lc.logger.DebugWithCtx(ctx, "Deleting api gateway base ingress", "name", name)

	if err := lc.ingressManager.DeleteByName(ctx,
		kube.IngressNameFromAPIGatewayName(name, false),
		namespace,
		true); err != nil {
		lc.logger.WarnWithCtx(ctx, "Failed to delete base ingress. Continuing with deletion",
			"err", errors.Cause(err).Error())
	}

	lc.logger.DebugWithCtx(ctx, "Deleting api gateway canary ingress", "name", name)
	if err := lc.ingressManager.DeleteByName(ctx,
		kube.IngressNameFromAPIGatewayName(name, true),
		namespace,
		true); err != nil {
		lc.logger.WarnWithCtx(ctx, "Failed to delete canary ingress. Continuing with deletion",
			"err", errors.Cause(err).Error())
	}
}

func (lc *lazyClient) tryRemovePreviousCanaryIngress(ctx context.Context, apiGateway *nuclioio.NuclioAPIGateway) {
	lc.logger.DebugWithCtx(ctx,
		"Trying to remove previous canary ingress",
		"apiGatewayName", apiGateway.Name)

	// remove old canary ingress if it exists
	// this works thanks to an assumption that ingress names == api gateway name
	previousCanaryIngressName := kube.IngressNameFromAPIGatewayName(apiGateway.Name, true)
	if err := lc.ingressManager.DeleteByName(ctx,
		previousCanaryIngressName,
		apiGateway.Namespace,
		true); err != nil {
		lc.logger.WarnWithCtx(ctx,
			"Failed to delete previous canary ingress on api gateway update",
			"previousCanaryIngressName", previousCanaryIngressName,
			"err", errors.Cause(err))
	}
}

func (lc *lazyClient) validateSpec(ctx context.Context, apiGateway *nuclioio.NuclioAPIGateway) error {
	upstreams := apiGateway.Spec.Upstreams

	if err := kube.ValidateAPIGatewaySpec(&apiGateway.Spec); err != nil {
		return err
	}

	// make sure each upstream is unique - meaning, there's no other api gateway with an upstream with the
	// same service (currently only nuclio function) name
	// (this is done because creating multiple ingresses with the same service name breaks nginx ingress controller)
	existingUpstreamFunctionNames, err := lc.getAllExistingUpstreamFunctionNames(ctx, apiGateway.Namespace, apiGateway.Name)
	if err != nil {
		return errors.Wrap(err, "Failed while getting all existing upstreams")
	}
	for _, upstream := range upstreams {
		if common.StringSliceContainsString(existingUpstreamFunctionNames, upstream.NuclioFunction.Name) {
			return errors.Errorf("Nuclio function '%s' is already being used in another api gateway",
				upstream.NuclioFunction.Name)
		}
	}

	return nil
}

func (lc *lazyClient) getAllExistingUpstreamFunctionNames(ctx context.Context, namespace, apiGatewayNameToExclude string) ([]string, error) {
	var existingUpstreamNames []string

	existingAPIGateways, err := lc.nuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(namespace).
		List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list existing api gateways")
	}

	for _, apiGateway := range existingAPIGateways.Items {
		if apiGateway.Name == apiGatewayNameToExclude {
			continue
		}

		for _, upstream := range apiGateway.Spec.Upstreams {
			existingUpstreamNames = append(existingUpstreamNames, upstream.NuclioFunction.Name)
		}
	}

	return existingUpstreamNames, nil
}

func (lc *lazyClient) generateNginxIngress(ctx context.Context,
	apiGateway *nuclioio.NuclioAPIGateway,
	upstream *platform.APIGatewayUpstreamSpec) (*ingress.Resources, error) {

	serviceName, servicePort, err := lc.getServiceNameAndPort(upstream)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get service name")
	}

	// add "/" as path prefix if not already there
	if !strings.HasPrefix(apiGateway.Spec.Path, "/") {
		apiGateway.Spec.Path = fmt.Sprintf("/%s", apiGateway.Spec.Path)
	}

	commonIngressSpec := ingress.Spec{
		APIGatewayName: apiGateway.Name,
		Namespace:      apiGateway.Namespace,
		ProjectName:    apiGateway.Labels[common.NuclioResourceLabelKeyProjectName],
		Host:           apiGateway.Spec.Host,
		Path:           apiGateway.Spec.Path,
		ServiceName:    serviceName,
		ServicePort:    servicePort,
		RewriteTarget:  upstream.RewriteTarget,
	}

	switch apiGateway.Spec.AuthenticationMode {
	case ingress.AuthenticationModeNone:
		commonIngressSpec.AuthenticationMode = ingress.AuthenticationModeNone
	case ingress.AuthenticationModeBasicAuth:
		if apiGateway.Spec.Authentication == nil || apiGateway.Spec.Authentication.BasicAuth == nil {
			return nil, errors.New("Basic auth specified but missing basic auth spec")
		}
		commonIngressSpec.AuthenticationMode = ingress.AuthenticationModeBasicAuth
		commonIngressSpec.Authentication = &ingress.Authentication{
			BasicAuth: &ingress.BasicAuth{
				Name:     kube.BasicAuthNameFromAPIGatewayName(apiGateway.Name),
				Username: apiGateway.Spec.Authentication.BasicAuth.Username,
				Password: apiGateway.Spec.Authentication.BasicAuth.Password,
			},
		}
	case ingress.AuthenticationModeOauth2:
		commonIngressSpec.AuthenticationMode = ingress.AuthenticationModeOauth2
		if apiGateway.Spec.Authentication != nil && apiGateway.Spec.Authentication.DexAuth != nil {
			commonIngressSpec.Authentication = &ingress.Authentication{
				DexAuth: apiGateway.Spec.Authentication.DexAuth,
			}
		}
	case ingress.AuthenticationModeAccessKey:
		commonIngressSpec.AuthenticationMode = ingress.AuthenticationModeAccessKey
	default:
		return nil, errors.New("Unsupported ApiGateway authentication mode provided")
	}

	// if percentage is given, it is the canary deployment
	canaryDeployment := upstream.Percentage != 0
	commonIngressSpec.Name = kube.IngressNameFromAPIGatewayName(apiGateway.Name, canaryDeployment)

	commonIngressSpec.Annotations = lc.resolveCommonAnnotations(canaryDeployment, upstream.Percentage)
	for annotationKey, annotationValue := range upstream.ExtraAnnotations {
		commonIngressSpec.Annotations[annotationKey] = annotationValue
	}

	// enrich ingress pathType
	if commonIngressSpec.PathType == nil {
		defaultPathType := networkingv1.PathTypeImplementationSpecific
		commonIngressSpec.PathType = &defaultPathType
	}

	return lc.ingressManager.GenerateResources(ctx, commonIngressSpec)
}

func (lc *lazyClient) getServiceNameAndPort(upstream *platform.APIGatewayUpstreamSpec) (string, int, error) {
	switch upstream.Kind {
	case platform.APIGatewayUpstreamKindNuclioFunction:

		// we used to get service name by actually getting the function's service
		// it was "stupified" to this logic, in order to prevent api-gateway failing when a function has no service
		// (which may happen when a function is imported, but not yet deployed, and in that point we import an api-gateway
		// that has this function as an upstream)
		serviceName := kube.ServiceNameFromFunctionName(upstream.NuclioFunction.Name)

		// use default port
		return serviceName, abstract.FunctionContainerHTTPPort, nil
	default:
		return "", 0, errors.Errorf("Unsupported API gateway upstream kind: %s", upstream.Kind)
	}
}

func (lc *lazyClient) resolveCommonAnnotations(canaryDeployment bool, upstreamPercentage int) map[string]string {
	annotations := map[string]string{}

	// add nginx specific annotations
	annotations["kubernetes.io/ingress.class"] = "nginx"

	// add canary deployment specific annotations
	if canaryDeployment {
		annotations["nginx.ingress.kubernetes.io/canary"] = "true"
		annotations["nginx.ingress.kubernetes.io/canary-weight"] = strconv.Itoa(upstreamPercentage)
	}
	return annotations
}

func (lc *lazyClient) resolveBaseAndCanaryUpstreamsFromSpec(upstreams []platform.APIGatewayUpstreamSpec) (
	*platform.APIGatewayUpstreamSpec, *platform.APIGatewayUpstreamSpec, error) {

	if len(upstreams) == 1 {
		return &upstreams[0], nil, nil
	}

	var primary *platform.APIGatewayUpstreamSpec
	var canary *platform.APIGatewayUpstreamSpec

	// determine which upstream is the canary one
	switch {
	case upstreams[0].Percentage != 0:
		primary = &upstreams[1]
		canary = &upstreams[0]
	case upstreams[1].Percentage != 0:
		primary = &upstreams[0]
		canary = &upstreams[1]
	default:
		return nil, nil, errors.New("Percentage must be set on one of the upstreams (canary)")
	}

	return primary, canary, nil
}

func (lc *lazyClient) enrichPrimaryIngressResources(primaryIngressResources *ingress.Resources,
	primaryUpstream *platform.APIGatewayUpstreamSpec,
	canaryUpstream *platform.APIGatewayUpstreamSpec) {

	// set nuclio target header on ingress
	targetHeaderValue := primaryUpstream.NuclioFunction.Name
	if canaryUpstream != nil {
		targetHeaderValue = fmt.Sprintf(`%s,%s`,
			primaryUpstream.NuclioFunction.Name,
			canaryUpstream.NuclioFunction.Name)
	}
	encodedPrimaryTargetHeader := fmt.Sprintf(`proxy_set_header X-Nuclio-Target "%s";`, targetHeaderValue)
	annotations := primaryIngressResources.Ingress.Annotations
	configurationSnippetHeaderName := "nginx.ingress.kubernetes.io/configuration-snippet"

	if _, headerExists := annotations[configurationSnippetHeaderName]; headerExists {
		annotations[configurationSnippetHeaderName] += fmt.Sprintf("\n%s", encodedPrimaryTargetHeader)
	} else {
		annotations[configurationSnippetHeaderName] = encodedPrimaryTargetHeader
	}
}

//
// Resources
//

type lazyResources struct {
	ingressResourcesMap map[string]*ingress.Resources
}

// Deployment returns the deployment
func (lr *lazyResources) IngressResourcesMap() map[string]*ingress.Resources {
	return lr.ingressResourcesMap
}
