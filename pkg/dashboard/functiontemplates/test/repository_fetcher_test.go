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

package test

import (
	"os"
	"testing"

	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
)

func TestRepositoryFetcher(t *testing.T) {
	githuAPItoken := os.Getenv("PROVAZIO_GITHUB_API_TOKEN")
	supportedSuffixes := []string{".go", ".py"}

	templateFetcher, err := functiontemplates.NewGithubFunctionTemplateFetcher("nuclio-templates", "ilaykav", "master", githuAPItoken, supportedSuffixes)
	if err != nil {
		t.Error(err)
	}

	repositoryFetcher, err := functiontemplates.NewRepositoryTemplateFetcher([]functiontemplates.FunctionTemplateFetcher{templateFetcher})
	if err != nil {
		t.Error(err)
	}

	t.Logf("Repository fetcher ended", "templates", repositoryFetcher.Templates)
}
