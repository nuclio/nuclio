package dlx

import (
	"fmt"
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/valyala/fasthttp"
	"k8s.io/client-go/kubernetes"
	"net/http"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	"net/http/httputil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/url"
	"sync"
	"time"
)

type requestResponseMathcer struct {
	request *http.Request
	response http.ResponseWriter
	responseChannel chan error
}

type functionSinksMap map[string]chan requestResponseMathcer

type FunctionStarter struct {
	logger                 logger.Logger
	kubeClientSet          kubernetes.Interface
	nuclioClientSet nuclioio_client.Interface
	namespace string
	functionSinksMap functionSinksMap
	functionSinkMutex      sync.Mutex
}

func NewFunctionStarter(parentLogger logger.Logger,
	namespace string,
	kubeClientSet kubernetes.Interface,
	nuclioClientSet nuclioio_client.Interface) (*FunctionStarter, error) {
	return &FunctionStarter{
		logger:                 parentLogger.GetChild("function-starter"),
		kubeClientSet:          kubeClientSet,
		functionSinksMap: make(functionSinksMap),
		nuclioClientSet: nuclioClientSet,
		namespace: namespace,
	}, nil
}

func (f *FunctionStarter) GetOrCreateFunctionSink(request *http.Request, response http.ResponseWriter, responseChannel chan error) {
	f.functionSinkMutex.Lock()
	defer f.functionSinkMutex.Unlock()

	target := request.Header.Get("X-nuclio-target")

	matcher := requestResponseMathcer{
		request: request,
		response: response,
		responseChannel: responseChannel,
	}
	if _, found := f.functionSinksMap[string(target)]; found {
		functionSinkChannel := f.functionSinksMap[string(target)]

		// do it in a new routine to free the lock
		go func() {functionSinkChannel <- matcher}()
	} else {

		// for the next requests coming in
		functionSinkChannel := make(chan requestResponseMathcer)
		f.functionSinksMap[string(target)] = functionSinkChannel
		f.logger.Debug("Created sink")

		go f.startFunction(functionSinkChannel, string(target))
		go func() {functionSinkChannel <- matcher}()
	}
}

func (f *FunctionStarter) updateFunctionStatus(functionName string) {
	function, err := f.nuclioClientSet.NuclioV1beta1().Functions(f.namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		f.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
		return
	}

	function.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	_, err = f.nuclioClientSet.NuclioV1beta1().Functions(f.namespace).Update(function)
	if err != nil {
		f.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
		return
	}
}

func (f *FunctionStarter) startFunction(c chan requestResponseMathcer, target string) {
	functionName := target

	// TODO proper parse
	f.logger.Debug("Starting function")
	f.updateFunctionStatus(target)
	for {
		function, err := f.nuclioClientSet.NuclioV1beta1().Functions(f.namespace).Get(functionName, metav1.GetOptions{})
		if err != nil {
			f.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
			return
		}
		f.logger.DebugWith("Started function", "state", function.Status.State)
		if function.Status.State != functionconfig.FunctionStateReady {
			time.Sleep(time.Second*5)
		} else {
			break
		}
	}

	target = fmt.Sprintf("http://%s:%d", target, 8080)
	tc := time.After(1*time.Minute)
	for {
		select {
		case matcher := <-c:
			f.logger.DebugWith("Got on channel", "target", target)
			_, err := f.sendRequest(matcher.request, matcher.response, target)
			if err != nil {
				f.logger.WarnWith("Got error when sending request", "err", err)
			} else {
				// release the responder
				f.logger.Debug("Sending")
				matcher.responseChannel <- err
			}
		case <- tc:
			f.logger.Debug("Releasing function sink")
			f.functionSinkMutex.Lock()
			delete(f.functionSinksMap, functionName)
			f.functionSinkMutex.Unlock()
		}
	}

}

func (f *FunctionStarter) sendRequest(req *http.Request, res http.ResponseWriter, target string) (*fasthttp.Response, error) {
	var err error
	// parse the url
	targeUrl, _ := url.Parse(target)

	// create the reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(targeUrl)

	// Update the headers to allow for SSL redirection
	req.URL.Host = targeUrl.Host
	req.URL.Scheme = targeUrl.Scheme
	req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
	req.Host = targeUrl.Host

	proxy.ServeHTTP(res, req)
	if err != nil {
		f.logger.WarnWith("Got error", "err", err)
		return nil, err
	}
	return nil, nil
}
