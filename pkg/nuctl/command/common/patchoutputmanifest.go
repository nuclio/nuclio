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

package common

import (
	"context"
	"sync"

	"github.com/nuclio/logger"
)

type PatchOutputManifest struct {
	lock    sync.Mutex
	success []string
	skipped []string
	failed  map[string]error
}

func NewOutputManifest() *PatchOutputManifest {
	return &PatchOutputManifest{
		lock:    sync.Mutex{},
		success: []string{},
		skipped: []string{},
		failed:  make(map[string]error),
	}
}

func (om *PatchOutputManifest) AddSuccess(name string) {
	om.lock.Lock()
	defer om.lock.Unlock()

	om.success = append(om.success, name)
}

func (om *PatchOutputManifest) AddSkipped(name string) {
	om.lock.Lock()
	defer om.lock.Unlock()

	om.skipped = append(om.skipped, name)
}

func (om *PatchOutputManifest) AddFailure(name string, err error) {
	om.lock.Lock()
	defer om.lock.Unlock()

	om.failed[name] = err
}

func (om *PatchOutputManifest) GetSuccess() []string {
	om.lock.Lock()
	defer om.lock.Unlock()

	return om.success
}

func (om *PatchOutputManifest) GetSkipped() []string {
	om.lock.Lock()
	defer om.lock.Unlock()

	return om.skipped
}

func (om *PatchOutputManifest) GetFailed() map[string]error {
	om.lock.Lock()
	defer om.lock.Unlock()

	return om.failed
}

func (om *PatchOutputManifest) LogOutput(ctx context.Context, loggerInstance logger.Logger) {
	if len(om.GetSuccess()) > 0 {
		loggerInstance.InfoWithCtx(ctx, "Patched functions successfully",
			"functions", om.GetSuccess())
	}
	if len(om.GetSkipped()) > 0 {
		loggerInstance.InfoWithCtx(ctx, "Skipped functions",
			"functions", om.GetSkipped())
	}
	if len(om.GetFailed()) > 0 {
		for function, err := range om.GetFailed() {
			loggerInstance.ErrorWithCtx(ctx, "Failed to patch function",
				"function", function,
				"err", err)
		}
	}
}
