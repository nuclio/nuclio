/*
Copyright 2018 The Nuclio Authors.

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

package kafka

import (
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"

	"github.com/Shopify/sarama"
	"github.com/mitchellh/mapstructure"
)

type workerAllocationMode string

const (
	workerAllocationModePool   workerAllocationMode = "pool"
	workerAllocationModeStatic workerAllocationMode = "static"
)

type Configuration struct {
	trigger.Configuration
	Brokers       []string
	brokers       []string
	Topics        []string
	ConsumerGroup string
	InitialOffset string
	SASL          struct {
		Enable   bool
		User     string
		Password string
	}

	SessionTimeout       string
	HearbeatInterval     string
	MaxProcessingTime    string
	WorkerAllocationMode workerAllocationMode

	sessionTimeout    time.Duration
	heartbeatInterval time.Duration
	maxProcessingTime time.Duration
	initialOffset     int64
}

func NewConfiguration(ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)

	workerAllocationModeValue := ""

	err := newConfiguration.PopulateConfigurationFromAnnotations([]trigger.AnnotationConfigField{
		{"nuclio.io/kafka-session-timeout", &newConfiguration.SessionTimeout},
		{"nuclio.io/kafka-heartbeat-interval", &newConfiguration.HearbeatInterval},
		{"nuclio.io/kafka-max-processing-time", &newConfiguration.MaxProcessingTime},
		{"nuclio.io/kafka-worker-allocation-mode", &workerAllocationModeValue},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to populate configuration from annotations")
	}

	newConfiguration.WorkerAllocationMode = workerAllocationMode(workerAllocationModeValue)

	// set default
	if triggerConfiguration.MaxWorkers == 0 {
		triggerConfiguration.MaxWorkers = 32
	}

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if len(newConfiguration.Topics) == 0 {
		return nil, errors.New("Topics must be set")
	}

	if newConfiguration.ConsumerGroup == "" {
		return nil, errors.New("Consumer group must be set")
	}

	newConfiguration.initialOffset, err = newConfiguration.resolveInitialOffset(newConfiguration.InitialOffset)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve initial offset")
	}

	newConfiguration.brokers, err = newConfiguration.resolveBrokers(newConfiguration.Brokers)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to resolve brokers")
	}

	for _, durationConfigField := range []trigger.DurationConfigField{
		{"session timeout", newConfiguration.SessionTimeout, &newConfiguration.sessionTimeout, 60 * time.Second},
		{"heartbeat interval", newConfiguration.HearbeatInterval, &newConfiguration.heartbeatInterval, 5 * time.Second},
		{"max processing timeout", newConfiguration.MaxProcessingTime, &newConfiguration.maxProcessingTime, 5 * time.Minute},
	} {
		if err = newConfiguration.ParseDurationOrDefault(&durationConfigField); err != nil {
			return nil, err
		}
	}

	if newConfiguration.WorkerAllocationMode == "" {
		newConfiguration.WorkerAllocationMode = workerAllocationModePool
	}

	return &newConfiguration, nil
}

func (c *Configuration) resolveInitialOffset(initialOffset string) (int64, error) {
	if initialOffset == "" {
		return sarama.OffsetNewest, nil
	}
	if lower := strings.ToLower(initialOffset); lower == "earliest" {
		return sarama.OffsetOldest, nil
	} else if lower == "latest" {
		return sarama.OffsetNewest, nil
	} else {
		return 0, errors.Errorf("InitialOffset must be either 'earliest' or 'latest', not '%s'", initialOffset)
	}
}

func (c *Configuration) resolveBrokers(brokers []string) ([]string, error) {
	if len(brokers) > 0 {
		return brokers, nil
	}

	if c.URL != "" {
		return []string{c.URL}, nil
	}

	return nil, errors.New("Brokers must be passed either in url or attributes.brokers")
}
