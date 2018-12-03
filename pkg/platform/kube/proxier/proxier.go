package proxier

import (
	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
)

type Proxier struct {
	logger           logger.Logger
	namespace        string
	kubeClientSet    kubernetes.Interface
}

func NewProxier(parentLogger logger.Logger, namespace string, kubeClientSet kubernetes.Interface) *Proxier {
	return &Proxier{
		logger: parentLogger.GetChild("proxier"),
		namespace: namespace,
		kubeClientSet: kubeClientSet,
	}
}

func (p *Proxier) Start() error {
	return nil
}
