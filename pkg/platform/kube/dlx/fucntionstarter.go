package dlx

import (
	"github.com/nuclio/logger"
	"github.com/valyala/fasthttp"
	"k8s.io/client-go/kubernetes"
	"sync"
	"time"
)

type functionSinksMap map[string]chan *fasthttp.Request

type FunctionStarter struct {
	logger    logger.Logger
	kubeClientSet    kubernetes.Interface
	functionSinks    functionSinksMap
	functionSinkMutex sync.Mutex
}

func NewFunctionStarter(parentLogger logger.Logger, kubeClientSet kubernetes.Interface) (*FunctionStarter, error) {
	return &FunctionStarter{
		logger: parentLogger.GetChild("function-starter"),
		kubeClientSet: kubeClientSet,
		functionSinks: make(functionSinksMap),
	}, nil
}

func (f *FunctionStarter) SendRequestGetResponse(requestCtx *fasthttp.Request, responseChannel chan *fasthttp.Response) {
	f.getOrCreateFunctionSink(requestCtx, responseChannel)
}

func (f *FunctionStarter) getOrCreateFunctionSink(request *fasthttp.Request, responseChannel chan *fasthttp.Response) {
	f.functionSinkMutex.Lock()
	defer f.functionSinkMutex.Unlock()

	target := request.Header.Peek("X-nuclio-target")
	if functionSinkChannel, found := f.functionSinks[string(target)]; !found {
		functionSinkChannel <- request
	} else {

		// for the next requests coming in
		f.functionSinks[string(target)] = make(chan *fasthttp.Request)
		functionSinkChannel <- request
		f.startFunction(string(target), responseChannel)
	}
}


func (f *FunctionStarter) startFunction(functionName string, responseChannel chan *fasthttp.Response) {
	f.logger.Debug("Starting function")
	time.Sleep(time.Duration(time.Second * 1))
	f.logger.Debug("Started function")

	c := f.functionSinks[functionName]
	for {
		select {
		case request := <-c:
			response, err := f.sendRequest(request)
			if err != nil {
				f.logger.WarnWith("Got error when sending request", "err", err)
			}
			responseChannel <- response
		}
	}
}

func (f *FunctionStarter) sendRequest(request *fasthttp.Request) (*fasthttp.Response, error) {
	var response *fasthttp.Response
	err := fasthttp.Do(request, response)
	if err != nil {
		f.logger.WarnWith("Got error", "err", err)
		return nil, err
	}
	f.logger.DebugWith("Got response", "status", response.StatusCode())
	return response, nil
}
