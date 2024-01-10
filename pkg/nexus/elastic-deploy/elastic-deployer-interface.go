package elastic_deploy

type ElasticDeployer interface {
	Start(functionName string) error
	Pause(functionName string) error
	IsRunning(functionName string) bool
	Initialize()
}
