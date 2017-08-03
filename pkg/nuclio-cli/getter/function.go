package getter

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nuclio/nuclio/pkg/functioncr"
	"github.com/nuclio/nuclio/pkg/nuclio-cli"
	"github.com/nuclio/nuclio/pkg/util/renderer"

	"github.com/nuclio/nuclio-sdk"
	"github.com/pkg/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type FunctionGetter struct {
	nucliocli.KubeConsumer
	logger  nuclio.Logger
	options *Options
	writer  io.Writer
}

func NewFunctionGetter(parentLogger nuclio.Logger, writer io.Writer) *FunctionGetter {
	return &FunctionGetter{
		logger: parentLogger.GetChild("get").(nuclio.Logger),
		writer: writer,
	}
}

func (fg *FunctionGetter) Execute(options *Options) error {
	fg.options = options

	_, clientset, functioncrClient, err := fg.GetClients(fg.logger, options.Common.KubeconfigPath)
	if err != nil {
		return errors.Wrap(err, "Failed to get clients")
	}

	resourceName, resourceVersion, err := fg.parseResourceIdentifier(options.ResourceIdentifier)
	if err != nil {
		return errors.Wrap(err, "Failed to parse resource identifier")
	}

	header := []string{"Namespace", "Name", "Version", "State", "Local URL", "Node Port", "Replicas"}
	if options.Format == "wide" {
		header = append(header, "Labels")
	}

	// if version is specified, get single function
	if resourceVersion != nil {

		// get specific function CR
		functioncrInstance, err := functioncrClient.Get(options.Common.Namespace, resourceName)
		if err != nil {
			return errors.Wrap(err, "Failed to get function CR")
		}

		// get its fields
		functionFields, err := fg.getFunctionFields(clientset, functioncrInstance, options.Format == "wide")
		if err != nil {
			return errors.Wrap(err, "Failed to get function fields")
		}

		// render it
		fg.renderFunctions(header, [][]string{functionFields}, functioncrInstance)

	} else {

	}

	return nil
}

func (fg *FunctionGetter) renderFunctions(header []string, records [][]string, functions interface{}) {
	renderer := renderer.NewRenderer(fg.writer)

	switch fg.options.Format {
	case "text", "wide":
		renderer.RenderTable(header, records)
	case "yaml":
		renderer.RenderYAML(functions)
	case "json":
		renderer.RenderJSON(functions)
	}
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

func (fg *FunctionGetter) getFunctionFields(clientset *kubernetes.Clientset,
	functioncrInstance *functioncr.Function,
	wide bool) ([]string, error) {
	line := []string{functioncrInstance.Namespace,
		functioncrInstance.Labels["name"],
		functioncrInstance.Labels["version"],
		string(functioncrInstance.Status.State)}

	// add info from service & deployment
	// TODO: for lists we can get Service & Deployment info using .List get into a map to save http gets

	service, err := clientset.CoreV1().Services(functioncrInstance.Namespace).Get(functioncrInstance.Name, meta_v1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get service")
	}

	deployment, err := clientset.AppsV1beta1().Deployments(functioncrInstance.Namespace).Get(functioncrInstance.Name, meta_v1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get deployment")
	}

	cport := strconv.Itoa(int(service.Spec.Ports[0].Port))
	nport := strconv.Itoa(int(service.Spec.Ports[0].NodePort))
	pods := strconv.Itoa(int(deployment.Status.AvailableReplicas)) + "/" + strconv.Itoa(int(*deployment.Spec.Replicas))
	line = append(line, []string{service.Spec.ClusterIP + ":" + cport, nport, pods}...)

	return line, nil
}
