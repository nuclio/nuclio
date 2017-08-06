package nucliocli

type CommonOptions struct {
	Verbose        bool
	Identifier     string
	Namespace      string
	KubeconfigPath string
	KubeHost       string
	SpecFilePath   string
}
