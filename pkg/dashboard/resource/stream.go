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
	"github.com/nuclio/nuclio/pkg/platform/abstract/project/external/leader/iguazio"
	"github.com/nuclio/nuclio/pkg/platformconfig"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/nuclio/errors"
	"github.com/nuclio/nuclio-sdk-go"
)

type streamResource struct {
	*resource
}

func (sr *streamResource) ExtendMiddlewares() error {
	sr.resource.addAuthMiddleware()
	return nil
}

func (sr *streamResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {

	// ensure namespace
	namespace := sr.getNamespaceFromRequest(request)
	if namespace == "" {
		return nil, nuclio.NewErrBadRequest("Namespace must exist")
	}

	// ensure project name
	projectName := request.Header.Get("x-nuclio-project-name")
	if projectName == "" {
		return nil, nuclio.NewErrBadRequest("Project name must not be empty")
	}

	// get project
	project, err := sr.getProjectByName(request, projectName, namespace)
	if err != nil {
		return nil, err
	}

	// get project functions
	ctx := request.Context()
	getFunctionsOptions := &platform.GetFunctionsOptions{
		Name:      "",
		Namespace: project.GetConfig().Meta.Namespace,
		Labels: fmt.Sprintf("%s=%s",
			common.NuclioResourceLabelKeyProjectName,
			project.GetConfig().Meta.Name),
		AuthSession: sr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(sr.getCtxSession(request)),
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
	}

	//functionsMap, _ := pr.getFunctionsAndFunctionEventsMap(request, project)
	functions, err := sr.getPlatform().GetFunctions(ctx, getFunctionsOptions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting project functions")
	}

	// iterate over functions and look for v3iostreams
	streams, err := sr.getStreamsFromFunctions(functions)
	if err != nil {
		return nil, errors.Wrap(err, "Failed getting streams from functions")
	}

	return streams, nil
}

func (sr *streamResource) getStreamsFromFunctions(functions []platform.Function) (map[string]restful.Attributes, error) {

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

func (sr *streamResource) getNamespaceFromRequest(request *http.Request) string {
	return sr.getNamespaceOrDefault(request.Header.Get("x-nuclio-project-namespace"))
}

func (sr *streamResource) getProjectByName(request *http.Request, projectName, projectNamespace string) (platform.Project, error) {
	ctx := request.Context()

	requestOrigin, sessionCookie := sr.getRequestOriginAndSessionCookie(request)
	projects, err := sr.getPlatform().GetProjects(ctx, &platform.GetProjectsOptions{
		Meta: platform.ProjectMeta{
			Name:      projectName,
			Namespace: projectNamespace,
		},
		AuthSession: sr.getCtxSession(request),
		PermissionOptions: opa.PermissionOptions{
			MemberIds:           opa.GetUserAndGroupIdsFromAuthSession(sr.getCtxSession(request)),
			RaiseForbidden:      true,
			OverrideHeaderValue: request.Header.Get(opa.OverrideHeader),
		},
		RequestOrigin: requestOrigin,
		SessionCookie: sessionCookie,
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get projects")
	}

	if len(projects) == 0 {
		return nil, nuclio.NewErrNotFound("Project not found")
	}
	return projects[0], nil
}

func (sr *streamResource) getRequestOriginAndSessionCookie(request *http.Request) (platformconfig.ProjectsLeaderKind, *http.Cookie) {
	requestOrigin := platformconfig.ProjectsLeaderKind(request.Header.Get(iguazio.ProjectsRoleHeaderKey))

	// ignore error here, and just return a nil cookie when no session was passed (relevant only on leader/follower mode)
	sessionCookie, _ := request.Cookie("session")

	return requestOrigin, sessionCookie
}

// register the resource
var streamResourceInstance = &streamResource{
	resource: newResource("api/streams", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	streamResourceInstance.Resource = streamResourceInstance
	streamResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
