package getter

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/util/common"
	"github.com/nuclio/nuclio/pkg/util/renderer"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type FunctionGetter struct {
	nucliocli.KubeConsumer
	logger           nuclio.Logger
	writer           io.Writer
	options          *Options
	functioncrClient *functioncr.Client
	clientset        *kubernetes.Clientset
}

func NewFunctionGetter(parentLogger nuclio.Logger, writer io.Writer, options *Options) (*FunctionGetter, error) {
	var err error

	newFunctionGetter := &FunctionGetter{
		logger:  parentLogger.GetChild("getter").(nuclio.Logger),
		writer:  writer,
		options: options,
	}

	// get kube stuff
	_, newFunctionGetter.clientset,
		newFunctionGetter.functioncrClient,
		err = newFunctionGetter.GetClients(newFunctionGetter.logger, options.Common.KubeconfigPath)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get clients")
	}

	return newFunctionGetter, nil
}

func (fg *FunctionGetter) Execute() error {
	var err error

	resourceName, resourceVersion, err := fg.parseResourceIdentifier(fg.options.ResourceIdentifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	functionsToRender := []*functioncr.Function{}

	// if version is specified, get single function
	if resourceVersion != nil {

		// get specific function CR
		function, err := fg.functioncrClient.Get(fg.options.Common.Namespace, resourceName)
		if err != nil {
			return errors.Wrap(err, "Failed to get function")
		}

		functionsToRender = append(functionsToRender, function)

	} else {

		functions, err := fg.functioncrClient.List(fg.options.Common.Namespace,
			meta_v1.ListOptions{LabelSelector: fg.options.Labels})

		if err != nil {
			return errors.Wrap(err, "Failed to list functions")
		}

		// convert []Function to []*Function
		for _, function := range functions.Items {
			functionsToRender = append(functionsToRender, &function)
		}
	}

	// render it
	return fg.renderFunctions(functionsToRender)
}

func (fg *FunctionGetter) renderFunctions(functions []*functioncr.Function) error {

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
			functionFields := fg.getFunctionFields(function, fg.options.Format == "wide")

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

func (fg *FunctionGetter) parseResourceIdentifier(resourceIdentifier string) (resourceName string,
	resourceVersion *string,
	err error) {

	// of the form: resourceName:resourceVersion or just resourceName
	list := strings.Split(resourceIdentifier, ":")

	// set the resource name
	resourceName = list[0]

	// only resource name provided
	if len(list) == 1 {
		return
	}

	// validate the resource version
	if err = fg.validateVersion(list[1]); err != nil {
		return
	}

	// set the resource version
	resourceVersion = &list[1]

	// if the resource is numeric
	if *resourceVersion != "latest" {
		resourceName = fmt.Sprintf("%s-%s", resourceName, *resourceVersion)
	}

	return
}

func (fg *FunctionGetter) validateVersion(resourceVersion string) error {

	// can be either "latest" or numeric
	if resourceVersion != "latest" {
		_, err := strconv.Atoi(resourceVersion)
		if err != nil {
			return errors.Wrap(err, `Version must be either "latest" or numeric`)
		}
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

	service, err := fg.clientset.CoreV1().Services(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
	if err != nil {
		return returnPartialFunctionFields()
	}

	deployment, err := fg.clientset.AppsV1beta1().Deployments(function.Namespace).Get(function.Name, meta_v1.GetOptions{})
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
