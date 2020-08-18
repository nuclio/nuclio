package apigatewayres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"github.com/nuclio/nuclio/pkg/platform/kube/ingress"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//
// Client
//

type lazyClient struct {
	Logger          logger.Logger
	kubeClientSet   kubernetes.Interface
	nuclioClientSet nuclioio_client.Interface
	ingressManager  *ingress.Manager
}

func NewLazyClient(loggerInstance logger.Logger,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioio_client.Interface,
	ingressManager *ingress.Manager) (Client, error) {

	newClient := lazyClient{
		Logger:          loggerInstance.GetChild("apigatewayres"),
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
	var appliedIngressNames []string

	apiGateway.Status.Name = apiGateway.Spec.Name

	if err := lc.validateSpec(apiGateway); err != nil {
		return nil, errors.Wrap(err, "Api gateway spec validation failed")
	}

	// generate an ingress for each upstream
	upstreams := apiGateway.Spec.Upstreams
	ingresses := map[string]*ingress.Resources{}

	// always try to remove previous canary ingress first, because
	// nginx returns 503 on all requests if primary service == secondary service. (happens on every promotion)
	// so during promotion all requests will be sent to the primary ingress
	lc.tryRemovePreviousCanaryIngress(ctx, apiGateway)

	if len(upstreams) == 1 {

		// create just a single ingress
		ingressResources, err := lc.generateNginxIngress(ctx, apiGateway, upstreams[0])
		if err != nil {
			return nil, errors.Wrap(err, "Failed to generate nginx ingress")
		}

		ingresses[ingressResources.Ingress.Name] = ingressResources

	} else if len(upstreams) == 2 {
		var canaryUpstream platform.APIGatewayUpstreamSpec
		var baseUpstream platform.APIGatewayUpstreamSpec

		// determine which upstream is the canary one
		if upstreams[0].Percentage != 0 {
			baseUpstream = upstreams[1]
			canaryUpstream = upstreams[0]
		} else if upstreams[1].Percentage != 0 {
			baseUpstream = upstreams[0]
			canaryUpstream = upstreams[1]
		} else {
			return nil, errors.New("Percentage must be set on one of the upstreams (canary)")
		}

		// validity check
		if canaryUpstream.Percentage > 100 || canaryUpstream.Percentage < 1 {
			return nil, errors.New("The canary upstream percentage must be between 1 and 100")
		}

		// add the base ingress
		baseIngressResources, err := lc.generateNginxIngress(ctx, apiGateway, baseUpstream)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to generate the base nginx ingress")
		}
		ingresses[baseIngressResources.Ingress.Name] = baseIngressResources

		// add the canary ingress
		canaryIngressResources, err := lc.generateNginxIngress(ctx, apiGateway, canaryUpstream)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to generate the canary nginx ingress")
		}
		ingresses[canaryIngressResources.Ingress.Name] = canaryIngressResources
	}

	// create ingresses
	// must be done synchronously, first primary and then canary
	// otherwise, when there is only canary ingress, the endpoint will not work (nginx behavior)
	for ingressName, ingressResources := range ingresses {
		if _, _, err := lc.ingressManager.CreateOrUpdateResources(ingressResources); err != nil {
			lc.Logger.WarnWithCtx(ctx, "Failed to create/update api-gateway ingress resources",
				"err", errors.Cause(err),
				"ingressName", ingressName,
				"appliedIngressNames", appliedIngressNames)
			return nil, errors.New("Failed to create/update api-gateway ingress resources")
		}

		appliedIngressNames = append(appliedIngressNames, ingressName)
	}

	return &lazyResources{
		ingressResourcesMap: ingresses,
	}, nil
}

func (lc *lazyClient) WaitAvailable(ctx context.Context, namespace string, name string) {
	lc.Logger.Debug("Sleeping for 4 seconds so nginx controller will stabilize")

	// sleep 4 seconds as a safety, so nginx will finish updating the ingresses properly (it takes time)
	time.Sleep(4 * time.Second)
}

func (lc *lazyClient) Delete(ctx context.Context, namespace string, name string) {
	lc.Logger.DebugWithCtx(ctx, "Deleting api-gateway base ingress", "name", name)

	err := lc.ingressManager.DeleteByName(lc.generateIngressName(name, false), namespace, true)
	if err != nil {
		lc.Logger.WarnWithCtx(ctx, "Failed to delete base ingress. Continuing with deletion",
			"err", errors.Cause(err))
	}

	lc.Logger.DebugWithCtx(ctx, "Deleting api-gateway canary ingress", "name", name)

	err = lc.ingressManager.DeleteByName(lc.generateIngressName(name, true), namespace, true)
	if err != nil {
		lc.Logger.WarnWithCtx(ctx, "Failed to delete canary ingress. Continuing with deletion",
			"err", errors.Cause(err))
	}
}

func (lc *lazyClient) tryRemovePreviousCanaryIngress(ctx context.Context, apiGateway *nuclioio.NuclioAPIGateway) {
	lc.Logger.DebugWithCtx(ctx, "Trying to remove previous canary ingress",
		"apiGatewayName", apiGateway.Name)

	// remove old canary ingress if it exists
	// this works thanks to an assumption that ingress names == api gateway name
	previousCanaryIngressName := lc.generateIngressName(apiGateway.Name, true)
	err := lc.ingressManager.DeleteByName(previousCanaryIngressName, apiGateway.Namespace, true)
	if err != nil {
		lc.Logger.WarnWithCtx(ctx,
			"Failed to delete previous canary ingress on api gateway update",
			"previousCanaryIngressName", previousCanaryIngressName,
			"err", errors.Cause(err))
	}
}

func (lc *lazyClient) validateSpec(apiGateway *nuclioio.NuclioAPIGateway) error {
	upstreams := apiGateway.Spec.Upstreams

	if len(upstreams) > 2 {
		return errors.New("Received more than 2 upstreams. Currently not supported")
	} else if len(upstreams) == 0 {
		return errors.New("One or more upstreams must be provided in spec")
	} else if apiGateway.Spec.Host == "" {
		return errors.New("Host must be provided in spec")
	}

	// TODO: update this when adding more upstream kinds. for now allow only `nucliofunction` upstreams
	kind := upstreams[0].Kind
	if kind != platform.APIGatewayUpstreamKindNuclioFunction {
		return errors.Errorf("Unsupported upstream kind: %s. (Currently supporting only nucliofunction)", kind)
	}

	if apiGateway.Name == "" {
		return errors.New("Api gateway name must be provided in spec")
	}

	// Validity checks per upstream
	// 1. make sure all upstreams have the same kind
	// 2. make sure each upstream is unique - meaning, there's no other api-gateway with an upstream with the
	//    same service (currently only nuclio function) name
	//    (this is done because creating multiple ingresses with the same service name breaks nginx ingress controller)
	existingUpstreamFunctionNames, err := lc.getAllExistingUpstreamFunctionNames(apiGateway.Namespace, apiGateway.Name)
	if err != nil {
		return errors.Wrap(err, "Failed while getting all existing upstreams")
	}
	for _, upstream := range upstreams {
		if upstream.Kind != kind {
			return errors.New("All upstreams must be of the same kind")
		}
		if common.StringSliceContainsString(existingUpstreamFunctionNames, upstream.Nucliofunction.Name) {
			return errors.Errorf("Nuclio function '%s' is already being used in another api-gateway",
				upstream.Nucliofunction.Name)
		}
	}

	return nil
}

func (lc *lazyClient) getAllExistingUpstreamFunctionNames(namespace, apiGatewayNameToExclude string) ([]string, error) {
	var existingUpstreamNames []string

	existingAPIGateways, err := lc.nuclioClientSet.NuclioV1beta1().
		NuclioAPIGateways(namespace).
		List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to list existing api gateways")
	}

	for _, apiGateway := range existingAPIGateways.Items {
		if apiGateway.Name == apiGatewayNameToExclude {
			continue
		}

		for _, upstream := range apiGateway.Spec.Upstreams {
			existingUpstreamNames = append(existingUpstreamNames, upstream.Nucliofunction.Name)
		}
	}

	return existingUpstreamNames, nil
}

func (lc *lazyClient) generateNginxIngress(ctx context.Context,
	apiGateway *nuclioio.NuclioAPIGateway,
	upstream platform.APIGatewayUpstreamSpec) (*ingress.Resources, error) {

	serviceName, servicePort, err := lc.getServiceNameAndPort(upstream, apiGateway.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get service name")
	}

	// add "/" as path prefix if not already there
	if !strings.HasPrefix(apiGateway.Spec.Path, "/") {
		apiGateway.Spec.Path = fmt.Sprintf("/%s", apiGateway.Spec.Path)
	}

	commonIngressSpec := ingress.Spec{
		Namespace:     apiGateway.Namespace,
		Host:          apiGateway.Spec.Host,
		Path:          apiGateway.Spec.Path,
		ServiceName:   serviceName,
		ServicePort:   servicePort,
		RewriteTarget: upstream.RewriteTarget,
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
				Name:     fmt.Sprintf("apigateway-%s", apiGateway.Name),
				Username: apiGateway.Spec.Authentication.BasicAuth.Username,
				Password: apiGateway.Spec.Authentication.BasicAuth.Password,
			},
		}
	case ingress.AuthenticationModeDex:
		if apiGateway.Spec.Authentication == nil || apiGateway.Spec.Authentication.DexAuth == nil {
			return nil, errors.New("Dex auth specified but missing dex auth spec")
		}
		commonIngressSpec.AuthenticationMode = ingress.AuthenticationModeDex
		commonIngressSpec.Authentication = &ingress.Authentication{
			DexAuth: &ingress.DexAuth{
				Oauth2ProxyURL: apiGateway.Spec.Authentication.DexAuth.Oauth2ProxyURL,
			},
		}
	case ingress.AuthenticationModeAccessKey:
		commonIngressSpec.AuthenticationMode = ingress.AuthenticationModeAccessKey
	default:
		return nil, errors.New("Unsupported ApiGateway authentication mode provided")
	}

	// add nginx specific annotations
	annotations := map[string]string{}
	annotations["kubernetes.io/ingress.class"] = "nginx"

	// if percentage is given, it is the canary upstream
	if upstream.Percentage != 0 {
		annotations["nginx.ingress.kubernetes.io/canary"] = "true"
		annotations["nginx.ingress.kubernetes.io/canary-weight"] = strconv.FormatInt(int64(upstream.Percentage), 10)
		commonIngressSpec.Name = lc.generateIngressName(apiGateway.Name, true)
	} else {
		commonIngressSpec.Name = lc.generateIngressName(apiGateway.Name, false)
	}

	commonIngressSpec.Annotations = annotations

	for annotationKey, annotationValue := range upstream.ExtraAnnotations {
		commonIngressSpec.Annotations[annotationKey] = annotationValue
	}

	return lc.ingressManager.GenerateResources(ctx, commonIngressSpec)
}

func (lc *lazyClient) getServiceNameAndPort(upstream platform.APIGatewayUpstreamSpec,
	namespace string) (string, int, error) {
	switch upstream.Kind {
	case platform.APIGatewayUpstreamKindNuclioFunction:
		return lc.getNuclioFunctionServiceNameAndPort(upstream, namespace)
	default:
		return "", 0, fmt.Errorf("Unsupported api gateway upstream kind: %s", upstream.Kind)
	}
}

func (lc *lazyClient) getNuclioFunctionServiceNameAndPort(upstream platform.APIGatewayUpstreamSpec,
	namespace string) (string, int, error) {

	// get the function's service
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("nuclio.io/function-name=%s", upstream.Nucliofunction.Name),
	}

	serviceList, err := lc.kubeClientSet.CoreV1().Services(namespace).List(listOptions)
	if err != nil {
		return "", 0, err
	}

	// there should be only one service for that label selector
	if len(serviceList.Items) != 1 {
		return "", 0, fmt.Errorf("Found unexpected number of services for function %s: %d",
			upstream.Nucliofunction.Name,
			len(serviceList.Items))
	}
	service := serviceList.Items[0]

	port, err := lc.getServiceHTTPPort(service)
	if err != nil {
		return "", 0, errors.Wrap(err, "Failed to get service's http port")
	}

	return service.Name, port, nil
}

func (lc *lazyClient) getServiceHTTPPort(service v1.Service) (int, error) {
	for _, portSpec := range service.Spec.Ports {
		if portSpec.Name == "http" {
			return int(portSpec.Port), nil
		}
	}

	return 0, errors.New("Service has no http port")
}

func (lc *lazyClient) generateIngressName(apiGatewayName string, canary bool) string {
	if canary {
		return fmt.Sprintf("apigateway-%s-canary", apiGatewayName)
	}

	return fmt.Sprintf("apigateway-%s", apiGatewayName)
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
