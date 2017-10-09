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

package renderer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/nuclio/nuclio/pkg/errors"

	"github.com/ghodss/yaml"
	"github.com/olekukonko/tablewriter"
)

type Renderer struct {
	output io.Writer
}

func NewRenderer(output io.Writer) *Renderer {
	return &Renderer{
		output: output,
	}
}

func (r *Renderer) RenderTable(header []string, records [][]string) {
	tableWriter := tablewriter.NewWriter(r.output)
	tableWriter.SetHeader(header)
	tableWriter.SetBorders(tablewriter.Border{Left: false, Top: false, Right: false, Bottom: false})
	tableWriter.SetCenterSeparator("|")
	tableWriter.SetHeaderLine(false)
	tableWriter.AppendBulk(records)
	tableWriter.Render()
}

func (r *Renderer) RenderYAML(items interface{}) error {
	body, err := yaml.Marshal(items)
	if err != nil {
		return errors.Wrap(err, "Failed to render YAML")
	}

	fmt.Fprintln(r.output, string(body))

	return nil
}

func (r *Renderer) RenderJSON(items interface{}) error {
	body, err := json.Marshal(items)
	if err != nil {
		return errors.Wrap(err, "Failed to render JSON")
	}

	pbody := bytes.Buffer{}
	err = json.Indent(&pbody, body, "", "\t")
	if err != nil {
		return errors.Wrap(err, "Failed to indent JSON")
	}

	fmt.Fprintln(r.output, string(pbody.Bytes()))

	return nil
}
