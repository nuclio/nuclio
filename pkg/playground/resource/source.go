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

package resource

import (
	"io/ioutil"
	"net/http"
	"os"
	"path"

	"github.com/nuclio/nuclio/pkg/playground"
	"github.com/nuclio/nuclio/pkg/playground/fixtures"
	"github.com/nuclio/nuclio/pkg/restful"

	"github.com/go-chi/chi"
)

type sourceResource struct {
	*resource
	sourcesDir string
}

// called after initialization
func (sr *sourceResource) OnAfterInitialize() {
	sr.sourcesDir = sr.GetServer().(*playground.Server).GetSourcesDir()

	sr.GetRouter().Get("/{id}", sr.handleGetSource)
	sr.GetRouter().Post("/{id}", sr.handlePostSource)

	// create sources fixtures
	for fixtureName, fixtureContent := range fixtures.Sources {
		sr.create(fixtureName, []byte(fixtureContent))
	}
}

func (sr *sourceResource) GetAll(request *http.Request) map[string]restful.Attributes {
	files, err := ioutil.ReadDir(sr.sourcesDir)
	if err != nil {
		sr.Logger.WarnWith("Failed to read directory", "dir", sr.sourcesDir, "err", err)

		return nil
	}

	resources := map[string]restful.Attributes{}
	for _, file := range files {
		resources[file.Name()] = nil
	}

	return resources
}

// Create creates a source file with a given name
func (sr *sourceResource) create(sourceName string, sourceContent []byte) error {
	sourcePath := sr.getSourcePath(sourceName)

	err := ioutil.WriteFile(sourcePath, sourceContent, os.FileMode(0600))
	if err != nil {
		sr.Logger.WarnWith("Couldn't write source body", "sourcePath", sourcePath, "err", err)

		return err
	}

	return nil
}

func (sr *sourceResource) handleGetSource(responseWriter http.ResponseWriter, request *http.Request) {
	sourceName := chi.URLParam(request, "id")

	// try to read the source
	sourceContent, err := ioutil.ReadFile(sr.getSourcePath(sourceName))
	if err != nil {
		sr.Logger.WarnWith("Couldn't find source", "name", sourceName)
		responseWriter.WriteHeader(http.StatusNotFound)
		return
	}

	responseWriter.Header().Set("Content-Type", "text/plain")

	// write the source
	responseWriter.Write(sourceContent)

	sr.Logger.DebugWith("Returned source", "name", sourceName, "len", len(sourceContent))
}

func (sr *sourceResource) handlePostSource(responseWriter http.ResponseWriter, request *http.Request) {
	sourceName := chi.URLParam(request, "id")

	sourceContent, err := ioutil.ReadAll(request.Body)
	if err != nil {
		sr.Logger.WarnWith("Couldn't read source body", "err", err)
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := sr.create(sourceName, sourceContent); err != nil {
		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseWriter.WriteHeader(http.StatusNoContent)

	sr.Logger.DebugWith("Stored source", "name", sourceName, "len", len(sourceContent))
}

func (sr *sourceResource) getSourcePath(sourceName string) string {
	return path.Join(sr.sourcesDir, sourceName)
}

// register the resource
var sourceResourceInstance = &sourceResource{
	resource: newResource("sources", []restful.ResourceMethod{
		restful.ResourceMethodGetList,
		restful.ResourceMethodCreate,
	}),
}

func init() {
	sourceResourceInstance.Resource = sourceResourceInstance
	sourceResourceInstance.Register(playground.PlaygroundResourceRegistrySingleton)
}
