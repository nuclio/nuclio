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

package kinesis

import (
	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/nuclio/logger"
	"github.com/vmware/vmware-go-kcl/clientlibrary/config"
	"github.com/vmware/vmware-go-kcl/clientlibrary/metrics"
	kclworker "github.com/vmware/vmware-go-kcl/clientlibrary/worker"
)

type kinesis struct {
	trigger.AbstractTrigger
	event         Event
	configuration *Configuration
	kinesisConfig *config.KinesisClientLibConfiguration
	kclWorkers    []*kclworker.Worker
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	instanceLogger := parentLogger.GetChild(configuration.ID)

	abstractTrigger, err := trigger.NewAbstractTrigger(instanceLogger,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"kinesis")
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger := &kinesis{
		AbstractTrigger: abstractTrigger,
		configuration:   configuration,
	}
	creds := credentials.NewCredentials(&credentials.StaticProvider{Value: credentials.Value{
		AccessKeyID:     configuration.AccessKeyID,
		SecretAccessKey: configuration.SecretAccessKey,
	}})

	newTrigger.kinesisConfig = config.NewKinesisClientLibConfigWithCredential(configuration.ApplicationName, configuration.StreamName, configuration.RegionName, "", creds)

	for i := 0; i < configuration.MaxWorkers; i++ {
		workerInstance, err := newTrigger.WorkerAllocator.Allocate(0)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to allocate worker #%d", i)
		}
		rpf := recordProcessorFactory{
			Logger:         instanceLogger.GetChild("recordProcessor"),
			worker:         workerInstance,
			kinesisTrigger: newTrigger,
		}
		kclWorkerInstance := kclworker.NewWorker(&rpf, newTrigger.kinesisConfig, &metrics.MonitoringConfiguration{})
		newTrigger.kclWorkers = append(newTrigger.kclWorkers, kclWorkerInstance)
	}

	return newTrigger, nil
}

func (k *kinesis) Start(checkpoint functionconfig.Checkpoint) error {
	k.Logger.InfoWith("Starting",
		"streamName", k.configuration.StreamName,
		"applicationName", k.configuration.ApplicationName)

	for _, workerInstance := range k.kclWorkers {
		if err := workerInstance.Start(); err != nil {
			k.Logger.ErrorWith("Failed to read from shard", "err", err)
			return err
		}
	}

	return nil
}

func (k *kinesis) Stop(force bool) (functionconfig.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (k *kinesis) GetConfig() map[string]interface{} {
	return common.StructureToMap(k.configuration)
}
