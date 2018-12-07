package dlx

import (
	"net/http"

	"github.com/nuclio/nuclio/pkg/errors"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/logger"
	"k8s.io/client-go/kubernetes"
)

type Configuration struct {
	URL string
}

type DLX struct {
	logger          logger.Logger
	namespace       string
	kubeClientSet   kubernetes.Interface
	nuclioClientSet nuclioio_client.Interface
	configuration   Configuration
	handler         Handler
}

func NewDLX(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioio_client.Interface,
	config Configuration) (*DLX, error) {
	dlxLogger := parentLogger.GetChild("dlx")
	functionStarter, err := NewFunctionStarter(dlxLogger, namespace, nuclioClientSet)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create function starter")
	}

	handler, err := NewHandler(dlxLogger, functionStarter)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create handler")
	}

	dlx := &DLX{
		logger:        dlxLogger,
		namespace:     namespace,
		kubeClientSet: kubeClientSet,
		configuration: config,
		handler:       handler,
	}
	return dlx, nil
}

func (p *DLX) Start() error {
	p.logger.InfoWith("Starting",
		"listenAddress", p.configuration.URL)

	http.HandleFunc("/", p.handler.handleFunc)
	if err := http.ListenAndServe(p.configuration.URL, nil); err != nil {
		return err
	}
	return nil
}
