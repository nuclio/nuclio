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
	Topics        []string
	ConsumerGroup string
	InitialOffset string
	SASL          struct {
		Enable   bool
		User     string
		Password string
	}

	SessionTimeout                string
	HeartbeatInterval             string
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
	CACert                        string
	AccessKey                     string
	AccessCertificate             string

	// resolved fields
	brokers                       []string
	initialOffset                 int64
	sessionTimeout                time.Duration
	heartbeatInterval             time.Duration
	maxProcessingTime             time.Duration
	rebalanceTimeout              time.Duration
	rebalanceRetryBackoff         time.Duration
	retryBackoff                  time.Duration
	maxWaitTime                   time.Duration
	maxWaitHandlerDuringRebalance time.Duration
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
		{Key: "nuclio.io/kafka-heartbeat-interval", ValueString: &newConfiguration.HeartbeatInterval},
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
		{Key: "nuclio.io/kafka-access-key", ValueString: &newConfiguration.AccessKey},
		{Key: "nuclio.io/kafka-access-cert", ValueString: &newConfiguration.AccessCertificate},
		{Key: "nuclio.io/kafka-ca-cert", ValueString: &newConfiguration.CACert},
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
		{
			Name:    "session timeout",
			Value:   newConfiguration.SessionTimeout,
			Field:   &newConfiguration.sessionTimeout,
			Default: 10 * time.Second,
		},
		{
			Name:    "heartbeat interval",
			Value:   newConfiguration.HeartbeatInterval,
			Field:   &newConfiguration.heartbeatInterval,
			Default: 3 * time.Second,
		},
		{
			Name:    "max processing timeout",
			Value:   newConfiguration.MaxProcessingTime,
			Field:   &newConfiguration.maxProcessingTime,
			Default: 24 * time.Hour,
		},
		{
			Name:    "rebalance timeout",
			Value:   newConfiguration.RebalanceTimeout,
			Field:   &newConfiguration.rebalanceTimeout,
			Default: 60 * time.Second,
		},
		{
			Name:    "rebalance retry backoff",
			Value:   newConfiguration.RebalanceRetryBackoff,
			Field:   &newConfiguration.rebalanceRetryBackoff,
			Default: 2 * time.Second,
		},
		{
			Name:    "retry backoff",
			Value:   newConfiguration.RetryBackoff,
			Field:   &newConfiguration.retryBackoff,
			Default: 2 * time.Second,
		},
		{
			Name:    "max wait time",
			Value:   newConfiguration.MaxWaitTime,
			Field:   &newConfiguration.maxWaitTime,
			Default: 250 * time.Millisecond,
		},
		{
			Name:    "max wait handler during rebalance",
			Value:   newConfiguration.MaxWaitHandlerDuringRebalance,
			Field:   &newConfiguration.maxWaitHandlerDuringRebalance,
			Default: 5 * time.Second,
		},
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

	// for certificates, replace spaces with newlines to allow passing in places like annotations
	for _, cert := range []*string{
		&newConfiguration.CACert,
		&newConfiguration.AccessKey,
		&newConfiguration.AccessCertificate,
	} {
		*cert = newConfiguration.unflattenCertificate(*cert)
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

func (c *Configuration) unflattenCertificate(certificate string) string {

	// if there are newlines in the certificate, it's not flat. return as is
	if strings.Contains(certificate, "\n") {
		return certificate
	}

	// in this mode, the user replaces newlines with "@"
	if strings.Contains(certificate, "@") {
		return strings.Replace(certificate, "@", "\n", -1)
	}

	//
	// try to be fancy and try to auto-unflatten the certificate
	//

	headers := []string{
		"BEGIN CERTIFICATE",
		"END CERTIFICATE",
		"BEGIN PRIVATE KEY",
		"END PRIVATE KEY",
	}

	// headers have spaces... remove them temporarily
	for _, spacedHeader := range headers {
		certificate = strings.Replace(certificate,
			spacedHeader,
			strings.Replace(spacedHeader, " ", "-", -1),
			-1)
	}

	// now replace all spaces with newline
	certificate = strings.Replace(certificate, " ", "\n", -1)

	// and revert header
	for _, spacedHeader := range headers {
		certificate = strings.Replace(certificate,
			strings.Replace(spacedHeader, " ", "-", -1),
			spacedHeader,
			-1)
	}

	return certificate
}
