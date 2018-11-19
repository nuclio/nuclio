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

package functiontemplates

import (
	"context"
	"encoding/base64"
	"github.com/nuclio/logger"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/google/go-github/github"
	"github.com/icza/dyno"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

type GithubFunctionTemplateFetcher struct {
	branch          string
	owner           string
	repository      string
	githubAPIClient *github.Client
	logger          logger.Logger
}

func NewGithubFunctionTemplateFetcher(parentLogger logger.Logger,
	repository string,
	owner string,
	branch string,
	githubAccessToken string) (*GithubFunctionTemplateFetcher, error) {

	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAccessToken},
	)
	tc := oauth2.NewClient(context.TODO(), tokenSource)

	client := github.NewClient(tc)

	return &GithubFunctionTemplateFetcher{
		repository:      repository,
		owner:           owner,
		branch:          branch,
		githubAPIClient: client,
		logger:          parentLogger.GetChild("GithubFunctionTemplateFetcher"),
	}, nil
}

func (gftf *GithubFunctionTemplateFetcher) Fetch() ([]*FunctionTemplate, error) {
	var functionTemplates []*FunctionTemplate

	gftf.logger.DebugWith("Fetching templates from github",
		"owner",
		gftf.owner,
		"repository",
		gftf.repository)

	// get sha of root of source tree
	treeSha, err := gftf.getSourceTreeSha()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get source tree sha")
	}

	// get templates from source tree sha
	functionTemplates, err = gftf.getTemplatesFromGithubSHA(treeSha, "")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get templates from source tree sha")
	}

	gftf.logger.DebugWith("Fetched templates from github", "numberOfFunctionTemplates", len(functionTemplates))

	return functionTemplates, nil
}

func (gftf *GithubFunctionTemplateFetcher) getTemplatesFromGithubSHA(treeSha string, upperDirName string) ([]*FunctionTemplate, error) {
	var functionTemplates []*FunctionTemplate

	// get subdir items from github sha
	// recursive set to false because when set to true it may not give all items in dir (https://developer.github.com/v3/git/trees/#get-a-tree-recursively)
	tree, _, err := gftf.githubAPIClient.Git.GetTree(context.TODO(), gftf.owner, gftf.repository, treeSha, false)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get source tree with GetTree go-github function")
	}

	// add template if there is one in current dir
	currentDirTemplate, err := gftf.getTemplateFromDir(tree.Entries, upperDirName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to template from dir")
	}

	// add found template to function templates
	if currentDirTemplate != nil {
		functionTemplates = append(functionTemplates, currentDirTemplate)
	}

	// search recursively in other entries (items in current dir) which are dirs
	for _, entry := range tree.Entries {
		if *entry.Type == "tree" {
			// get subdir templates
			subdirTemplates, err := gftf.getTemplatesFromGithubSHA(*entry.SHA, *entry.Path)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get templates from sub directory")
			}

			functionTemplates = append(functionTemplates, subdirTemplates...)
		}
	}

	return functionTemplates, nil
}

func (gftf *GithubFunctionTemplateFetcher) getTemplateFromDir(dir []github.TreeEntry, upperDirName string) (*FunctionTemplate, error) {
	currentDirFunctionTemplate := FunctionTemplate{}

	// add dir name as function's Name
	currentDirFunctionTemplate.Name = upperDirName

	if sourceFile, err := gftf.getFirstSourceFile(dir); sourceFile != nil {
		currentDirFunctionTemplate.SourceCode = *sourceFile
	} else if err != nil {
		return nil, errors.Wrap(err, "Failed to get and process source file")
	}

	// get function.yaml - error if failed to get its content although it exists
	file, err := gftf.getFileFromTreeEntries(dir, "function.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "Found function.yaml but failed to get its content")
	}

	// if we got functionconfig we're done
	if file != nil {
		err = yaml.Unmarshal([]byte(*file), &currentDirFunctionTemplate.FunctionConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshall yaml file function.yaml")
		}

		return &currentDirFunctionTemplate, nil
	}

	// get function.yaml.template - error if failed to get its content although it exists
	yamlTemplateFile, yamlValuesFile, err := gftf.getFunctionYAMLTemplateAndValuesFromTreeEntries(dir)

	if err != nil {
		return nil, errors.Wrap(err, "Found function.yaml.template yaml file or "+
			"function.yaml.values yaml file but failed to get its content")
	}

	// if one is set both are set - else getFunctionYAMLTemplateAndValuesFromTreeEntries would have raise an error
	if yamlTemplateFile != nil {
		currentDirFunctionTemplate.FunctionConfigTemplate = *yamlTemplateFile

		var values map[string]interface{}
		err := yaml.Unmarshal([]byte(*yamlValuesFile), &values)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshall function template's values file")
		}

		for valueName, valueInterface := range values {
			values[valueName] = dyno.ConvertMapI2MapS(valueInterface)
		}
		currentDirFunctionTemplate.FunctionConfigValues = values

		return &currentDirFunctionTemplate, nil

	}

	// if we got here no error raised, but we did'nt find files
	return nil, nil
}

func (gftf *GithubFunctionTemplateFetcher) getFunctionYAMLTemplateAndValuesFromTreeEntries(dir []github.TreeEntry) (*string, *string, error) {
	yamlTemplate, err := gftf.getFileFromTreeEntries(dir, "function.yaml.template")
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get function.yaml.template")
	}
	gftf.logger.DebugWith("Got function template directory structure from github", "dir", dir)

	yamlValuesFile, err := gftf.getFileFromTreeEntries(dir, "function.yaml.values")

	if err != nil {
		return nil, nil, errors.Wrap(err, "Found function.yaml.values yaml file but failed to get its content")
	}

	if (yamlTemplate == nil) != (yamlValuesFile == nil) {
		return nil, nil, errors.New("Could found only one file out of function.yaml.value & function.yaml.template")
	}

	return yamlTemplate, yamlValuesFile, nil
}

func (gftf *GithubFunctionTemplateFetcher) getSourceTreeSha() (string, error) {
	branch, _, err := gftf.githubAPIClient.Repositories.GetBranch(context.TODO(), gftf.owner, gftf.repository, gftf.branch)

	if err != nil {
		return "", errors.Wrap(err, "Failed to get source tree")
	}

	return *branch.GetCommit().SHA, nil
}

func (gftf *GithubFunctionTemplateFetcher) getFirstSourceFile(entries []github.TreeEntry) (*string, error) {
	for _, entry := range entries {
		if *entry.Type == "blob" && !strings.Contains(*entry.Path, ".yaml") {
			fileContent, err := gftf.getBlobContentFromSha(*entry.SHA)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get content of blob")
			}
			return fileContent, nil
		}
	}
	return nil, nil
}

func (gftf *GithubFunctionTemplateFetcher) getBlobContentFromSha(sha string) (*string, error) {
	blob, _, err := gftf.githubAPIClient.Git.GetBlob(context.TODO(), gftf.owner, gftf.repository, sha)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get file content using githubAPI")
	}
	if *blob.Encoding != "base64" {
		return nil, errors.New("Failed to decode blob's content - cannot decode not base64-encoded files")
	}
	blobContent, err := base64.StdEncoding.DecodeString(*blob.Content)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to decode blob's content with base64 stdDecoder")
	}
	blobContentString := string(blobContent)
	return &blobContentString, nil
}

func (gftf *GithubFunctionTemplateFetcher) getFileFromTreeEntries(entries []github.TreeEntry, filename string) (*string, error) {
	for _, entry := range entries {
		if *entry.Path == filename {
			blobContent, err := gftf.getBlobContentFromSha(*entry.SHA)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get content of blob")
			}
			return blobContent, nil
		}
	}
	return nil, nil
}
