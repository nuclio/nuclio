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
	"encoding/json"
	"os"
	"sync"

	"github.com/nuclio/logger"
)

type PatchManifest struct {
	*patchManifest
	lock sync.Mutex
}

type patchManifest struct {
	Success []string                   `json:"success,omitempty"`
	Skipped []string                   `json:"skipped,omitempty"`
	Failed  map[string]FailDescription `json:"failed,omitempty"`
}

type FailDescription struct {
	Err       string `json:"error,omitempty"`
	Retryable bool   `json:"retryable"`
}

func NewPatchManifest() *PatchManifest {
	return &PatchManifest{
		lock: sync.Mutex{},
		patchManifest: &patchManifest{
			Success: []string{},
			Skipped: []string{},
			Failed:  make(map[string]FailDescription),
		},
	}
}

func NewPatchManifestFromFile(path string) *PatchManifest {
	parsedManifest, err := readManifestFromFile(path)
	if err != nil {
		return nil
	}
	return &PatchManifest{
		patchManifest: parsedManifest,
		lock:          sync.Mutex{},
	}
}

func (om *PatchManifest) AddSuccess(name string) {
	om.lock.Lock()
	defer om.lock.Unlock()

	om.Success = append(om.Success, name)
}

func (om *PatchManifest) AddSkipped(name string) {
	om.lock.Lock()
	defer om.lock.Unlock()

	om.Skipped = append(om.Skipped, name)
}

func (om *PatchManifest) AddFailure(name string, err error, retryable bool) {
	om.lock.Lock()
	defer om.lock.Unlock()

	om.Failed[name] = FailDescription{
		Err:       err.Error(),
		Retryable: retryable,
	}
}

func (om *PatchManifest) GetSuccess() []string {
	om.lock.Lock()
	defer om.lock.Unlock()

	return om.Success
}

func (om *PatchManifest) GetSkipped() []string {
	om.lock.Lock()
	defer om.lock.Unlock()

	return om.Skipped
}

func (om *PatchManifest) GetFailed() map[string]FailDescription {
	om.lock.Lock()
	defer om.lock.Unlock()

	return om.Failed
}

func (om *PatchManifest) GetRetryable() []string {
	retryable := make([]string, 0)
	for name, deployRes := range om.GetFailed() {
		if deployRes.Retryable {
			retryable = append(retryable, name)
		}
	}
	return retryable
}

func (om *PatchManifest) LogOutput(ctx context.Context, loggerInstance logger.Logger) {
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

func (om *PatchManifest) SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string) {
	file, err := json.Marshal(om.patchManifest)
	if err != nil {
		loggerInstance.ErrorWithCtx(ctx, "Failed to marshal to json",
			"err", err, "path", path)
	}
	err = os.WriteFile(path, file, 0644)
	if err != nil {
		loggerInstance.ErrorWithCtx(ctx, "Failed to write report to file",
			"err", err, "path", path)
	}
}

func readManifestFromFile(path string) (*patchManifest, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var patchManifestInstance *patchManifest
	err = json.Unmarshal(file, &patchManifestInstance)
	if err != nil {
		return nil, err
	}
	return patchManifestInstance, nil
}
