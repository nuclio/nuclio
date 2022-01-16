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

package v3iostream

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/runtime"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"

	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	v3io "github.com/v3io/v3io-go/pkg/dataplane"
	"github.com/v3io/v3io-go/pkg/dataplane/streamconsumergroup"
)

type Configuration struct {
	trigger.Configuration
	ConsumerGroup                   string
	ContainerName                   string
	StreamPath                      string
	NumTransportWorkers             int
	WorkerAllocationMode            partitionworker.AllocationMode
	SeekTo                          string
	ReadBatchSize                   int
	SessionTimeout                  string
	HeartbeatInterval               string
	SequenceNumberCommitInterval    string
	SequenceNumberShardWaitInterval string
	RecordBatchSizeChan             int
	AckWindowSize                   uint64

	seekTo v3io.SeekShardInputType

	// backwards compatibility
	PollingIntervalMs int
}

func NewConfiguration(id string, triggerConfiguration *functionconfig.Trigger,
	runtimeConfiguration *runtime.Configuration,
	logger logger.Logger) (*Configuration, error) {
	newConfiguration := Configuration{}

	// create base
	newConfiguration.Configuration = *trigger.NewConfiguration(id, triggerConfiguration, runtimeConfiguration)

	if err := newConfiguration.PopulateConfigurationFromAnnotations([]trigger.AnnotationConfigField{
		{Key: "custom.nuclio.io/v3iostream-window-size", ValueUInt64: &newConfiguration.AckWindowSize},
	}); err != nil {
		return nil, errors.Wrap(err, "Failed to populate configuration from annotations")
	}

	// parse attributes
	if err := mapstructure.Decode(newConfiguration.Configuration.Attributes, &newConfiguration); err != nil {
		return nil, errors.Wrap(err, "Failed to decode attributes")
	}

	if newConfiguration.NumTransportWorkers == 0 {
		newConfiguration.NumTransportWorkers = 8
	}

	if newConfiguration.ReadBatchSize == 0 {
		newConfiguration.ReadBatchSize = 64
	}

	switch newConfiguration.SeekTo {
	case "", "latest":
		newConfiguration.seekTo = v3io.SeekShardInputTypeLatest
	case "earliest":
		newConfiguration.seekTo = v3io.SeekShardInputTypeEarliest
	default:
		return nil, errors.Errorf("Invalid value for seekTo: %s", newConfiguration.SeekTo)
	}

	if newConfiguration.SeekTo == "" {
		newConfiguration.SeekTo = "latest"
	}

	if newConfiguration.WorkerAllocationMode == "" {
		newConfiguration.WorkerAllocationMode = partitionworker.AllocationModePool
	}

	// for backwards compatibility, allow populating container name, streampath and consumer group
	// name from url
	if newConfiguration.ContainerName == "" &&
		newConfiguration.StreamPath == "" &&
		newConfiguration.ConsumerGroup == "" {
		if err := newConfiguration.parseURLForBackwardsCompatibility(); err != nil {
			return nil, errors.Wrap(err, "Could not parse URL")
		}
	}

	// if the password is a uuid - assume it is an access key and clear out the username/pass
	if _, err := uuid.ParseUUID(newConfiguration.Password); err == nil {
		newConfiguration.Secret = newConfiguration.Password
		newConfiguration.Username = ""
		newConfiguration.Password = ""
	}

	return &newConfiguration, nil
}

// Parses: https://some.address.com:8080/mycontainername/some/stream/path@consumergroup
// into url, container name, stream path, consumer group
func (c *Configuration) parseURLForBackwardsCompatibility() error {
	parsedURL, err := url.Parse(c.URL)
	if err != nil {
		return errors.Wrap(err, "Failed to parse URL")
	}

	c.URL = fmt.Sprintf("%s://%s", parsedURL.Scheme, parsedURL.Host)

	// the path should contain the consumer group name
	splitPathAndConsumerGroupName := strings.Split(parsedURL.Path, "@")
	if len(splitPathAndConsumerGroupName) != 2 {
		return errors.Errorf("Path must contain @ indicating consumer group name")
	}

	// set consumer group name
	c.ConsumerGroup = splitPathAndConsumerGroupName[1]

	conatinerNameAndStreamPath := splitPathAndConsumerGroupName[0]

	// path starts with "/", remove it
	conatinerNameAndStreamPath = strings.TrimPrefix(conatinerNameAndStreamPath, "/")

	// split the path
	splitPath := strings.SplitN(conatinerNameAndStreamPath, "/", 2)

	// must contain at least two parts - the container name and stream path
	if len(splitPath) != 2 {
		return errors.Errorf("Path must contain the container name and stream path: %s", parsedURL.Path)
	}

	// first part is the container name
	c.ContainerName = splitPath[0]
	c.StreamPath = "/" + splitPath[1]

	return nil
}

func (c *Configuration) getStreamConsumerGroupConfig() (*streamconsumergroup.Config, error) {
	streamConsumerGroupConfig := streamconsumergroup.NewConfig()
	streamConsumerGroupConfig.Claim.RecordBatchChanSize = c.RecordBatchSizeChan
	streamConsumerGroupConfig.Claim.RecordBatchFetch.NumRecordsInBatch = c.ReadBatchSize
	streamConsumerGroupConfig.Claim.RecordBatchFetch.InitialLocation = c.seekTo

	for _, durationConfigField := range []trigger.DurationConfigField{
		{
			Name:    "session timeout",
			Value:   c.SessionTimeout,
			Field:   &streamConsumerGroupConfig.Session.Timeout,
			Default: 10 * time.Second,
		},
		{
			Name:    "heartbeat interval",
			Value:   c.HeartbeatInterval,
			Field:   &streamConsumerGroupConfig.Session.HeartbeatInterval,
			Default: 3 * time.Second,
		},
		{
			Name:    "sequence number commit interval",
			Value:   c.SequenceNumberCommitInterval,
			Field:   &streamConsumerGroupConfig.SequenceNumber.CommitInterval,
			Default: 1 * time.Second,
		},
		{
			Name:    "sequence number shard wait interval",
			Value:   c.SequenceNumberShardWaitInterval,
			Field:   &streamConsumerGroupConfig.SequenceNumber.ShardWaitInterval,
			Default: 1 * time.Second,
		},
	} {
		if err := c.ParseDurationOrDefault(&durationConfigField); err != nil {
			return nil, err
		}
	}

	return streamConsumerGroupConfig, nil
}
