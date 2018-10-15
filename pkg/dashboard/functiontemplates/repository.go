package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/errors"
)

type repoFunctionTemplateFetcher struct {
	Templates []FunctionTemplate
	fetchers  []FunctionTemplateFetcher
	FunctionTemplateFetcher
}

func NewRepositoryTemplateFetcher(fetchers []FunctionTemplateFetcher) (*repoFunctionTemplateFetcher, error) {
	var templates []FunctionTemplate

	for _, fetcher := range fetchers {
		currentFetcherTemplates, err := fetcher.Fetch()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to fetch one of given templateFetchers")
		} else {
			templates = append(templates, currentFetcherTemplates...)
		}
	}

	return &repoFunctionTemplateFetcher{
		Templates: templates,
		fetchers:  fetchers,
	}, nil
}

func (rftf *repoFunctionTemplateFetcher) Fetch() ([]FunctionTemplate, error) {
	var templates []FunctionTemplate

	for _, fetcher := range rftf.fetchers {
		currentFetcherTemplates, err := fetcher.Fetch()
		if err != nil {
			return nil, errors.Wrap(err, "Failed to fetch one of repo-fetcher templateFetchers")
		} else {
			templates = append(templates, currentFetcherTemplates...)
		}
	}

	return templates, nil
}
