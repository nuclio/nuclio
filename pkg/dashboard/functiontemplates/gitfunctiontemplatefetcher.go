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
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/ghodss/yaml"
	"github.com/icza/dyno"
	"github.com/nuclio/logger"
	"github.com/rs/xid"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

type GitFunctionTemplateFetcher struct {
	branch     string
	repository string
	logger     logger.Logger
}

func NewGitFunctionTemplateFetcher(parentLogger logger.Logger,
	repository string,
	branch string) (*GitFunctionTemplateFetcher, error) {

	return &GitFunctionTemplateFetcher{
		repository: repository,
		branch:     branch,
		logger:     parentLogger.GetChild("GitFunctionTemplateFetcher"),
	}, nil
}

func (gftf *GitFunctionTemplateFetcher) Fetch() ([]*FunctionTemplate, error) {
	var functionTemplates []*FunctionTemplate

	gftf.logger.DebugWith("Fetching templates from git",
		"repository", gftf.repository,
		"branch", gftf.branch)

	rootTree, err := gftf.getRootTree()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to clone repository")
	}

	// get templates from source tree sha
	functionTemplates, err = gftf.getTemplatesFromGitTree(rootTree, "")
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get templates from source tree sha")
	}

	gftf.logger.DebugWith("Fetched templates from git", "numberOfFunctionTemplates", len(functionTemplates))

	return functionTemplates, nil
}

func (gftf *GitFunctionTemplateFetcher) getRootTree() (*object.Tree, error) {
	gitRepo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:           gftf.repository,
		ReferenceName: plumbing.NewBranchReferenceName(gftf.branch),
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize git repository")
	}

	ref, err := gitRepo.Head()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize git repository (get reference for HEAD)")
	}

	commit, err := gitRepo.CommitObject(ref.Hash())
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize git repository (get commit object)")
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get source tree")
	}

	return tree, nil
}

func (gftf *GitFunctionTemplateFetcher) getTemplatesFromGitTree(rootTree *object.Tree, upperDirName string) ([]*FunctionTemplate, error) {
	var functionTemplates []*FunctionTemplate
	var tree *object.Tree
	var err error

	if upperDirName != "" {
		tree, err = rootTree.Tree(upperDirName)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to get contents of path")
		}
	} else {
		tree = rootTree
	}

	// add template if there is one in current dir
	currentDirTemplate, err := gftf.getTemplateFromDir(tree, upperDirName)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to template from dir")
	}

	// add found template to function templates
	if currentDirTemplate != nil {
		functionTemplates = append(functionTemplates, currentDirTemplate)
	}

	// search recursively in other entries (items in current dir) which are dirs
	for _, entry := range tree.Entries {
		if entry.Mode == filemode.Dir {
			// get subdir templates
			subdirTemplates, err := gftf.getTemplatesFromGitTree(tree, entry.Name)
			if err != nil {
				return nil, errors.Wrap(err, "Failed to get templates from sub directory")
			}

			functionTemplates = append(functionTemplates, subdirTemplates...)
		}
	}

	return functionTemplates, nil
}

func (gftf *GitFunctionTemplateFetcher) getTemplateFromDir(dir *object.Tree, upperDirName string) (*FunctionTemplate, error) {
	currentDirFunctionTemplate := FunctionTemplate{}

	// add dir name as function's Name
	currentDirFunctionTemplate.Name = upperDirName

	if sourceFile, err := gftf.getFirstSourceFile(dir); sourceFile != "" {
		currentDirFunctionTemplate.SourceCode = sourceFile
	} else if err != nil {
		return nil, errors.Wrap(err, "Failed to get and process source file")
	}

	// get function.yaml - error if failed to get its content although it exists
	file, err := gftf.getFileFromTreeEntries(dir, "function.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "Found function.yaml but failed to get its content")
	}

	// if we got functionconfig we're done
	if file != "" {
		err = yaml.Unmarshal([]byte(file), &currentDirFunctionTemplate.FunctionConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshall yaml file function.yaml")
		}

		gftf.enrichFunctionTemplate(&currentDirFunctionTemplate)
		return &currentDirFunctionTemplate, nil
	}

	// get function.yaml.template - error if failed to get its content although it exists
	yamlTemplateFile, yamlValuesFile, err := gftf.getFunctionYAMLTemplateAndValuesFromTreeEntries(dir)

	if err != nil {
		return nil, errors.Wrap(err, "Found function.yaml.template yaml file or "+
			"function.yaml.values yaml file but failed to get its content")
	}

	// if one is set both are set - else getFunctionYAMLTemplateAndValuesFromTreeEntries would have raise an error
	if yamlTemplateFile != "" {
		currentDirFunctionTemplate.FunctionConfigTemplate = yamlTemplateFile

		var values map[string]interface{}
		err := yaml.Unmarshal([]byte(yamlValuesFile), &values)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshall function template's values file")
		}

		for valueName, valueInterface := range values {
			values[valueName] = dyno.ConvertMapI2MapS(valueInterface)
		}
		currentDirFunctionTemplate.FunctionConfigValues = values

		currentDirFunctionTemplate.FunctionConfig = &functionconfig.Config{}

		gftf.replaceSourceCodeInTemplate(&currentDirFunctionTemplate)
		gftf.enrichFunctionTemplate(&currentDirFunctionTemplate)
		return &currentDirFunctionTemplate, nil

	}

	// if we got here no error raised, but we did'nt find files
	gftf.logger.Debug("No function templates found")
	return nil, nil
}

func (gftf *GitFunctionTemplateFetcher) getFunctionYAMLTemplateAndValuesFromTreeEntries(dir *object.Tree) (string, string, error) {
	yamlTemplate, err := gftf.getFileFromTreeEntries(dir, "function.yaml.template")
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to get function.yaml.template")
	}
	gftf.logger.DebugWith("Got function template directory structure from git", "dir", dir)

	yamlValuesFile, err := gftf.getFileFromTreeEntries(dir, "function.yaml.values")

	if err != nil {
		return "", "", errors.Wrap(err, "Found function.yaml.values yaml file but failed to get its content")
	}

	if (yamlTemplate == "") != (yamlValuesFile == "") {
		return "", "", errors.New("Found only one file out of function.yaml.value & function.yaml.template")
	}

	return yamlTemplate, yamlValuesFile, nil
}

func (gftf *GitFunctionTemplateFetcher) getFirstSourceFile(entries *object.Tree) (string, error) {
	iter := entries.Files()
	for {
		file, err := iter.Next()
		if err == io.EOF {
			return "", errors.New("Failed to locate file")
		}

		if !strings.Contains(file.Name, ".yaml") {
			return file.Blob.Hash.String(), nil
		}
	}
	return "", nil
}

func (gftf *GitFunctionTemplateFetcher) getFileFromTreeEntries(entries *object.Tree, filename string) (string, error) {
	iter := entries.Files()
	for {
		file, err := iter.Next()
		if err == io.EOF {
			return "", nil
		}

		if file.Name == filename {

			contents, err := file.Contents()
			if err != nil {
				return "", errors.Wrap(err, "Failed to read file")
			}

			return contents, nil
		}
	}
	return "", nil
}

func (gftf *GitFunctionTemplateFetcher) replaceSourceCodeInTemplate(functionTemplate *FunctionTemplate) {

	// hack: if template writer passed a function source code, reflect it in template by replacing `functionSourceCode: {{ .SourceCode }}`
	replacement := fmt.Sprintf("functionSourceCode: %s",
		base64.StdEncoding.EncodeToString([]byte(functionTemplate.SourceCode)))
	pattern := "functionSourceCode: {{ .SourceCode }}"
	functionTemplate.FunctionConfigTemplate = strings.Replace(functionTemplate.FunctionConfigTemplate,
		pattern,
		replacement,
		1)
}

func (gftf *GitFunctionTemplateFetcher) enrichFunctionTemplate(functionTemplate *FunctionTemplate) {

	// set the source code we got earlier
	functionTemplate.FunctionConfig.Spec.Build.FunctionSourceCode = base64.StdEncoding.EncodeToString(
		[]byte(functionTemplate.SourceCode))

	// set something unique, the UI will ignore everything after `:`, this is par to pre-generated templates
	functionTemplate.FunctionConfig.Meta = functionconfig.Meta{
		Name: functionTemplate.Name + ":" + xid.New().String(),
	}
}
