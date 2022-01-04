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
	"time"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/trigger"
	"github.com/nuclio/nuclio/pkg/processor/util/partitionworker"
	"github.com/nuclio/nuclio/pkg/processor/worker"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio-sdk-go"
	"github.com/v3io/v3io-go/pkg/dataplane"
	v3iohttp "github.com/v3io/v3io-go/pkg/dataplane/http"
	"github.com/v3io/v3io-go/pkg/dataplane/streamconsumergroup"
)

type submittedEvent struct {
	event  Event
	worker *worker.Worker
	done   chan error
}

type v3iostream struct {
	trigger.AbstractTrigger
	configuration             *Configuration
	v3iostreamConfig          *streamconsumergroup.Config
	streamConsumerGroupMember streamconsumergroup.Member
	shutdownSignal            chan struct{}
	stopConsumptionChan       chan struct{}
	partitionWorkerAllocator  partitionworker.Allocator
	topic                     string
}

func newTrigger(parentLogger logger.Logger,
	workerAllocator worker.Allocator,
	configuration *Configuration) (trigger.Trigger, error) {
	var err error

	loggerInstance := parentLogger.GetChild(configuration.ID)

	newTrigger := &v3iostream{
		configuration:       configuration,
		stopConsumptionChan: make(chan struct{}, 1),
		topic:               "v3io", // v3io doesn't support topics, use constant (never goes to v3io)
	}

	newTrigger.AbstractTrigger, err = trigger.NewAbstractTrigger(loggerInstance,
		workerAllocator,
		&configuration.Configuration,
		"async",
		"v3io-stream",
		configuration.Name)
	if err != nil {
		return nil, errors.New("Failed to create abstract trigger")
	}

	newTrigger.v3iostreamConfig, err = configuration.getStreamConsumerGroupConfig()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get v3io stream config")
	}

	return newTrigger, nil
}

func (vs *v3iostream) Start(checkpoint functionconfig.Checkpoint) error {
	var err error

	vs.streamConsumerGroupMember, err = vs.newConsumerGroupMember()
	if err != nil {
		return errors.Wrap(err, "Failed to create consumer")
	}

	vs.shutdownSignal = make(chan struct{}, 1)

	// start consumption in the background
	go func() {
		vs.Logger.DebugWith("Starting to consume from v3io")
		for {

			// start consuming
			if err := vs.streamConsumerGroupMember.Consume(vs); err != nil {
				vs.Logger.WarnWith("Failed to consume from group, retrying...",
					"err", errors.GetErrorStackString(err, 10))
				time.Sleep(1 * time.Second)
				continue
			}

			// TODO: support rebalance. Once supported, comment out below break
			break
		}
	}()

	return nil
}

func (vs *v3iostream) Stop(force bool) (functionconfig.Checkpoint, error) {
	vs.shutdownSignal <- struct{}{}
	close(vs.shutdownSignal)

	if err := vs.streamConsumerGroupMember.Close(); err != nil {
		return nil, errors.Wrap(err, "Failed to close consumer")
	}
	return nil, nil
}

func (vs *v3iostream) GetConfig() map[string]interface{} {
	return common.StructureToMap(vs.configuration)
}

func (vs *v3iostream) Setup(session streamconsumergroup.Session) error {
	var err error

	var shardIDs []int
	for _, claim := range session.GetClaims() {
		shardIDs = append(shardIDs, claim.GetShardID())
	}

	vs.Logger.InfoWith("Starting consumer session",
		"shardIDs", shardIDs,
		"memberID", session.GetMemberID(),
		"workersAvailable", vs.WorkerAllocator.GetNumWorkersAvailable())

	vs.partitionWorkerAllocator, err = vs.createPartitionWorkerAllocator(session)
	if err != nil {
		return errors.Wrap(err, "Failed to create partition worker allocator")
	}

	return nil
}

func (vs *v3iostream) Cleanup(session streamconsumergroup.Session) error {
	if err := vs.partitionWorkerAllocator.Stop(); err != nil {
		return errors.Wrap(err, "Failed to stop partition worker allocator")
	}

	vs.Logger.InfoWith("Ending consumer session",
		"claims", session.GetClaims(),
		"memberID", session.GetMemberID(),
		"workersAvailable", vs.WorkerAllocator.GetNumWorkersAvailable())

	return nil
}

func (vs *v3iostream) ConsumeClaim(session streamconsumergroup.Session, claim streamconsumergroup.Claim) error {
	var submitError error

	submittedEventInstance := submittedEvent{
		done: make(chan error),
	}

	submittedEventChan := make(chan *submittedEvent)

	// submit the events in a goroutine so that we can unblock immediately
	go vs.eventSubmitter(claim, submittedEventChan)

	if vs.configuration.AckWindowSize > 0 {
		vs.Logger.DebugWith("Starting claim consumption with ack window",
			"shardID", claim.GetShardID(),
			"ackWindowSize", vs.configuration.AckWindowSize)
	}

	commitRecordFuncHandler := vs.resolveCommitRecordFuncHandler(session)

	// the exit condition is that (a) the Messages() channel was closed and (b) we got a signal telling us
	// to stop consumption
	for recordBatch := range claim.GetRecordBatchChan() {
		for recordIndex := 0; recordIndex < len(recordBatch.Records); recordIndex++ {
			record := &recordBatch.Records[recordIndex]

			// allocate a worker for this topic/partition
			workerInstance, cookie, err := vs.partitionWorkerAllocator.AllocateWorker(vs.topic, claim.GetShardID(), nil)
			if err != nil {
				return errors.Wrap(err, "Failed to allocate worker")
			}

			submittedEventInstance.event.record = record
			submittedEventInstance.worker = workerInstance

			// handle in the goroutine so we don't block
			submittedEventChan <- &submittedEventInstance

			// wait for handling done or indication to stop
			err = <-submittedEventInstance.done

			// we successfully submitted the message to the handler. mark it
			if err == nil {
				commitRecordFuncHandler(record)
			}

			// release the worker from whence it came
			if err := vs.partitionWorkerAllocator.ReleaseWorker(cookie, workerInstance); err != nil {
				return errors.Wrap(err, "Failed to release worker")
			}

		}
	}

	vs.Logger.DebugWith("Claim consumption stopped", "shardID", claim.GetShardID())

	// shut down the event submitter
	close(submittedEventChan)

	return submitError
}

func (vs *v3iostream) eventSubmitter(claim streamconsumergroup.Claim, submittedEventChan chan *submittedEvent) {
	vs.Logger.DebugWith("Event submitter started",
		"shardID", claim.GetShardID())

	// while there are events to submit, submit them to the given worker
	for submittedEvent := range submittedEventChan {

		// submit the event to the worker
		_, processErr := vs.SubmitEventToWorker(nil, submittedEvent.worker, &submittedEvent.event)
		if processErr != nil {
			vs.Logger.DebugWith("Event processing error",
				"shardID", submittedEvent.event.record.ShardID,
				"err", processErr)
		}

		// indicate that we're done
		submittedEvent.done <- processErr
	}

	vs.Logger.DebugWith("Event submitter stopped", "shardID", claim.GetShardID())
}

func (vs *v3iostream) newConsumerGroupMember() (streamconsumergroup.Member, error) {

	v3ioContext, err := v3iohttp.NewContext(vs.Logger,
		&v3iohttp.NewContextInput{
			NumWorkers: vs.configuration.NumTransportWorkers,
		})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create v3io context")
	}

	v3ioSession, err := v3ioContext.NewSession(&v3io.NewSessionInput{
		URL:       vs.configuration.URL,
		Username:  vs.configuration.Username,
		Password:  vs.configuration.Password,
		AccessKey: vs.configuration.Secret,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create v3io session")
	}

	v3ioContainer, err := v3ioSession.NewContainer(&v3io.NewContainerInput{
		ContainerName: vs.configuration.ContainerName,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create v3io container")
	}

	// ensure path exists
	if err := v3ioContainer.CheckPathExistsSync(&v3io.CheckPathExistsInput{
		Path: vs.configuration.StreamPath,
	}); err != nil {
		if errWithStatusCode, ok := err.(nuclio.WithStatusCode); ok &&
			errWithStatusCode.StatusCode() == nuclio.ErrNotFound.StatusCode() {
			vs.Logger.WarnWith("Stream path does not exists",
				"path", vs.configuration.StreamPath)
			return nil, errors.Wrap(err, "Stream path does not exists")
		}
		return nil, errors.Wrap(err, "Failed to check stream path existence")
	}

	maxReplicas := 1
	if vs.configuration.RuntimeConfiguration.Config.Spec.Replicas != nil {
		maxReplicas = *vs.configuration.RuntimeConfiguration.Config.Spec.Replicas
	} else if vs.configuration.RuntimeConfiguration.Config.Spec.MaxReplicas != nil {
		maxReplicas = *vs.configuration.RuntimeConfiguration.Config.Spec.MaxReplicas
	}

	streamConsumerGroup, err := streamconsumergroup.NewStreamConsumerGroup(vs.Logger,
		vs.configuration.ConsumerGroup,
		vs.v3iostreamConfig,
		v3ioContainer,
		vs.configuration.StreamPath,
		maxReplicas)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer group")
	}

	streamConsumerGroupMember, err := streamconsumergroup.NewMember(streamConsumerGroup, vs.FunctionName)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer group member")
	}

	vs.Logger.DebugWith("Consumer created",
		"clusterURL", vs.configuration.URL,
		"containerName", vs.configuration.ContainerName,
		"streamPath", vs.configuration.StreamPath)

	return streamConsumerGroupMember, nil
}

func (vs *v3iostream) createPartitionWorkerAllocator(session streamconsumergroup.Session) (partitionworker.Allocator, error) {
	switch vs.configuration.WorkerAllocationMode {
	case partitionworker.AllocationModePool:
		return partitionworker.NewPooledWorkerAllocator(vs.Logger, vs.WorkerAllocator)

	case partitionworker.AllocationModeStatic:
		var shardIDs []int

		for _, claim := range session.GetClaims() {
			shardIDs = append(shardIDs, claim.GetShardID())
		}

		return partitionworker.NewStaticWorkerAllocator(vs.Logger,
			vs.WorkerAllocator,
			map[string][]int{
				vs.topic: shardIDs,
			})

	default:
		return nil, errors.Errorf("Unknown worker allocation mode: %s", vs.configuration.WorkerAllocationMode)
	}
}

func (vs *v3iostream) resolveCommitRecordFuncHandler(session streamconsumergroup.Session) func(*v3io.StreamRecord) {

	commitRecordDefaultFuncHandler := func(record *v3io.StreamRecord) {
		session.MarkRecord(record) // nolint: errcheck
	}

	commitRecordWithWindowAckSizeFuncHandler := func(record *v3io.StreamRecord) {

		// offset record sequence number by the trigger's configured ack window size
		if record.SequenceNumber >= vs.configuration.AckWindowSize {
			record.SequenceNumber -= vs.configuration.AckWindowSize
			session.MarkRecord(record) // nolint: errcheck
		}

		// nothing to mark just yet
	}

	if vs.configuration.AckWindowSize > 0 {
		return commitRecordWithWindowAckSizeFuncHandler
	}

	return commitRecordDefaultFuncHandler
}
