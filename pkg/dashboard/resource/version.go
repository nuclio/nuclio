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
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/restful"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/nuclio/nuclio-sdk-go"
)

type versionResource struct {
	*resource
}

// GetAll returns all versions
func (vr *versionResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	versionInfo, err := version.Get()
	if err != nil {
		return nil, nuclio.WrapErrInternalServerError(errors.Wrap(err, "Failed to get version"))
	}

	response := map[string]restful.Attributes{
		"dashboard": {
			"label":     versionInfo.Label,
			"gitCommit": versionInfo.GitCommit,
			"os":        versionInfo.OS,
			"arch":      versionInfo.Arch,
		},
	}

	return response, nil
}

// register the resource
var versionResourceInstance = &versionResource{
	resource: newResource("api/versions", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	versionResourceInstance.Resource = versionResourceInstance
	versionResourceInstance.Register(dashboard.DashboardResourceRegistrySingleton)
}
