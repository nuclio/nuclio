package common

import (
	"fmt"
	"io"
	"os"

	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/nuclio/errors"
)

func FormatFunctionIngresses(function platform.Function) string {
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

func OpenFile(filepath string) (io.Reader, error) {
	fileInfo, err := os.Stat(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrapf(err, "File path `%s` does not exists", filepath)
		}
		return nil, errors.Wrapf(err, "Failed to stat file `%s`", filepath)
	}
	if fileInfo.IsDir() {
		return nil, errors.Errorf("Expected path to a file, received a dir `%s`", filepath)
	}
	file, err := os.Open(filepath)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to open file %s", filepath)
	}
	return file, err
}
