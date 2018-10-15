package functiontemplates

import (
	"context"
	"encoding/base64"
	"strings"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v2"
)

func NewGithubFunctionTemplateFetcher(repository string, owner string, branch string, githubAccessToken string, supportedSourceTypesSuffixes []string) (*GithubFunctionTemplateFetcher, error) {
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAccessToken},
	)
	tc := oauth2.NewClient(context.TODO(), tokenSource)

	client := github.NewClient(tc)

	return &GithubFunctionTemplateFetcher{
		Repository:                   repository,
		Owner:                        owner,
		Branch:                       branch,
		githubAPIClient:              client,
		supportedSourceTypesSuffixes: supportedSourceTypesSuffixes,
	}, nil
}

func (gftf *GithubFunctionTemplateFetcher) Fetch() ([]FunctionTemplate, error) {
	var functionTemplates []FunctionTemplate

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

	return functionTemplates, nil
}

func (gftf *GithubFunctionTemplateFetcher) getTemplatesFromGithubSHA(treeSha string, upperDirName string) ([]FunctionTemplate, error) {
	var functionTemplates []FunctionTemplate

	// get subdir items from github sha
	// recursive set to false because when set to true it may not give all items in dir (https://developer.github.com/v3/git/trees/#get-a-tree-recursively)
	tree, _, err := gftf.githubAPIClient.Git.GetTree(context.TODO(), gftf.Owner, gftf.Repository, treeSha, false)
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
		functionTemplates = append(functionTemplates, *currentDirTemplate)
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
	functionTemplate := FunctionTemplate{}

	// add dir name as function's Name
	functionTemplate.Name = upperDirName

	if sourceFile, err := gftf.getFirstSourceFile(dir, gftf.supportedSourceTypesSuffixes); sourceFile != nil {
		functionTemplate.SourceCode = *sourceFile
	} else if err != nil {
		return nil, errors.Wrap(err, "Failed to get and process source file")
	}

	// get function.yaml - error if failed to get its content although it exists
	if file, err := gftf.getFileFromTreeEntries(dir, "function.yaml"); err != nil {
		return nil, errors.Wrap(err, "Found function.yaml but failed to get its content")
	} else if file != nil {
		err = yaml.Unmarshal([]byte(*file), &functionTemplate.FunctionConfig)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to unmarshall yaml file function.yaml")
		}

		// get function.yaml.template - error if failed to get its content although it exists
	} else if yemlTemplateFile, err := gftf.getFileFromTreeEntries(dir, "function.yaml.template"); err != nil {

		// found but failes to get
		return nil, errors.Wrap(err, "Found function.yaml.template yaml file but failed to get its content")
	} else if yemlTemplateFile != nil {

		// set functionTemplate.FunctionConfigTemplate as yaml template file which has been found
		functionTemplate.FunctionConfigTemplate = *yemlTemplateFile
		yamlValuesFile, err := gftf.getFileFromTreeEntries(dir, "function.yaml.values")

		// if file does not exist don't raise error
		if err != nil {
			return nil, errors.Wrap(err, "Found function.yaml.values yaml file but failed to get its content")
		} else if yamlValuesFile == nil {
			return nil, nil
		}

		// set functionTemplate.FunctionConfigTemplate as yaml template file which has been found
		functionTemplate.FunctionConfigValues = *yamlValuesFile
	} else {

		// no template found
		return nil, nil
	}

	// is &functionTemplate nil in case of empty functionTemplate
	return &functionTemplate, nil
}

func (gftf *GithubFunctionTemplateFetcher) getSourceTreeSha() (string, error) {
	branch, _, err := gftf.githubAPIClient.Repositories.GetBranch(context.TODO(), gftf.Owner, gftf.Repository, gftf.Branch)

	if err != nil {
		return "", errors.Wrap(err, "Failed to get source tree")
	}

	return *branch.GetCommit().SHA, nil
}

func (gftf *GithubFunctionTemplateFetcher) getFirstSourceFile(entries []github.TreeEntry, supportedFileEndings []string) (*string, error) {
	for _, entry := range entries {
		for _, supportedFileEnding := range supportedFileEndings {
			if *entry.Type == "blob" && strings.HasSuffix(*entry.Path, supportedFileEnding) {
				fileContent, err := gftf.getBlobContentFromSha(*entry.SHA)
				if err != nil {
					return nil, errors.Wrap(err, "Failed to get content of blob")
				}
				return fileContent, nil
			}
		}
	}
	return nil, nil
}

func (gftf *GithubFunctionTemplateFetcher) getBlobContentFromSha(sha string) (*string, error) {
	blob, _, err := gftf.githubAPIClient.Git.GetBlob(context.TODO(), gftf.Owner, gftf.Repository, sha)
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

//gftf.Fetch()
