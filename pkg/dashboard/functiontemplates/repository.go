package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/errors"
)

func NewRepositoryTemplateFetcher(fetchers []FunctionTemplateFetcher) (*RepoFunctionTemplateFetcher, error) {
	var templates []FunctionTemplate

	for _, fetcher := range fetchers {
		currentFetcherTemplates, err := fetcher.Fetch()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to fetch one of given templateFetchers")
		}
		templates = append(templates, currentFetcherTemplates...)
	}

	return &RepoFunctionTemplateFetcher{
		Templates: templates,
		fetchers:  fetchers,
	}, nil
}

func (rftf *RepoFunctionTemplateFetcher) Fetch() ([]FunctionTemplate, error) {
	var templates []FunctionTemplate

	for _, fetcher := range rftf.fetchers {
		currentFetcherTemplates, err := fetcher.Fetch()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to fetch one of repo-fetcher templateFetchers")
		}
		templates = append(templates, currentFetcherTemplates...)
	}

	return templates, nil
}
