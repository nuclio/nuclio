package executor

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type FunctionExecutor struct {
	nucliocli.KubeConsumer
	logger           nuclio.Logger
	writer           io.Writer
	options          *Options
	functioncrClient *functioncr.Client
	clientset        *kubernetes.Clientset
}

func NewFunctionExecutor(parentLogger nuclio.Logger,
	writer io.Writer,
	options *Options) (*FunctionExecutor, error) {
	var err error
	kubeHost := ""

	newFunctionExecutor := &FunctionExecutor{
		logger:  parentLogger.GetChild("executor").(nuclio.Logger),
		writer:  writer,
		options: options,
	}

	// get kube stuff
	kubeHost,
		newFunctionExecutor.clientset,
		newFunctionExecutor.functioncrClient,
		err = newFunctionExecutor.GetClients(newFunctionExecutor.logger, options.Common.KubeconfigPath)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	// update kubehost if not set
	if options.Common.KubeHost == "" {
		options.Common.KubeHost = kubeHost
	}

	return newFunctionExecutor, nil
}

func (fe *FunctionExecutor) Execute() error {
	functioncrInstance, err := fe.functioncrClient.Get(fe.options.Common.Namespace, fe.options.Name)
	if err != nil {
		return errors.Wrap(err, "Failed to get function custom resource")
	}

	functionService, err := fe.clientset.CoreV1().Services(functioncrInstance.Namespace).Get(functioncrInstance.Name, meta_v1.GetOptions{})
	if err != nil {
		return errors.Wrap(err, "Failed to get function service")
	}

	if fe.options.ClusterIP == "" {
		url, err := url.Parse(fe.options.Common.KubeHost)
		if err == nil && url.Host != "" {
			fe.options.ClusterIP = strings.Split(url.Host, ":")[0]
		}
	}

	port := strconv.Itoa(int(functionService.Spec.Ports[0].NodePort))

	fullpath := "http://" + fe.options.ClusterIP + ":" + port + "/" + fe.options.Url

	client := &http.Client{}
	var req *http.Request
	var body io.Reader = http.NoBody

	// set body for post
	if fe.options.Method == "POST" {
		body = bytes.NewBuffer([]byte(fe.options.Body))
	}

	// issue the request
	req, err = http.NewRequest(fe.options.Method, fullpath, body)
	if err != nil {
		return errors.Wrap(err, "Failed to create HTTP request")
	}

	req.Header.Set("Content-Type", fe.options.ContentType)
	headers := common.StringToStringMap(fe.options.Headers)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	response, err := client.Do(req)
	if err != nil {
		return errors.Wrap(err, "Failed to send HTTP request")
	}

	defer response.Body.Close()
	fmt.Fprintf(fe.writer, "Got response from %s:\nStatus: %s\nBody:\n", functioncrInstance.Name, response.Status)

	htmlData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	fmt.Fprintf(fe.writer, "%s", htmlData)

	return nil
}
