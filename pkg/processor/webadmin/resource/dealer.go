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
	"fmt"
	"net/http"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/util"
	"github.com/nuclio/nuclio/pkg/processor/webadmin"
	"github.com/nuclio/nuclio/pkg/restful"
)

type dealerResource struct {
	*resource
}

// DealerRequest is dealer request
type DealerRequest struct {
	Name string `json:"name"`
	Jobs map[string]struct {
		Tasks []struct {
			ID    int   `json:"id"`
			State int64 `json:"state"`
		} `json:"tasks"`
	} `json:"jobs"`
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

func (dr *dealerResource) GetAll(request *http.Request) (map[string]restful.Attributes, error) {
	response := dr.responseFromConfiguration(dr.getProcessor().GetConfiguration())
	return response, nil
}

func (dr *dealerResource) setRoutes(request *http.Request) (*restful.CustomRouteFuncResponse, error) {

	dealerRequest := DealerRequest{}
	decoder := json.NewDecoder(request.Body)
	if err := decoder.Decode(&dealerRequest); err != nil {
		return &restful.CustomRouteFuncResponse{}, errors.Wrap(err, "Can't decode request")
	}

	processor := dr.getProcessor()
	processorConfigCopy := util.CopyConfiguration(processor.GetConfiguration())

	for jobID, job := range dealerRequest.Jobs {
		triggerConfig, found := processorConfigCopy.Spec.Triggers[jobID]
		if !found {
			// TODO: How can I return both error and status code in restful?
			return &restful.CustomRouteFuncResponse{
				StatusCode: http.StatusBadRequest,
			}, nil
		}

		triggerConfig.Partitions = make([]functionconfig.Partition, 0, len(job.Tasks))
		for _, task := range job.Tasks {
			checkpoint := fmt.Sprintf("%d", task.State)
			partition := functionconfig.Partition{
				ID:         fmt.Sprintf("%d", task.ID),
				Checkpoint: &checkpoint,
			}
			triggerConfig.Partitions = append(triggerConfig.Partitions, partition)
		}
		processorConfigCopy.Spec.Triggers[jobID] = triggerConfig
	}

	if err := processor.SetConfiguration(processorConfigCopy); err != nil {
		return &restful.CustomRouteFuncResponse{
			StatusCode: http.StatusBadRequest,
		}, nil
	}

	configuration := dr.getProcessor().GetLastUpdate().GetConfiguration()
	response := dr.responseFromConfiguration(configuration)

	return &restful.CustomRouteFuncResponse{
		StatusCode: http.StatusCreated,
		Resources:  response,
	}, nil
}

func (dr *dealerResource) responseFromConfiguration(configuration *processor.Configuration) map[string]restful.Attributes {
	response := make(map[string]restful.Attributes)

	for triggerID, trigger := range configuration.Spec.Triggers {
		response[triggerID] = restful.Attributes{"tasks": trigger.Partitions}

	}

	return response
}

func (dr *dealerResource) findTrigger(id string) trigger.Trigger {
	for _, trigger := range dr.getProcessor().GetTriggers() {
		if trigger.GetID() == id {
			return trigger
		}
	}

	return nil
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
