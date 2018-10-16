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

type githubFunctionTemplateFetcher struct {
	Branch          string
	Owner           string
	Repository      string
	githubAPIClient *github.Client
	FunctionTemplateFetcher
}

func NewGithubFunctionTemplateFetcher(repository string, owner string, branch string, githubAccessToken string) (*githubFunctionTemplateFetcher, error) {
	tokenSource := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAccessToken},
	)
	tc := oauth2.NewClient(context.TODO(), tokenSource)

	client := github.NewClient(tc)

	return &githubFunctionTemplateFetcher{
		Repository:      repository,
		Owner:           owner,
		Branch:          branch,
		githubAPIClient: client,
	}, nil
}

func (gftf *githubFunctionTemplateFetcher) Fetch() ([]functionTemplate, error) {
	var functionTemplates []functionTemplate

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

func (gftf *githubFunctionTemplateFetcher) getTemplatesFromGithubSHA(treeSha string, upperDirName string) ([]functionTemplate, error) {
	var functionTemplates []functionTemplate

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

func (gftf *githubFunctionTemplateFetcher) getTemplateFromDir(dir []github.TreeEntry, upperDirName string) (*functionTemplate, error) {
	currentDirFunctionTemplate := functionTemplate{}

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
	yemlTemplateFile, yamlValuesFile, err := gftf.getFunctionYAMLTemplateAndValuesFromTreeEntries(dir)

	if err != nil {
		return nil, errors.Wrap(err, "Found function.yaml.template yaml file or "+
			"function.yaml.values yaml file but failed to get its content")
	}

	// if one is set both are set - else getFunctionYAMLTemplateAndValuesFromTreeEntries would have raise an error
	if yemlTemplateFile != nil {
		currentDirFunctionTemplate.FunctionConfigTemplate = *yemlTemplateFile
		currentDirFunctionTemplate.FunctionConfigValues = *yamlValuesFile

		return &currentDirFunctionTemplate, nil

	}

	// if we got here no error raised, but we did'nt find files
	return nil, nil
}

func (gftf *githubFunctionTemplateFetcher) getFunctionYAMLTemplateAndValuesFromTreeEntries(dir []github.TreeEntry) (*string, *string, error) {
	yamlTemplate, err := gftf.getFileFromTreeEntries(dir, "function.yaml.template")
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to get function.yaml.template")
	}

	yamlValuesFile, err := gftf.getFileFromTreeEntries(dir, "function.yaml.values")

	if err != nil {
		return nil, nil, errors.Wrap(err, "Found function.yaml.values yaml file but failed to get its content")
	}

	if (yamlTemplate == nil) != (yamlValuesFile == nil) {
		return nil, nil, errors.New("Could found only one file out of function.yaml.value & function.yaml.template")
	}

	return yamlTemplate, yamlValuesFile, nil
}

func (gftf *githubFunctionTemplateFetcher) getSourceTreeSha() (string, error) {
	branch, _, err := gftf.githubAPIClient.Repositories.GetBranch(context.TODO(), gftf.Owner, gftf.Repository, gftf.Branch)

	if err != nil {
		return "", errors.Wrap(err, "Failed to get source tree")
	}

	return *branch.GetCommit().SHA, nil
}

func (gftf *githubFunctionTemplateFetcher) getFirstSourceFile(entries []github.TreeEntry) (*string, error) {
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

func (gftf *githubFunctionTemplateFetcher) getBlobContentFromSha(sha string) (*string, error) {
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

func (gftf *githubFunctionTemplateFetcher) getFileFromTreeEntries(entries []github.TreeEntry, filename string) (*string, error) {
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
