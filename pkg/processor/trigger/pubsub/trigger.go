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

package pubsub

import (
    "time"

    "github.com/nuclio/nuclio/pkg/common"
    "github.com/nuclio/nuclio/pkg/errors"
    "github.com/nuclio/nuclio/pkg/processor/trigger"
    "github.com/nuclio/nuclio/pkg/processor/worker"

    ps "cloud.google.com/go/pubsub"
    "github.com/nuclio/logger"
)

type pubsub struct {
    trigger.AbstractTrigger
    event            Event
    configuration    *Configuration
    stop             chan bool
    pubsubSubscription *ps.Subscription
}

func newTrigger(parentLogger logger.Logger,
    workerAllocator worker.Allocator,
    configuration *Configuration) (trigger.Trigger, error) {

    newTrigger := &pubsub{
        AbstractTrigger: trigger.AbstractTrigger{
            ID:              configuration.ID,
            Logger:          parentLogger.GetChild(configuration.ID),
            WorkerAllocator: workerAllocator,
            Class:           "async",
            Kind:            "pubsub",
        },
        configuration: configuration,
        stop:          make(chan bool),
    }

    return newTrigger, nil
}

func (n *pubsub) Start(checkpoint trigger.Checkpoint) error {
    n.Logger.InfoWith("Starting",
        "topic", n.configuration.Topic,
        "project", n.configuration.Project
    )
    ctx := context.Background()
    client, err := ps.NewClient(ctx, n.configuration.Project)
    if err != nil {
        return errors.Wrapf(err, "Can't connect to pubsub server %s", n.configuration.URL)
    }
    sub, err := client.CreateSubscription(ctx, "nuclio-trigger", ps.SubscriptionConfig{
        Topic:       n.configuration.Topic,
        AckDeadline: 10 * time.Second,
    })
    err = sub.Receive(ctx, func(ctx context.Context, m *ps.Message) {
        // NOTE: May be called concurrently; synchronize access to shared memory.
        n.event.psMessage = m
        // process the event, don't really do anything with response
        _, submitError, processError := n.AllocateWorkerAndSubmitEvent(&n.event, n.Logger, 10*time.Second)
        if submitError != nil {
            n.Logger.ErrorWith("Can't submit event", "error", submitError)
        }
        if processError != nil {
            n.Logger.ErrorWith("Can't process event", "error", processError)
        }
        m.Ack()
    })
    if err != context.Canceled {
        return errors.Wrapf(err, "Message reciever cancelled")
    }
    return nil
}

func (n *pubsub) Stop(force bool) (trigger.Checkpoint, error) {
    n.stop <- true
    ctx := context.Background()
    return nil, n.pubsubSubscription.Delete(ctx); err != nil {
        return errors.Wrapf(err, "Delete subscription")
    }
}

func (n *pubsub) GetConfig() map[string]interface{} {
    return common.StructureToMap(n.configuration)
}
