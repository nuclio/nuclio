package proxier

import (
	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
)

type FunctionStarter struct {
	Logger    *logger.Logger
	kubeClientSet    kubernetes.Interface
}


