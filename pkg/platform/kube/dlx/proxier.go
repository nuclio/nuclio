package dlx

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/valyala/fasthttp"
	"k8s.io/client-go/kubernetes"
)

type Configuration struct {
	URL string
	ReadBufferSize int
}

type Proxier struct {
	logger           logger.Logger
	namespace        string
	kubeClientSet    kubernetes.Interface
	configuration Configuration
	handler      Handler
}

func NewProxier(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	config Configuration) (*Proxier, error) {
	dlxLogger := parentLogger.GetChild("dlx")
	functionStarter, err := NewFunctionStarter(dlxLogger, kubeClientSet)
	if err != nil {
		return nil, errors.Wrap(err,"Failed to create function starter")
	}
	handler, err := NewHandler(functionStarter)
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

	s := &fasthttp.Server{
		Handler:        p.handler.requestHandler,
		Name:           "nuclio-dlx",
		ReadBufferSize: p.configuration.ReadBufferSize,
	}

	// start listening
	go s.ListenAndServe(p.configuration.URL) // nolint: errcheck
	return nil
}

