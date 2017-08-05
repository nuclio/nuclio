package getter

import (
	"io"
	"strconv"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/util/renderer"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type FunctionGetter struct {
	nucliocli.KubeConsumer
	logger  nuclio.Logger
	writer  io.Writer
	options *Options
}

func NewFunctionGetter(parentLogger nuclio.Logger, writer io.Writer, options *Options) (*FunctionGetter, error) {
	var err error

	newFunctionGetter := &FunctionGetter{
		logger:  parentLogger.GetChild("getter").(nuclio.Logger),
		writer:  writer,
		options: options,
	}

	// get kube stuff
	_, err = newFunctionGetter.GetClients(newFunctionGetter.logger, options.Common.KubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionGetter, nil
}

func (fg *FunctionGetter) Execute() error {
	var err error

	resourceName, resourceVersion, err := nucliocli.ParseResourceIdentifier(fg.options.ResourceIdentifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	functionsToRender := []functioncr.Function{}

	// if version is specified, get single function
	if resourceVersion != nil {

		// get specific function CR
		function, err := fg.FunctioncrClient.Get(fg.options.Common.Namespace, resourceName)
		if err != nil {
			return errors.Wrap(err, "Failed to get function")
		}

		functionsToRender = append(functionsToRender, *function)

	} else {

		functions, err := fg.FunctioncrClient.List(fg.options.Common.Namespace,
			&meta_v1.ListOptions{LabelSelector: fg.options.Labels})

		if err != nil {
			return errors.Wrap(err, "Failed to list functions")
		}

		// convert []Function to []*Function
		functionsToRender = functions.Items
	}

	// render it
	return fg.renderFunctions(functionsToRender)
}

func (fg *FunctionGetter) renderFunctions(functions []functioncr.Function) error {

	rendererInstance := renderer.NewRenderer(fg.writer)

	switch fg.options.Format {
	case "text", "wide":
		header := []string{"Namespace", "Name", "Version", "State", "Local URL", "Node Port", "Replicas"}
		if fg.options.Format == "wide" {
			header = append(header, "Labels")
		}

		functionRecords := [][]string{}

		// for each field
		for _, function := range functions {

			// get its fields
			functionFields := fg.getFunctionFields(&function, fg.options.Format == "wide")

			// add to records
			functionRecords = append(functionRecords, functionFields)
		}

		rendererInstance.RenderTable(header, functionRecords)
	case "yaml":
		rendererInstance.RenderYAML(functions)
	case "json":
		rendererInstance.RenderJSON(functions)
	}

	return nil
}

func (fg *FunctionGetter) getFunctionFields(function *functioncr.Function, wide bool) []string {

	// populate stuff from function
	line := []string{function.Namespace,
		function.Labels["name"],
		function.Labels["version"],
		string(function.Status.State)}

	// add info from service & deployment
	// TODO: for lists we can get Service & Deployment info using .List get into a map to save http gets

	returnPartialFunctionFields := func() []string {
		return append(line, []string{"-", "-", "-"}...)
	}

	service, err := fg.Clientset.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {
		return returnPartialFunctionFields()
	}

	deployment, err := fg.Clientset.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {
		return returnPartialFunctionFields()
	}

	cport := strconv.Itoa(int(service.Spec.Ports[0].Port))
	nport := strconv.Itoa(int(service.Spec.Ports[0].NodePort))
	pods := strconv.Itoa(int(deployment.Status.AvailableReplicas)) + "/" + strconv.Itoa(int(*deployment.Spec.Replicas))
	line = append(line, []string{service.Spec.ClusterIP + ":" + cport, nport, pods}...)

	if fg.options.Format == "wide" {
		line = append(line, common.StringMapToString(function.Labels))
	}

	return line
}
