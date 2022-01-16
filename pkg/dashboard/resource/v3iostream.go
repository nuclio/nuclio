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
	"fmt"
	"net/http"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/opa"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type v3ioStreamResource struct {
	*resource
}

func (vsr *v3ioStreamResource) ExtendMiddlewares() error {
	vsr.resource.addAuthMiddleware()
	return nil
}

func (vsr *v3ioStreamResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {

	functions, err := vsr.getFunctions(request)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting project functions")
	}

	// iterate over functions and look for v3iostreams
	streams, err := vsr.getStreamsFromFunctions(functions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting streams from functions")
	}

	return streams, nil
}

func (vsr *v3ioStreamResource) getFunctions(request *http.Request) ([]platform.Function, error) {

	// ensure namespace
	namespace := vsr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	// ensure project name
	projectName := request.Header.Get("x-nuclio-project-name")
	if projectName == "" {
		return nil, nuclio.NewErrBadRequest("Project name must not be empty")
	}

	// get project functions
	ctx := request.Context()
	getFunctionsOptions := &platform.GetFunctionsOptions{
		Name:      "",
		Namespace: namespace,
		Labels: fmt.Sprintf("%s=%s",
			common.NuclioResourceLabelKeyProjectName,
			projectName),
		AuthSession: vsr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(vsr.getCtxSession(request)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}

	return vsr.getPlatform().GetFunctions(ctx, getFunctionsOptions)
}

func (vsr *v3ioStreamResource) getStreamsFromFunctions(functions []platform.Function) (map[string]restful.Attributes, error) {

	streamsMap := map[string]restful.Attributes{}

	for _, function := range functions {
		v3ioStreamsMap := functionconfig.GetTriggersByKind(function.GetConfig().Spec.Triggers, "v3ioStream")
		for streamName, stream := range v3ioStreamsMap {

			// add stream to map, with a key in the format: "function-name@stream-name"
			keyName := fmt.Sprintf("%s@%s", function.GetConfig().Meta.Name, streamName)
			streamsMap[keyName] = restful.Attributes{
				"consumerGroup": stream.Attributes["consumerGroup"],
				"containerName": stream.Attributes["containerName"],
				"streamPath":    stream.Attributes["streamPath"],
			}
		}
	}

	return streamsMap, nil
}

func (vsr *v3ioStreamResource) getNamespaceFromRequest(request *http.Request) string {
	return vsr.getNamespaceOrDefault(request.Header.Get("x-nuclio-project-namespace"))
}

// register the resource
var v3ioStreamResourceInstance = &v3ioStreamResource{
	resource: newResource("api/v3io_streams", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	v3ioStreamResourceInstance.Resource = v3ioStreamResourceInstance
	v3ioStreamResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
