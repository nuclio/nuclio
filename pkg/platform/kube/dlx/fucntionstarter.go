package dlx

import (
	"net/http"
	"sync"
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	nuclioio_client "github.com/nuclio/nuclio/pkg/platform/kube/client/clientset/versioned"

	"github.com/nuclio/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type responseChannel chan FunctionStatusResult
type functionSinksMap map[string]chan responseChannel

type nuclioWrapper interface {
	waitFunctionReadiness(string, chan bool)
	updateFunctionStatus(string)
}

type nuclioActioner struct {
	nuclioClientSet nuclioio_client.Interface
	functionStarter *FunctionStarter
}

type FunctionStarter struct {
	logger                   logger.Logger
	nuclioClientSet          nuclioio_client.Interface
	namespace                string
	functionSinksMap         functionSinksMap
	functionSinkMutex        sync.Mutex
	nuclioActioner           nuclioWrapper
	functionReadinnesTimeout time.Duration
}

type FunctionStatusResult struct {
	FunctionName string
	Status       int
	Error        error
}

func NewFunctionStarter(parentLogger logger.Logger,
	namespace string,
	nuclioClientSet nuclioio_client.Interface) (*FunctionStarter, error) {
	fs := &FunctionStarter{
		logger:                   parentLogger.GetChild("function-starter"),
		functionSinksMap:         make(functionSinksMap),
		namespace:                namespace,
		functionReadinnesTimeout: time.Minute,
	}
	fs.nuclioActioner = &nuclioActioner{
		nuclioClientSet: nuclioClientSet,
		functionStarter: fs,
	}
	return fs, nil
}

func (f *FunctionStarter) handleFunctionStart(originalTarget string, handlerResponseChannel responseChannel) {
	functionSinkChannel := f.getOrCreateFunctionSink(originalTarget, handlerResponseChannel)
	functionSinkChannel <- handlerResponseChannel
}

func (f *FunctionStarter) getOrCreateFunctionSink(originalTarget string,
	handlerResponseChannel responseChannel) chan responseChannel {
	var functionSinkChannel chan responseChannel
	f.functionSinkMutex.Lock()
	defer f.functionSinkMutex.Unlock()

	if _, found := f.functionSinksMap[originalTarget]; found {
		functionSinkChannel = f.functionSinksMap[originalTarget]
	} else {

		// for the next requests coming in
		functionSinkChannel = make(chan responseChannel)
		f.functionSinksMap[originalTarget] = functionSinkChannel
		f.logger.DebugWith("Created function sink", "target", originalTarget)

		// start the function and get ready to listen on function sink channel
		go f.startFunction(functionSinkChannel, originalTarget)
	}

	return functionSinkChannel
}

func (f *FunctionStarter) startFunction(functionSinkChannel chan responseChannel, target string) {
	var resultStatus FunctionStatusResult

	// simple for now
	functionName := target

	f.logger.DebugWith("Starting function", "function", functionName)
	f.nuclioActioner.updateFunctionStatus(functionName)
	functionReadyChannel := make(chan bool, 1)
	defer close(functionReadyChannel)

	go f.nuclioActioner.waitFunctionReadiness(functionName, functionReadyChannel)

	select {
	case <-time.After(f.functionReadinnesTimeout):
		f.logger.WarnWith("Timed out waiting for function to be ready", "function", functionName)
		defer f.deleteFunctionSink(functionName)
		resultStatus = FunctionStatusResult{
			Error:        errors.New("Timed out waiting for function to be ready"),
			Status:       http.StatusGatewayTimeout,
			FunctionName: functionName,
		}
	case <-functionReadyChannel:
		f.logger.DebugWith("Function ready", "target", target)
		resultStatus = FunctionStatusResult{
			Status:       http.StatusOK,
			FunctionName: functionName,
		}
	}

	// now handle all pending requests for a minute
	tc := time.After(1 * time.Minute)
	for {
		select {
		case responseChannel := <-functionSinkChannel:
			responseChannel <- resultStatus
		case <-tc:
			f.logger.Debug("Releasing function sink")
			f.deleteFunctionSink(functionName)
			return
		}
	}

}

func (n *nuclioActioner) updateFunctionStatus(functionName string) {
	function, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(n.functionStarter.namespace).Get(functionName, metav1.GetOptions{})
	if err != nil {
		n.functionStarter.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
		return
	}

	function.Status.State = functionconfig.FunctionStateWaitingForResourceConfiguration
	_, err = n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(n.functionStarter.namespace).Update(function)
	if err != nil {
		n.functionStarter.logger.WarnWith("Failed to update function", "functionName", functionName, "err", err)
		return
	}
}

func (n *nuclioActioner) waitFunctionReadiness(functionName string, readyChannel chan bool) {
	for {
		function, err := n.nuclioClientSet.NuclioV1beta1().NuclioFunctions(n.functionStarter.namespace).Get(functionName, metav1.GetOptions{})
		if err != nil {
			n.functionStarter.logger.WarnWith("Failed to get nuclio function", "functionName", functionName, "err", err)
			return
		}
		n.functionStarter.logger.DebugWith("Started function", "state", function.Status.State)
		if function.Status.State != functionconfig.FunctionStateReady {
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}
	readyChannel <- true
}

func (f *FunctionStarter) deleteFunctionSink(functionName string) {
	f.functionSinkMutex.Lock()
	delete(f.functionSinksMap, functionName)
	f.functionSinkMutex.Unlock()
}
