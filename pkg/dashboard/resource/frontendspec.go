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
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
)

type frontendSpecResource struct {
	*resource
}

func (fesr *frontendSpecResource) getFrontendSpec(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	externalIPAddresses, err := fesr.getPlatform().GetExternalIPAddresses()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get external IP addresses")
	}

	scaleToZeroConfiguration, err := fesr.getPlatform().GetScaleToZeroConfiguration()
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting scale to zero configuration")
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
	defaultWorkerAvailabilityTimeoutMilliseconds := trigger.DefaultWorkerAvailabilityTimeoutMilliseconds
	defaultFunctionSpec := functionconfig.Spec{
		MinReplicas:             &one,
		MaxReplicas:             &one,
		ReadinessTimeoutSeconds: abstract.DefaultReadinessTimeoutSeconds,
		TargetCPU:               abstract.DefaultTargetCPU,
		Triggers: map[string]functionconfig.Trigger{

			// notice that this is a mapping between trigger kind and its default values
			"http": {
				WorkerAvailabilityTimeoutMilliseconds: &defaultWorkerAvailabilityTimeoutMilliseconds,
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

// register the resource
var frontendSpecResourceInstance = &frontendSpecResource{
	resource: newResource("api/frontend_spec", []restful.ResourceMethod{}),
}

func init() {
	frontendSpecResourceInstance.Resource = frontendSpecResourceInstance
	frontendSpecResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
