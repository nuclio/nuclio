package nuclio

type Context struct {
	Logger      Logger
	DataBinding map[string]DataBinding
}
