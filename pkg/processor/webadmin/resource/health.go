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

	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/restful"
)

type healthResource struct {
	*resource
}

func (esr *healthResource) GetSingle(request *http.Request) (string, restful.Attributes) {
	return "processor", restful.Attributes{
		"oper_status": "up",
	}
}

// register the resource
var health = &healthResource{
	resource: newResource("health", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
	}),
}

func init() {
	health.Resource = health
	health.Register(webadmin.WebAdminResourceRegistrySingleton)
}
