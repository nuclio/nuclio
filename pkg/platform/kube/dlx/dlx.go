package dlx

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"net/http"
)

type Configuration struct {
	URL string
	ReadBufferSize int
}

type Proxier struct {
	logger           logger.Logger
	namespace        string
	kubeClientSet    kubernetes.Interface
	nuclioClientSet        nuclioio_client.Interface
	configuration Configuration
	handler      Handler
}

func NewProxier(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioio_client.Interface,
	config Configuration) (*Proxier, error) {
	dlxLogger := parentLogger.GetChild("dlx")
	functionStarter, err := NewFunctionStarter(dlxLogger, namespace, nuclioClientSet)
	if err != nil {
		return nil, errors.Wrap(err,"Failed to create function starter")
	}
	handler, err := NewHandler(dlxLogger, functionStarter)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create handler")
	}

	dlx := &Proxier{
		logger: dlxLogger,
		namespace: namespace,
		kubeClientSet: kubeClientSet,
		configuration: config,
		handler: handler,
	}
	return dlx, nil
}

func (p *Proxier) Start() error {
	p.logger.InfoWith("Starting",
		"listenAddress", p.configuration.URL,
		"readBufferSize", p.configuration.ReadBufferSize)

	http.HandleFunc("/", p.handler.requestHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		return err
	}
	return nil
}
