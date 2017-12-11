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

package functioncr

import (
	"time"

	"github.com/nuclio/nuclio-sdk"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"
)

type ChangeKind int

const (
	ChangeKindAdded ChangeKind = iota
	ChangeKindUpdated
	ChangeKindDeleted
)

type Change struct {
	Kind             ChangeKind
	Function         *Function
	PreviousFunction *Function // applicable only in updated
}

type Watcher struct {
	client     *Client
	logger     nuclio.Logger
	namespace  string
	changeChan chan Change
}

func newWatcher(client *Client, namespace string, changeChan chan Change) (*Watcher, error) {
	newWatcher := &Watcher{
		logger:     client.logger.GetChild("watcher"),
		namespace:  namespace,
		changeChan: changeChan,
	}

	newWatcher.logger.Debug("Watching for changes")

	listWatch := cache.NewListWatchFromClient(client.restClient,
		client.getNamePlural(),
		newWatcher.namespace,
		fields.Everything())

	_, controller := cache.NewInformer(
		listWatch,
		&Function{},
		10*time.Minute,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(function interface{}) {
				newWatcher.dispatchChange(ChangeKindAdded, function.(*Function), nil)
			},
			DeleteFunc: func(function interface{}) {
				newWatcher.dispatchChange(ChangeKindDeleted, function.(*Function), nil)
			},
			UpdateFunc: func(previousFunction interface{}, newFunction interface{}) {
				newWatcher.dispatchChange(ChangeKindUpdated,
					newFunction.(*Function),
					previousFunction.(*Function))
			},
		},
	)

	// run the watcher. TODO: pass a channel that can receive stop requests, when stop is supported
	go controller.Run(make(chan struct{}))

	return newWatcher, nil
}

func (w *Watcher) dispatchChange(kind ChangeKind, function *Function, previousFunction *Function) {
	w.logger.DebugWith("Dispatching change",
		"kind", kind,
		"function_name", function.Name)

	// sanitize the unmarshalled function (work around unmarshalling issues)
	function.Sanitize()

	w.changeChan <- Change{kind, function, previousFunction}
}
