/*
Copyright 2023 The Nuclio Authors.

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

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/nuclio/errors"
	"sigs.k8s.io/yaml"
)

type Renderer struct {
	output io.Writer
}

func NewRenderer(output io.Writer) *Renderer {
	return &Renderer{
		output: output,
	}
}

func (r *Renderer) RenderTable(header []interface{}, records [][]interface{}) {
	tw := table.NewWriter()
	tw.SetOutputMirror(r.output)
	tw.SetStyle(table.Style{
		Name: "Nuclio",
		Box: table.BoxStyle{
			MiddleVertical: "|",
			PaddingLeft:    " ",
			PaddingRight:   " ",
		},
		Options: table.Options{
			DoNotColorBordersAndSeparators: true,
			DrawBorder:                     false,
			SeparateColumns:                true,
			SeparateFooter:                 false,
			SeparateHeader:                 false,
			SeparateRows:                   false,
		},
		Color:  table.ColorOptionsDefault,
		Format: table.FormatOptionsDefault,
		HTML:   table.DefaultHTMLOptions,
		Title:  table.TitleOptionsDefault,
	})
	tw.AppendHeader(r.rowInterfaceToTableRow(header), table.RowConfig{})
	tw.AppendRows(r.rowsStringToTableRows(records), table.RowConfig{})
	tw.Render()
}

func (r *Renderer) RenderYAML(items interface{}) error {
	body, err := yaml.Marshal(items)
	if err != nil {
		return errors.Wrap(err, "Failed to render YAML")
	}

	fmt.Fprintln(r.output, string(body)) // nolint: errcheck

	return nil
}

func (r *Renderer) RenderJSON(items interface{}) error {
	body, err := json.Marshal(items)
	if err != nil {
		return errors.Wrap(err, "Failed to render JSON")
	}

	var pbody bytes.Buffer
	if err := json.Indent(&pbody, body, "", "\t"); err != nil {
		return errors.Wrap(err, "Failed to indent JSON")
	}

	fmt.Fprintln(r.output, pbody.String()) // nolint: errcheck

	return nil
}

func (r *Renderer) rowsStringToTableRows(rows [][]interface{}) []table.Row {
	tableRows := make([]table.Row, len(rows))
	for rowIndex, rowValue := range rows {
		tableRows[rowIndex] = r.rowInterfaceToTableRow(rowValue)
	}
	return tableRows
}

func (r *Renderer) rowInterfaceToTableRow(row []interface{}) table.Row {
	tableRow := make(table.Row, len(row))
	copy(tableRow, row)
	return tableRow
}
