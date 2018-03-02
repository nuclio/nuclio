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
	"time"

	"github.com/nuclio/nuclio/pkg/errors"

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

func NewMultiWorker(parentLogger logger.Logger,
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
	newMultiWorker.informer.AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
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
	}, *resyncInterval)

	return newMultiWorker, nil
}

func (mw *MultiWorker) Start() error {
	mw.logger.InfoWith("Starting")

	// run the informer
	go mw.informer.Run(mw.stopChannel)

	// wait for cache to sync up with
	if !cache.WaitForCacheSync(mw.stopChannel, mw.informer.HasSynced) {
		return errors.New("Failed to wait for cache sync")
	}

	for workerIdx := 0; workerIdx < mw.numWorkers; workerIdx++ {
		go func() {
			wait.Until(mw.processItems, time.Second, mw.stopChannel)
		}()
	}

	// wait for stop signal
	<-mw.stopChannel

	mw.logger.InfoWith("Stopped")

	return nil
}

func (mw *MultiWorker) Stop() chan struct{} {

	// TODO
	return nil
}

func (mw *MultiWorker) processItems() {
	for {

		// get next item from the queue
		item, shutdown := mw.queue.Get()
		if shutdown {
			mw.logger.DebugWith("Worker shutting down")
			break
		}

		// get the key from the item
		itemKey, keyIsString := item.(string)
		if !keyIsString {
			mw.logger.WarnWith("Got item which is not a string, ignoring")
		}

		// try to process the item
		err := mw.processItem(itemKey)
		if err != nil {
			mw.logger.WarnWith("Failed to process item", "itemKey", itemKey, "err", err)

			// do we have any more retries?
			if mw.queue.NumRequeues(itemKey) < mw.maxProcessingRetries {
				mw.logger.DebugWith("Requeueing", "itemKey", itemKey)

				// add it back, rate limited
				mw.queue.AddRateLimited(itemKey)
			} else {
				mw.logger.WarnWith("No retries, left. Giving up", "itemKey", itemKey)

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

func (mw *MultiWorker) processItem(itemKey string) error {

	itemNamespace, itemName, err := cache.SplitMetaNamespaceKey(itemKey)
	if err != nil {
		return errors.Wrap(err, "Failed to split item key to namespace/name")
	}

	// Get the object
	itemObject, itemObjectExists, err := mw.informer.GetIndexer().GetByKey(itemKey)
	if err != nil {
		return err
	}

	mw.logger.DebugWith("Got item from queue",
		"itemKey", itemKey,
		"itemObjectExists", itemObjectExists,
		"itemObject", itemObject)

	// if the item doesn't exist
	if !itemObjectExists {

		// do the delete
		return mw.changeHandler.Delete(itemNamespace, itemName)
	}

	// do the create or update
	return mw.changeHandler.CreateOrUpdate(itemObject.(runtime.Object))
}
