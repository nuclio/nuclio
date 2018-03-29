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
	"encoding/json"
	"net/http"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/restful"
)

type dealerResource struct {
	*resource
}

// Request is dealer request
type Request struct {
	Name string `json:"name"`
	Jobs map[string]struct {
		Tasks []struct {
			ID    int   `json:"id"`
			State int64 `json:"state"`
		} `json:"tasks"`
	} `json:"jobs"`
}

func (dr *dealerResource) findTrigger(id string) trigger.Trigger {
	for _, trigger := range dr.getProcessor().GetTriggers() {
		dr.Logger.InfoWith("trigger", "id", trigger.GetID())
		if trigger.GetID() == id {
			return trigger
		}
	}

	return nil
}

func (dr *dealerResource) Update(request *http.Request, id string) (restful.Attributes, error) {
	trigger := dr.findTrigger(id)

	if trigger == nil {
		dr.Logger.WarnWith("Can't find trigger", "id", id)
		return nil, errors.Errorf("Can't find trigger with id %q", id)
	}

	return restful.Attributes{
		"ok": true,
	}, nil
}

func (dr *dealerResource) GetCustomRoutes() ([]restful.CustomRoute, error) {

	// just for demonstration. when stats are supported, this will be wired
	return []restful.CustomRoute{
		{
			Pattern:   "/",
			Method:    http.MethodPost,
			RouteFunc: dr.setRoutes,
		},
	}, nil
}

func (dr *dealerResource) setRoutes(request *http.Request) (*restful.CustomRouteFuncResponse, error) {
	defer request.Body.Close()

	dealerRequest := Request{}
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&dealerRequest); err != nil {
		return &restful.CustomRouteFuncResponse{}, errors.Wrap(err, "Can't decode request")
	}

	return &restful.CustomRouteFuncResponse{
		StatusCode: http.StatusCreated,
	}, nil
}

func (dr *dealerResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	response := make(map[string]restful.Attributes)
	for _, trigger := range dr.getProcessor().GetTriggers() {
		dr.Logger.InfoWith("trigger", "id", trigger.GetID())
		rawAttributes := trigger.GetConfig()["attributes"]
		attributes, ok := rawAttributes.(map[string]interface{})
		var tasks interface{}
		if ok {
			tasks = attributes["partitions"]
		} else {
			// TODO: Log?
			dr.Logger.WarnWith("bad attributes", "id", trigger.GetID())
			tasks = make([]int, 0)
		}

		response[trigger.GetID()] = restful.Attributes{"tasks": tasks}
	}

	return response, nil
}

func init() {
	dealer := &dealerResource{
		resource: newResource("dealer", []restful.ResourceMethod{
			restful.ResourceMethodGetList,
			restful.ResourceMethodUpdate,
		}),
	}

	dealer.Resource = dealer
	dealer.Register(webadmin.WebAdminResourceRegistrySingleton)
}
