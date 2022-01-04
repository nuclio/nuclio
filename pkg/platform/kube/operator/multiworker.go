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

package operator

import (
	"context"
	"fmt"
	"time"

	"github.com/nuclio/nuclio/pkg/common"

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

type MultiWorker struct {
	logger               logger.Logger
	queue                workqueue.RateLimitingInterface
	informer             cache.SharedIndexInformer
	numWorkers           int
	maxProcessingRetries int
	stopChannel          chan struct{}
	changeHandler        ChangeHandler
}

func NewMultiWorker(ctx context.Context,
	parentLogger logger.Logger,
	numWorkers int,
	listWatcher cache.ListerWatcher,
	object runtime.Object,
	resyncInterval *time.Duration,
	changeHandler ChangeHandler) (Operator, error) {
	newMultiWorker := &MultiWorker{
		logger:               parentLogger.GetChild("operator"),
		numWorkers:           numWorkers,
		maxProcessingRetries: 3,
		stopChannel:          make(chan struct{}),
		changeHandler:        changeHandler,
	}

	newMultiWorker.logger.DebugWithCtx(ctx,
		"Creating multiworker",
		"numWorkers", numWorkers,
		"resyncInterval", resyncInterval,
		"objectKind", fmt.Sprintf("%T", object))

	// create rate limited queue
	newMultiWorker.queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	// set default resync
	if resyncInterval == nil {
		defaultInterval := 5 * time.Minute
		resyncInterval = &defaultInterval
	}

	// create a shared index informer
	newMultiWorker.informer = cache.NewSharedIndexInformer(listWatcher, object, *resyncInterval, cache.Indexers{})

	// register event handlers for the informer
	newMultiWorker.informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(obj)
			if err == nil {
				newMultiWorker.queue.Add(key)
			}
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			key, err := cache.MetaNamespaceKeyFunc(new)
			if err == nil {
				newMultiWorker.queue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
			if err == nil {
				newMultiWorker.queue.Add(key)
			}
		},
	})

	return newMultiWorker, nil
}

func (mw *MultiWorker) Start(ctx context.Context) error {
	mw.logger.InfoWithCtx(ctx, "Starting")

	// run the informer
	go func() {
		defer common.CatchAndLogPanic(ctx, // nolint: errcheck
			mw.logger,
			"running multi worker informer")

		mw.informer.Run(mw.stopChannel)
	}()

	// wait for cache to sync up with
	if !cache.WaitForCacheSync(mw.stopChannel, mw.informer.HasSynced) {
		return errors.New("Failed to wait for cache sync")
	}

	workersCtx, workersCtxCancel := context.WithCancel(ctx)
	for workerID := 0; workerID < mw.numWorkers; workerID++ {
		workerID := workerID
		go func() {
			defer common.CatchAndLogPanic(ctx, // nolint: errcheck
				mw.logger,
				"processing items")

			workerCtx := context.WithValue(workersCtx, WorkerIDKey, workerID)

			// assign each worker that process items with context
			wait.UntilWithContext(workerCtx, mw.processItems, time.Second)
		}()
	}

	// wait for stop signal
	<-mw.stopChannel

	// stop workers context
	workersCtxCancel()

	mw.logger.InfoWithCtx(ctx, "Stopped")
	return nil
}

func (mw *MultiWorker) Stop() chan struct{} {

	// TODO
	return nil
}

func (mw *MultiWorker) processItems(ctx context.Context) {
	workerID := ctx.Value(WorkerIDKey)
	for {

		select {
		case <-ctx.Done():
			mw.logger.DebugWithCtx(ctx, "Context is terminated", "workerID", workerID)
			return
		default:

			// get next item from the queue
			item, shutdown := mw.queue.Get()
			if shutdown {
				mw.logger.DebugWithCtx(ctx, "Worker shutting down", "workerID", workerID)
				break
			}

			// get the key from the item
			itemKey, keyIsString := item.(string)
			if !keyIsString {
				mw.logger.WarnWithCtx(ctx,
					"Got item which is not a string, ignoring",
					"itemKey", itemKey,
					"workerID", workerID)
			}

			// try to process the item
			if err := mw.processItem(ctx, itemKey); err != nil {
				mw.logger.WarnWithCtx(ctx,
					"Failed to process item",
					"workerID", workerID,
					"itemKey", itemKey,
					"err", errors.Cause(err).Error())

				// do we have any more retries?
				if mw.queue.NumRequeues(itemKey) < mw.maxProcessingRetries {
					mw.logger.DebugWithCtx(ctx,
						"Requeueing",
						"itemKey", itemKey,
						"workerID", workerID)

					// add it back, rate limited
					mw.queue.AddRateLimited(itemKey)
				} else {
					mw.logger.WarnWithCtx(ctx,
						"No retries, left. Giving up",
						"workerID", workerID,
						"itemKey", itemKey)

					mw.queue.Forget(itemKey)
				}

			} else {

				// we're done with this key
				mw.queue.Forget(itemKey)
			}

			// indicate that we're done with the item
			mw.queue.Done(item)
		}

	}
}

func (mw *MultiWorker) processItem(ctx context.Context, itemKey string) error {
	itemNamespace, itemName, err := cache.SplitMetaNamespaceKey(itemKey)
	if err != nil {
		return errors.Wrap(err, "Failed to split item key to namespace/name")
	}

	// Get the object
	itemObject, itemObjectExists, err := mw.informer.GetIndexer().GetByKey(itemKey)
	if err != nil {
		mw.logger.ErrorWithCtx(ctx,
			"Failed to find item by key",
			"err", errors.Cause(err).Error(),
			"itemKey", itemKey,
			"itemObjectExists", itemObjectExists)
		return errors.Wrapf(err, "Failed to find item by key %s", itemKey)
	}

	mw.logger.DebugWithCtx(ctx,
		"Got item from queue",
		"itemKey", itemKey,
		"itemObjectExists", itemObjectExists,
		"itemObjectType", fmt.Sprintf("%T", itemObject))

	// if the item doesn't exist
	if !itemObjectExists {

		// do the deletion
		return mw.changeHandler.Delete(ctx, itemNamespace, itemName)
	}

	// do the create or update
	return mw.changeHandler.CreateOrUpdate(ctx, itemObject.(runtime.Object))
}
