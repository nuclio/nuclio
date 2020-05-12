package command

import (
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/platform"
	"github.com/nuclio/nuclio/pkg/renderer"

	"github.com/nuclio/logger"
)

func formatFunctionIngresses(function platform.Function) string {
	var formattedIngresses string

	ingresses := function.GetIngresses()

	for _, ingress := range ingresses {
		host := ingress.Host
		if host != "" {
			host += ":<port>"
		}

		for _, path := range ingress.Paths {
			formattedIngresses += fmt.Sprintf("%s%s, ", host, path)
		}
	}

	// add default ingress
	formattedIngresses += fmt.Sprintf("/%s/%s",
		function.GetConfig().Meta.Name,
		function.GetVersion())

	return formattedIngresses
}

func renderFunctions(logger logger.Logger,
	functions []platform.Function,
	format string,
	writer io.Writer,
	renderCallback func(functions []platform.Function, renderer func(interface{}) error) error) error {

	waitGroup := sync.WaitGroup{}
	waitGroup.Add(len(functions))

	// iterate over each function and make sure it's initialized
	for _, function := range functions {
		go func(function platform.Function) {
			if err := function.Initialize(nil); err != nil {
				logger.WarnWith("Failed to initialize function", "err", err.Error())
			}
			waitGroup.Done()
		}(function)
	}
	waitGroup.Wait()

	rendererInstance := renderer.NewRenderer(writer)

	switch format {
	case OutputFormatText, OutputFormatWide:
		header := []string{"Namespace", "Name", "Project", "State", "Node Port", "Replicas"}
		if format == OutputFormatWide {
			header = append(header, []string{
				"Labels",
				"Ingresses",
			}...)
		}

		var functionRecords [][]string

		// for each field
		for _, function := range functions {
			availableReplicas, specifiedReplicas := function.GetReplicas()

			// get its fields
			functionFields := []string{
				function.GetConfig().Meta.Namespace,
				function.GetConfig().Meta.Name,
				function.GetConfig().Meta.Labels["nuclio.io/project-name"],
				string(function.GetStatus().State),
				strconv.Itoa(function.GetStatus().HTTPPort),
				fmt.Sprintf("%d/%d", availableReplicas, specifiedReplicas),
			}

			// add fields for wide view
			if format == OutputFormatWide {
				functionFields = append(functionFields, []string{
					common.StringMapToString(function.GetConfig().Meta.Labels),
					formatFunctionIngresses(function),
				}...)
			}

			// add to records
			functionRecords = append(functionRecords, functionFields)
		}

		rendererInstance.RenderTable(header, functionRecords)
	case OutputFormatYAML:
		return renderCallback(functions, rendererInstance.RenderYAML)
	case OutputFormatJSON:
		return renderCallback(functions, rendererInstance.RenderJSON)
	}

	return nil
}

func renderProjects(projects []platform.Project,
	format string,
	writer io.Writer,
	renderCallback func(functions []platform.Project, renderer func(interface{}) error) error) error {

	rendererInstance := renderer.NewRenderer(writer)

	switch format {
	case OutputFormatText, OutputFormatWide:
		header := []string{"Namespace", "Name", "Display Name"}
		if format == OutputFormatWide {
			header = append(header, []string{
				"Description",
			}...)
		}

		var projectRecords [][]string

		// for each field
		for _, project := range projects {

			// get its fields
			projectFields := []string{
				project.GetConfig().Meta.Namespace,
				project.GetConfig().Meta.Name,
				project.GetConfig().Spec.DisplayName,
			}

			// add fields for wide view
			if format == OutputFormatWide {
				projectFields = append(projectFields, []string{
					project.GetConfig().Spec.Description,
				}...)
			}

			// add to records
			projectRecords = append(projectRecords, projectFields)
		}

		rendererInstance.RenderTable(header, projectRecords)
	case OutputFormatYAML:
		return renderCallback(projects, rendererInstance.RenderYAML)
	case OutputFormatJSON:
		return renderCallback(projects, rendererInstance.RenderJSON)
	}

	return nil
}
