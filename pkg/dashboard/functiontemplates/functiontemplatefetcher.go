package functiontemplates

type FunctionTemplateFetcher interface {
	Fetch() ([]FunctionTemplate, error)
}
