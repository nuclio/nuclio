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

	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/platform/abstract"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	triggercommon "github.com/nuclio/nuclio/pkg/processor/trigger/common"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"k8s.io/api/core/v1"
)

type frontendSpecResource struct {
	*resource
}

func (fesr *frontendSpecResource) getFrontendSpec(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	externalIPAddresses, err := fesr.getPlatform().GetExternalIPAddresses()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get external IP addresses")
	}

	// try to get platform kind
	platformKind := ""
	if dashboardServer, ok := fesr.resource.GetServer().(*dashboard.Server); ok {
		platformConfiguration := dashboardServer.GetPlatformConfiguration()
		if platformConfiguration != nil {
			platformKind = platformConfiguration.Kind
		}
	}

	scaleToZeroConfiguration, err := fesr.getPlatform().GetScaleToZeroConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting scale to zero configuration")
	}

	allowedAuthenticationModes, err := fesr.getPlatform().GetAllowedAuthenticationModes()
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting allowed authentication modes")
	}

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

	defaultFunctionConfig := fesr.getDefaultFunctionConfig()

	frontendSpec := map[string]restful.Attributes{
		"frontendSpec": { // frontendSpec is the ID of this singleton resource
			"externalIPAddresses":            externalIPAddresses,
			"namespace":                      fesr.getNamespaceOrDefault(""),
			"defaultHTTPIngressHostTemplate": fesr.getPlatform().GetDefaultHTTPIngressHostTemplate(),
			"imageNamePrefixTemplate":        fesr.getPlatform().GetImageNamePrefixTemplate(),
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

func (fesr *frontendSpecResource) getDefaultFunctionConfig() map[string]interface{} {
	one := 1
	defaultWorkerAvailabilityTimeoutMilliseconds := triggercommon.DefaultWorkerAvailabilityTimeoutMilliseconds

	defaultServiceType := fesr.resolveDefaultServiceType()
	defaultHTTPTrigger := functionconfig.GetDefaultHTTPTrigger()
	defaultHTTPTrigger.WorkerAvailabilityTimeoutMilliseconds = &defaultWorkerAvailabilityTimeoutMilliseconds
	defaultHTTPTrigger.Attributes = map[string]interface{}{
		"serviceType": defaultServiceType,
	}

	defaultFunctionSpec := functionconfig.Spec{
		MinReplicas:             &one,
		MaxReplicas:             &one,
		ReadinessTimeoutSeconds: abstract.DefaultReadinessTimeoutSeconds,
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
func (fesr *frontendSpecResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// since frontendSpec is a singleton we create a custom route that will return this single object
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodGet,
			RouteFunc: fesr.getFrontendSpec,
		},
	}, nil
}

func (fesr *frontendSpecResource) resolveDefaultServiceType() v1.ServiceType {
	var defaultServiceType v1.ServiceType = ""
	if dashboardServer, ok := fesr.resource.GetServer().(*dashboard.Server); ok {
		defaultServiceType = dashboardServer.GetPlatformConfiguration().Kube.DefaultServiceType
	}
	return defaultServiceType
}

// register the resource
var frontendSpecResourceInstance = &frontendSpecResource{
	resource: newResource("api/frontend_spec", []restful.ResourceMethod{}),
}

func init() {
	frontendSpecResourceInstance.Resource = frontendSpecResourceInstance
	frontendSpecResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
