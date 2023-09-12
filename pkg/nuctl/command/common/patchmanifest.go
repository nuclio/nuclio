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

	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
)

type PatchManifest struct {
	*patchManifest
	lock sync.Mutex
}

type patchManifest struct {
	Success []string                   `json:"success,omitempty"`
	Skipped []string                   `json:"skipped,omitempty"`
	Failed  map[string]failDescription `json:"failed,omitempty"`
}

type failDescription struct {
	Err       string `json:"error,omitempty"`
	Retryable bool   `json:"retryable"`
}

func NewPatchManifest() *PatchManifest {
	return &PatchManifest{
		lock: sync.Mutex{},
		patchManifest: &patchManifest{
			Success: []string{},
			Skipped: []string{},
			Failed:  make(map[string]failDescription),
		},
	}
}

func NewPatchManifestFromFile(path string) (*PatchManifest, error) {
	parsedManifest, err := readManifestFromFile(path)
	if err != nil {
		return nil, err
	}
	return &PatchManifest{
		patchManifest: parsedManifest,
		lock:          sync.Mutex{},
	}, nil
}

func (m *PatchManifest) AddSuccess(name string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.Success = append(m.Success, name)
}

func (m *PatchManifest) AddSkipped(name string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.Skipped = append(m.Skipped, name)
}

func (m *PatchManifest) AddFailure(name string, err error, retryable bool) {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.Failed[name] = failDescription{
		Err:       err.Error(),
		Retryable: retryable,
	}
}

func (m *PatchManifest) GetSuccess() []string {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.Success
}

func (m *PatchManifest) GetSkipped() []string {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.Skipped
}

func (m *PatchManifest) GetFailed() map[string]failDescription {
	m.lock.Lock()
	defer m.lock.Unlock()

	return m.Failed
}

func (m *PatchManifest) GetRetryableFunctionNames() []string {
	retryable := make([]string, 0)
	for name, failDescription := range m.GetFailed() {
		if failDescription.Retryable {
			retryable = append(retryable, name)
		}
	}
	return retryable
}

func (m *PatchManifest) LogOutput(ctx context.Context, loggerInstance logger.Logger) {
	if len(m.GetSuccess()) > 0 {
		loggerInstance.InfoWithCtx(ctx, "Patched functions successfully",
			"functions", m.GetSuccess())
	}
	if len(m.GetSkipped()) > 0 {
		loggerInstance.InfoWithCtx(ctx, "Skipped functions",
			"functions", m.GetSkipped())
	}
	if len(m.GetFailed()) > 0 {
		for function, failDescription := range m.GetFailed() {
			loggerInstance.ErrorWithCtx(ctx, "Failed to patch function",
				"function", function,
				"err", failDescription.Err,
				"retryable", failDescription.Retryable)
		}
	}
}

func (m *PatchManifest) SaveToFile(ctx context.Context, loggerInstance logger.Logger, path string) {
	file, err := json.Marshal(m.patchManifest)
	if err != nil {
		loggerInstance.ErrorWithCtx(ctx,
			"Failed to marshal report to json",
			"err", err,
			"path", path)
	}
	if err := os.WriteFile(path, file, 0644); err != nil {
		loggerInstance.ErrorWithCtx(ctx,
			"Failed to write report to file",
			"err", err,
			"path", path)
	}
}

func readManifestFromFile(path string) (*patchManifest, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to read patch manifest from file")
	}
	var patchManifestInstance *patchManifest
	err = json.Unmarshal(file, &patchManifestInstance)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to unmarshal patch manifest")
	}
	return patchManifestInstance, nil
}
