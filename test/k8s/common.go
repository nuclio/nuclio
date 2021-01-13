package k8s

import "github.com/nuclio/nuclio/pkg/common"

func GetDefaultIngressHost() string {
	defaultTestAPIGatewayHost := common.GetEnvOrDefaultString("NUCTL_TEST_DEFAULT_INGRESS_HOST", "")
	if defaultTestAPIGatewayHost != "" {
		return defaultTestAPIGatewayHost
	}

	// select host address according to system's kubernetes runner (minikube / docker-for-mac)
	if common.GetEnvOrDefaultString("MINIKUBE_HOME", "") != "" {
		return "host.minikube.internal"
	}

	return "kubernetes.docker.internal"
}
