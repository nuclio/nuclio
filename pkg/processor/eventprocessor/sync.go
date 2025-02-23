/*
Copyright 2023 The Nuclio Authors.

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

package eventprocessor

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

// Load returns the allocator stored in the map for a key
func (w *AllocatorSyncMap) Load(key string) (Allocator, bool) {
	load, found := w.syncMap.Load(key)
	if found {
		return load.(Allocator), found
	}
	return nil, false
}

// Store sets the allocator per key
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

// LoadOrStore tries to load exiting object by key, if not existing - creates one and returns it
// if key is empty - always create and return a new object allocator
func (w *AllocatorSyncMap) LoadOrStore(key string,
	allocatorCreator func() (Allocator, error)) (Allocator, error) {

	if key == "" {
		return allocatorCreator()
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	// try to find an allocator
	objectAllocator, loaded := w.Load(key)

	// if it already exists, just use it
	if loaded {
		return objectAllocator, nil
	}

	// if it doesn't exist - create it
	objectAllocator, err := allocatorCreator()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create allocator")
	}

	w.syncMap.Store(key, objectAllocator)
	return objectAllocator, nil
}

// Delete deletes allocator
func (w *AllocatorSyncMap) Delete(key string) {
	w.syncMap.Delete(key)
}

// Range calls handler for each allocator in map
// if handler returns false, iteration is stopped
func (w *AllocatorSyncMap) Range(handler func(string, Allocator) bool) {
	w.syncMap.Range(func(key, value interface{}) bool {
		return handler(key.(string), value.(Allocator))
	})
}
