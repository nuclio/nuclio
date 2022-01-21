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

package trigger

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/registry"

	"github.com/nuclio/logger"
)

// Creator creates a trigger instance
type Creator interface {

	// Create creates a trigger instance
	Create(logger.Logger, string, *functionconfig.Trigger, *runtime.Configuration, *worker.AllocatorSyncMap) (Trigger, error)
}

type Registry struct {
	registry.Registry
}

// RegistrySingleton is a trigger global singleton
var RegistrySingleton = Registry{
	Registry: *registry.NewRegistry("trigger"),
}

func (r *Registry) NewTrigger(logger logger.Logger,
	kind string,
	name string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration,
	namedWorkerAllocators *worker.AllocatorSyncMap) (Trigger, error) {

	registree, err := r.Get(kind)
	if err != nil {
		return nil, err
	}

	triggerConfiguration.Name = name

	// if there's no worker allocator - the runtime belongs to a single trigger. if there is,
	// it belongs to multiple workers and therefore pass the worker allocator name
	if triggerConfiguration.WorkerAllocatorName == "" {
		runtimeConfiguration.TriggerKind = kind
		runtimeConfiguration.TriggerName = name
	} else {
		runtimeConfiguration.TriggerKind = ""
		runtimeConfiguration.TriggerName = triggerConfiguration.WorkerAllocatorName
	}

	return registree.(Creator).Create(logger,
		name,
		triggerConfiguration,
		runtimeConfiguration,
		namedWorkerAllocators)
}
