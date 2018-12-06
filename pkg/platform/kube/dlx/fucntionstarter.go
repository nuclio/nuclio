package dlx

import (
	"github.com/nuclio/logger"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"sync"
	"time"
)

type responseChannel chan FunctionStatusResult
type functionSinksMap map[string]chan responseChannel

type nuclioWrapper interface {
	waitFunctionReadiness (string, chan bool)
	updateFunctionStatus(string)
}

type nuclioActioner struct {
	nuclioClientSet nuclioio_client.Interface
	functionStarter *FunctionStarter
}

type FunctionStarter struct {
	logger                 logger.Logger
	nuclioClientSet nuclioio_client.Interface
	namespace string
	functionSinksMap functionSinksMap
	functionSinkMutex      sync.Mutex
	nuclioActioner nuclioWrapper
	functionReadinnesTimeout time.Duration
}

type Endpoint struct {
	Host string
	Port int
	Path string
}

type FunctionStatusResult struct {
	FunctionName string
	FunctionEndpoint Endpoint
	Status int
	Error error
}

func NewFunctionStarter(parentLogger logger.Logger,
	namespace string,
	nuclioClientSet nuclioio_client.Interface) (*FunctionStarter, error) {
	fs := &FunctionStarter{
		logger:                 parentLogger.GetChild("function-starter"),
		functionSinksMap: make(functionSinksMap),
		namespace: namespace,
		functionReadinnesTimeout: time.Duration(1*time.Minute),
	}
	fs.nuclioActioner = &nuclioActioner{
		nuclioClientSet: nuclioClientSet,
		functionStarter: fs,
	}
	return fs, nil
}

func (f *FunctionStarter) GetOrCreateFunctionSink(originalTarget string,
	handlerResponseChannel responseChannel) {
	f.functionSinkMutex.Lock()
	defer f.functionSinkMutex.Unlock()

	if _, found := f.functionSinksMap[originalTarget]; found {
		functionSinkChannel := f.functionSinksMap[originalTarget]

		// do it in a new routine to free the lock
		go func() {functionSinkChannel <- handlerResponseChannel}()
	} else {

		// for the next requests coming in
		functionSinkChannel := make(chan responseChannel)
		f.functionSinksMap[originalTarget] = functionSinkChannel
		f.logger.Debug("Created sink")

		go f.startFunction(functionSinkChannel, originalTarget)
		go func() {functionSinkChannel <- handlerResponseChannel}()
	}
}

func (f *FunctionStarter) startFunction(functionSinkChannel chan responseChannel, target string) {
	var resultStatus FunctionStatusResult

	// simple for now
	functionName := target

	f.logger.Debug("Starting function")
	f.nuclioActioner.updateFunctionStatus(functionName)
	functionReadyChannel := make(chan bool, 1)
	defer close(functionReadyChannel)

	go f.nuclioActioner.waitFunctionReadiness(functionName, functionReadyChannel)

	select {
	case <- time.After(f.functionReadinnesTimeout):
		f.logger.WarnWith("Timed out waiting for function to be ready", "function", functionName)
		defer f.deleteFunctionSink(functionName)
		resultStatus = FunctionStatusResult{
			Error: errors.New("Timed out waiting for function to be ready"),
			Status: http.StatusGatewayTimeout,
			FunctionName: functionName,
		}
	case <-functionReadyChannel:
		f.logger.DebugWith("Function ready", "target", target)
		resultStatus = FunctionStatusResult{
			Status: http.StatusOK,
			FunctionName: functionName,
		}
	}


	// now handle all pending requests for a minute
	tc := time.After(1*time.Minute)
	for {
		select {
		case responseChannel := <-functionSinkChannel:
			f.logger.DebugWith("Got on channel", "target", target)
			responseChannel <- resultStatus
		case <- tc:
			f.logger.Debug("Releasing function sink")
			f.deleteFunctionSink(functionName)
			return
		}
	}

}

func (n *nuclioActioner) updateFunctionStatus(functionName string) {
	function, err := n.nuclioClientSet.NuclioV1beta1().Functions(n.functionStarter.namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		n.functionStarter.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
		return
	}

	function.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	_, err = n.nuclioClientSet.NuclioV1beta1().Functions(n.functionStarter.namespace).Update(function)
	if err != nil {
		n.functionStarter.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
		return
	}
}

func (n *nuclioActioner) waitFunctionReadiness(functionName string, ch chan bool) {
	for {
		function, err := n.nuclioClientSet.NuclioV1beta1().Functions(n.functionStarter.namespace).Get(functionName, metav1.GetOptions{})
		if err != nil {
			n.functionStarter.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
			return
		}
		n.functionStarter.logger.DebugWith("Started function", "state", function.Status.State)
		if function.Status.State != functionconfig.FunctionStateReady {
			time.Sleep(time.Second*5)
		} else {
			break
		}
	}
	ch <- true
}

func (f *FunctionStarter) deleteFunctionSink(functionName string) {
	f.functionSinkMutex.Lock()
	delete(f.functionSinksMap, functionName)
	f.functionSinkMutex.Unlock()
}
