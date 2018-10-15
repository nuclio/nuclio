package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/google/go-github/github"
)

type FunctionTemplate struct {
	Name                   string
	DisplayName            string
	SourceCode             string
	FunctionConfigTemplate string
	FunctionConfigValues   string
	FunctionConfig         functionconfig.Config
}

type GithubFunctionTemplateFetcher struct {
	Branch                       string
	Owner                        string
	Repository                   string
	githubAPIClient              *github.Client
	supportedSourceTypesSuffixes []string
	FunctionTemplateFetcher
}

type RepoFunctionTemplateFetcher struct {
	Templates []FunctionTemplate
	fetchers  []FunctionTemplateFetcher
	FunctionTemplateFetcher
}
