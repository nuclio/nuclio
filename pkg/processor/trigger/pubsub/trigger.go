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
    "fmt"
    "time"

    "github.com/nuclio/nuclio/pkg/common"
    "github.com/nuclio/nuclio/pkg/errors"
    "github.com/nuclio/nuclio/pkg/processor/trigger"
    "github.com/nuclio/nuclio/pkg/processor/worker"

    ps "cloud.google.com/go/pubsub"
    "github.com/nuclio/logger"
    "github.com/rs/xid"
)

type pubsub struct {
    trigger.AbstractTrigger
    event              Event
    configuration      *Configuration
    stop               chan bool
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
        "project", n.configuration.Project,
    )
    ctx := context.Background()

    // pubsub client consumes namespace/project string to be created
    client, err := ps.NewClient(ctx, n.configuration.Project)
    if err != nil {
        return errors.Wrapf(err, "Can't connect to pubsub project")
    }

    // every replica will get a unique id
    subName := fmt.Sprintf("nuclio-trigger-%s", xid.New().String())
    sub := client.CreateSubscription(ctx, subName, ps.SubscriptionConfig{
        Topic:       n.configuration.Topic,
        AckDeadline: 10 * time.Second,
    })

    // https://godoc.org/cloud.google.com/go/pubsub#ReceiveSettings
    sub.ReceiveSettings.NumGoroutines = n.configuration.MaxWorkers

    // listen to subscribed topic messages
    err = sub.Receive(ctx, func(ctx context.Context, m *ps.Message) {
        // NOTE: May be called concurrently; synchronize access to shared memory.
        n.event.psMessage = m

        // process the event, don't really do anything with response
        _, submitError, processError := n.AllocateWorkerAndSubmitEvent(&n.event, n.Logger, 10*time.Second)
        if submitError != nil {
            n.Logger.ErrorWith("Can't submit event", "error", submitError)
            m.Nack() // necessary to call on fail
            return
        }
        if processError != nil {
            n.Logger.ErrorWith("Can't process event", "error", processError)
            m.Nack()
            return
        }
        m.Ack()
    })
    if err != context.Canceled {
        return errors.Wrapf(err, "Message receiver cancelled")
    }
    return nil
}

func (n *pubsub) Stop(force bool) (trigger.Checkpoint, error) {
    n.stop <- true
    ctx := context.Background()
    if err := n.pubsubSubscription.Delete(ctx); err != nil {
        return errors.Wrapf(err, "Delete subscription")
    }
    return nil
}

func (n *pubsub) GetConfig() map[string]interface{} {
    return common.StructureToMap(n.configuration)
}
