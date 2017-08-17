/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package executor

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionExecutor struct {
	nucliocli.KubeConsumer
	logger  nuclio.Logger
	writer  io.Writer
	options *Options
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
	kubeHost, err = newFunctionExecutor.GetClients(newFunctionExecutor.logger, options.Common.KubeconfigPath)
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
	functioncrInstance, err := fe.FunctioncrClient.Get(fe.options.Common.Namespace, fe.options.Common.Identifier)
	if err != nil {
		return errors.Wrap(err, "Failed to get function custom resource")
	}

	functionService, err := fe.Clientset.CoreV1().Services(functioncrInstance.Namespace).Get(functioncrInstance.Name, meta_v1.GetOptions{})
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

	fe.logger.InfoWith("Executing function",
		"method", fe.options.Method,
		"url", fullpath,
		"body", body,
	)

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

	htmlData, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	fe.logger.InfoWith("Got response",
		"status", response.Status,
		"body", string(htmlData),
	)

	return nil
}
