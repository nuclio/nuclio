package functiontemplates

type FunctionTemplateFetcher interface {
	Fetch() ([]functionTemplate, error)
}
