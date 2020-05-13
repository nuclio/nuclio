package common

import (
	"fmt"

	"github.com/nuclio/nuclio/pkg/platform"
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
