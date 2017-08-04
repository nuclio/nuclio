package nucliocli

type CommonOptions struct {
	Verbose        bool
	Namespace      string
	KubeconfigPath string
	KubeHost       string
	SpecFilePath   string
}
