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

package resource

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/restful"

	"k8s.io/api/core/v1"
)

type frontendSpecResource struct {
	*resource
}

func (fsr *frontendSpecResource) getFrontendSpec(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	externalIPAddresses, err := fsr.getPlatform().GetExternalIPAddresses()
	if err != nil {
		externalIPAddresses = []string{"localhost"}
		fsr.Logger.WarnWith("Failed to get external IP addresses, falling back to default",
			"err", err,
			"externalIPAddresses", externalIPAddresses)
	}

	// try to get platform kind
	platformKind := ""
	if dashboardServer, ok := fsr.resource.GetServer().(*dashboard.Server); ok {
		platformConfiguration := dashboardServer.GetPlatformConfiguration()
		if platformConfiguration != nil {
			platformKind = platformConfiguration.Kind
		}
	}

	scaleToZeroConfiguration := fsr.getPlatform().GetScaleToZeroConfiguration()

	allowedAuthenticationModes := fsr.getPlatform().GetAllowedAuthenticationModes()

	scaleToZeroMode := platformconfig.DisabledScaleToZeroMode
	var inactivityWindowPresets []string
	var scaleResources []functionconfig.ScaleResource

	if scaleToZeroConfiguration != nil {
		scaleToZeroMode = scaleToZeroConfiguration.Mode
		inactivityWindowPresets = scaleToZeroConfiguration.InactivityWindowPresets
		scaleResources = scaleToZeroConfiguration.ScaleResources
	}

	scaleToZeroAttribute := map[string]interface{}{
		"mode":                    scaleToZeroMode,
		"inactivityWindowPresets": inactivityWindowPresets,
		"scaleResources":          scaleResources,
	}

	defaultFunctionConfig := fsr.getDefaultFunctionConfig()
	defaultHTTPIngressHostTemplate := fsr.getDefaultHTTPIngressHostTemplate()

	frontendSpec := map[string]restful.Attributes{
		"frontendSpec": { // frontendSpec is the ID of this singleton resource
			"externalIPAddresses":            externalIPAddresses,
			"namespace":                      fsr.getNamespaceOrDefault(""),
			"defaultHTTPIngressHostTemplate": defaultHTTPIngressHostTemplate,
			"imageNamePrefixTemplate":        fsr.getPlatform().GetImageNamePrefixTemplate(),
			"scaleToZero":                    scaleToZeroAttribute,
			"defaultFunctionConfig":          defaultFunctionConfig,
			"platformKind":                   platformKind,
			"allowedAuthenticationModes":     allowedAuthenticationModes,
		},
	}

	return &restful.CustomRouteFuncResponse{
		Single:     true,
		StatusCode: http.StatusOK,
		Resources:  frontendSpec,
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func (fsr *frontendSpecResource) getDefaultFunctionConfig() map[string]interface{} {
	one := 1
	defaultWorkerAvailabilityTimeoutMilliseconds := trigger.DefaultWorkerAvailabilityTimeoutMilliseconds

	defaultFunctionNodeSelector := fsr.resolveDefaultFunctionNodeSelector()
	defaultServiceType := fsr.resolveDefaultServiceType()
	defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
	defaultHTTPTrigger.WorkerAvailabilityTimeoutMilliseconds = &defaultWorkerAvailabilityTimeoutMilliseconds
	defaultHTTPTrigger.Attributes = map[string]interface{}{
		"serviceType": defaultServiceType,
	}

	defaultFunctionSpec := functionconfig.Spec{
		MinReplicas:             &one,
		MaxReplicas:             &one,
		ReadinessTimeoutSeconds: fsr.resolveFunctionReadinessTimeoutSeconds(),
		NodeSelector:            defaultFunctionNodeSelector,
		TargetCPU:               abstract.DefaultTargetCPU,
		Triggers: map[string]functionconfig.Trigger{

			// this trigger name starts with the prefix "default" and should be used as a default http trigger
			// as opposed to the other defaults which only hold configurations for the creation of every other trigger.
			defaultHTTPTrigger.Name: defaultHTTPTrigger,

			// notice that this is a mapping between trigger kind and its default values
			"http": {
				WorkerAvailabilityTimeoutMilliseconds: &defaultWorkerAvailabilityTimeoutMilliseconds,
				Attributes: map[string]interface{}{
					"serviceType": defaultServiceType,
				},
			},
			"cron": {
				WorkerAvailabilityTimeoutMilliseconds: &defaultWorkerAvailabilityTimeoutMilliseconds,
			},
		},
	}

	return map[string]interface{}{"attributes": functionconfig.Config{Spec: defaultFunctionSpec}}
}

// returns a list of custom routes for the resource
func (fsr *frontendSpecResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since frontendSpec is a singleton we create a custom route that will return this single object
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodGet,
			RouteFunc: fsr.getFrontendSpec,
		},
	}, nil
}

func (fsr *frontendSpecResource) resolveDefaultServiceType() v1.ServiceType {
	var defaultServiceType v1.ServiceType
	if dashboardServer, ok := fsr.resource.GetServer().(*dashboard.Server); ok {
		defaultServiceType = dashboardServer.GetPlatformConfiguration().Kube.DefaultServiceType
	}
	return defaultServiceType
}

func (fsr *frontendSpecResource) resolveFunctionReadinessTimeoutSeconds() int {
	readinessTimeoutSeconds := platformconfig.DefaultFunctionReadinessTimeoutSeconds
	if dashboardServer, ok := fsr.resource.GetServer().(*dashboard.Server); ok {
		return int(dashboardServer.GetPlatformConfiguration().GetDefaultFunctionReadinessTimeout().Seconds())
	}
	return readinessTimeoutSeconds
}

func (fsr *frontendSpecResource) resolveDefaultFunctionNodeSelector() map[string]string {
	var defaultNodeSelector map[string]string
	if dashboardServer, ok := fsr.resource.GetServer().(*dashboard.Server); ok {
		defaultNodeSelector = dashboardServer.GetPlatformConfiguration().Kube.DefaultFunctionNodeSelector
	}
	return defaultNodeSelector
}

func (fsr *frontendSpecResource) getDefaultHTTPIngressHostTemplate() string {

	// try read from platform configuration first, if set use that, otherwise
	// fallback reading from envvar for backwards compatibility with old helm charts
	if dashboardServer, ok := fsr.resource.GetServer().(*dashboard.Server); ok {
		defaultHTTPIngressHostTemplate := dashboardServer.GetPlatformConfiguration().Kube.DefaultHTTPIngressHostTemplate
		if defaultHTTPIngressHostTemplate != "" {
			return defaultHTTPIngressHostTemplate
		}
	}

	return common.GetEnvOrDefaultString(
		"NUCLIO_DASHBOARD_HTTP_INGRESS_HOST_TEMPLATE", "")
}

// register the resource
var frontendSpecResourceInstance = &frontendSpecResource{
	resource: newResource("api/frontend_spec", []restful.ResourceMethod{}),
}

func init() {
	frontendSpecResourceInstance.Resource = frontendSpecResourceInstance
	frontendSpecResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
