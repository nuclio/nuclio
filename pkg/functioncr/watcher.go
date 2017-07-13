package functioncr

import (
	"time"

	"github.com/nuclio/nuclio-logger/logger"
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
	logger     logger.Logger
	changeChan chan Change
}

func newWatcher(client *Client, changeChan chan Change) (*Watcher, error) {
	newWatcher := &Watcher{
		logger:     client.logger.GetChild("watcher").(logger.Logger),
		changeChan: changeChan,
	}

	newWatcher.logger.Debug("Watching for changes")

	listWatch := cache.NewListWatchFromClient(client.restClient,
		client.getNamePlural(),
		"", // TODO: this should be passed from somewhere
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

	w.changeChan <- Change{kind, function, previousFunction}
}
