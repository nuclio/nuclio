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
package worker

import (
	"sync"

	"github.com/nuclio/errors"
)

type AllocatorSyncMap struct {
	syncMap *sync.Map
	lock    sync.Locker
}

func NewAllocatorSyncMap() *AllocatorSyncMap {
	return &AllocatorSyncMap{
		syncMap: &sync.Map{},
		lock:    &sync.Mutex{},
	}
}

// Load returns the worker allocator stored in the map for a key
func (w *AllocatorSyncMap) Load(key string) (Allocator, bool) {
	load, found := w.syncMap.Load(key)
	if found {
		return load.(Allocator), found
	}
	return nil, false
}

// Store sets the worker allocator per key
func (w *AllocatorSyncMap) Store(key string, value Allocator) {
	w.syncMap.Store(key, value)
}

// Keys returns all allocator keys
func (w *AllocatorSyncMap) Keys() []string {
	var keys []string
	w.Range(func(s string, allocator Allocator) bool {
		keys = append(keys, s)
		return true
	})
	return keys
}

// LoadOrStore tries to load exiting worker by key, if not existing - creates one and returns it
// if key is empty - always create and return a new worker allocator
func (w *AllocatorSyncMap) LoadOrStore(key string,
	workerAllocatorCreator func() (Allocator, error)) (Allocator, error) {

	if key == "" {
		return workerAllocatorCreator()
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	// try to find worker allocator
	workerAllocator, loaded := w.Load(key)

	// if it already exists, just use it
	if loaded {
		return workerAllocator, nil
	}

	// if it doesn't exist - create it
	workerAllocator, err := workerAllocatorCreator()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create worker allocator")
	}

	w.syncMap.Store(key, workerAllocator)
	return workerAllocator, nil
}

// Delete delete worker allocator
func (w *AllocatorSyncMap) Delete(key string) {
	w.syncMap.Delete(key)
}

// Range calls handler for each worker allocator in map
// if handler returns false, iteration is stopped
func (w *AllocatorSyncMap) Range(handler func(string, Allocator) bool) {
	w.syncMap.Range(func(key, value interface{}) bool {
		return handler(key.(string), value.(Allocator))
	})
}
