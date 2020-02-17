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

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"

	"github.com/Shopify/sarama"
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
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

	SessionTimeout                string
	HearbeatInterval              string
	MaxProcessingTime             string
	RebalanceTimeout              string
	RebalanceRetryBackoff         string
	RetryBackoff                  string
	MaxWaitTime                   string
	MaxWaitHandlerDuringRebalance string
	WorkerAllocationMode          partitionworker.AllocationMode
	RebalanceRetryMax             int
	FetchMin                      int
	FetchDefault                  int
	FetchMax                      int
	ChannelBufferSize             int

	sessionTimeout                time.Duration
	heartbeatInterval             time.Duration
	maxProcessingTime             time.Duration
	rebalanceTimeout              time.Duration
	rebalanceRetryBackoff         time.Duration
	retryBackoff                  time.Duration
	maxWaitTime                   time.Duration
	maxWaitHandlerDuringRebalance time.Duration
	initialOffset                 int64
}

func NewConfiguration(ID string,
	triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(ID, triggerConfiguration, runtimeConfiguration)

	workerAllocationModeValue := ""

	err := newConfiguration.PopulateConfigurationFromAnnotations([]trigger.AnnotationConfigField{
		{Key: "nuclio.io/kafka-session-timeout", ValueString: &newConfiguration.SessionTimeout},
		{Key: "nuclio.io/kafka-heartbeat-interval", ValueString: &newConfiguration.HearbeatInterval},
		{Key: "nuclio.io/kafka-max-processing-time", ValueString: &newConfiguration.MaxProcessingTime},
		{Key: "nuclio.io/kafka-rebalance-timeout", ValueString: &newConfiguration.RebalanceTimeout},
		{Key: "nuclio.io/kafka-rebalance-retry-backoff", ValueString: &newConfiguration.RebalanceRetryBackoff},
		{Key: "nuclio.io/kafka-retry-backoff", ValueString: &newConfiguration.RetryBackoff},
		{Key: "nuclio.io/kafka-max-wait-time", ValueString: &newConfiguration.MaxWaitTime},
		{Key: "nuclio.io/kafka-max-wait-handler-during-rebalance", ValueString: &newConfiguration.MaxWaitHandlerDuringRebalance},
		{Key: "nuclio.io/kafka-worker-allocation-mode", ValueString: &workerAllocationModeValue},
		{Key: "nuclio.io/kafka-rebalance-retry-max", ValueInt: &newConfiguration.RebalanceRetryMax},
		{Key: "nuclio.io/kafka-fetch-min", ValueInt: &newConfiguration.FetchMin},
		{Key: "nuclio.io/kafka-fetch-default", ValueInt: &newConfiguration.FetchDefault},
		{Key: "nuclio.io/kafka-fetch-max", ValueInt: &newConfiguration.FetchMax},
		{Key: "nuclio.io/kafka-channel-buffer-size", ValueInt: &newConfiguration.ChannelBufferSize},
	})

	if err != nil {
		return nil, errors.Wrap(err, "Failed to populate configuration from annotations")
	}

	newConfiguration.WorkerAllocationMode = partitionworker.AllocationMode(workerAllocationModeValue)

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
		{"session timeout", newConfiguration.SessionTimeout, &newConfiguration.sessionTimeout, 10 * time.Second},
		{"heartbeat interval", newConfiguration.HearbeatInterval, &newConfiguration.heartbeatInterval, 3 * time.Second},
		{"max processing timeout", newConfiguration.MaxProcessingTime, &newConfiguration.maxProcessingTime, 5 * time.Minute},
		{"rebalance timeout", newConfiguration.RebalanceTimeout, &newConfiguration.rebalanceTimeout, 60 * time.Second},
		{"rebalance retry backoff", newConfiguration.RebalanceRetryBackoff, &newConfiguration.rebalanceRetryBackoff, 2 * time.Second},
		{"retry backoff", newConfiguration.RetryBackoff, &newConfiguration.retryBackoff, 2 * time.Second},
		{"max wait time", newConfiguration.MaxWaitTime, &newConfiguration.maxWaitTime, 250 * time.Millisecond},
		{"max wait handler during rebalance", newConfiguration.MaxWaitHandlerDuringRebalance, &newConfiguration.maxWaitHandlerDuringRebalance, 5 * time.Second},
	} {
		if err = newConfiguration.ParseDurationOrDefault(&durationConfigField); err != nil {
			return nil, err
		}
	}

	if newConfiguration.WorkerAllocationMode == "" {
		newConfiguration.WorkerAllocationMode = partitionworker.AllocationModePool
	}

	if newConfiguration.RebalanceRetryMax == 0 {
		newConfiguration.RebalanceRetryMax = 4
	}

	if newConfiguration.FetchMin == 0 {
		newConfiguration.FetchMin = 1
	}

	if newConfiguration.FetchDefault == 0 {
		newConfiguration.FetchDefault = 1 * 1024 * 1024
	}

	if newConfiguration.ChannelBufferSize == 0 {
		newConfiguration.ChannelBufferSize = 256
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
