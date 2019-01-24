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
	"io"
	"strings"

	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

type GitFunctionTemplateFetcher struct {
	BaseFunctionTemplateFetcher

	ref        string
	repository string
	logger     logger.Logger
}

func NewGitFunctionTemplateFetcher(parentLogger logger.Logger,
	repository string,
	ref string) (*GitFunctionTemplateFetcher, error) {

	return &GitFunctionTemplateFetcher{
		repository: repository,
		ref:        ref,
		logger:     parentLogger.GetChild("GitFunctionTemplateFetcher"),
	}, nil
}

func (gftf *GitFunctionTemplateFetcher) Fetch() ([]*FunctionTemplate, error) {
	var functionTemplates []*FunctionTemplate

	gftf.logger.DebugWith("Fetching templates from git",
		"repository", gftf.repository,
		"ref", gftf.ref)

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
	referenceName := plumbing.ReferenceName(gftf.ref)
	gitRepo, err := git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
		URL:           gftf.repository,
		ReferenceName: referenceName,
		Depth:         1,
	})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to initialize git repository")
	}

	// don't try to do any symbolic resolving
	ref, err := gitRepo.Reference(referenceName, false)
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
	functionTemplateFileContents := FunctionTemplateFileContents{}
	var currentDirFunctionTemplate *FunctionTemplate

	if sourceFile, err := gftf.getFirstSourceFile(dir); sourceFile != "" {
		functionTemplateFileContents.Code = sourceFile
	} else if err != nil {
		return nil, errors.Wrap(err, "Failed to get and process source file")
	}

	// get function.yaml - error if failed to get its content although it exists
	file, err := gftf.getFileFromTreeEntries(dir, "function.yaml")
	if err != nil {
		return nil, errors.Wrap(err, "Found function.yaml but failed to get its content")
	}

	if file != "" {

		// if the template is of the first type (with function.yaml file only)
		functionTemplateFileContents.TemplateAndValues = file
	} else {

		// if the template is of the second type (with function.yaml.template and function.yaml.values files)
		yamlTemplateFile, yamlValuesFile, err := gftf.getFunctionYAMLTemplateAndValuesFromTreeEntries(dir)
		if err != nil {
			return nil, errors.Wrap(err, "Found function.yaml.template yaml file or "+
				"function.yaml.values yaml file but failed to get its content")
		}

		functionTemplateFileContents.Template = yamlTemplateFile
		functionTemplateFileContents.Values = yamlValuesFile
	}

	currentDirFunctionTemplate, err = gftf.createFunctionTemplate(functionTemplateFileContents, upperDirName)
	if err != nil {
		gftf.logger.WarnWith("Failed to create function template", "err", err)
		return nil, nil
	}

	return currentDirFunctionTemplate, nil
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
			contents, err := file.Contents()
			if err != nil {
				return "", errors.Wrap(err, "Failed to read file contents")
			}
			return contents, nil
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
