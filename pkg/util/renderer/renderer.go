package renderer

import (
	"io"
	"fmt"
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/olekukonko/tablewriter"
	"github.com/ghodss/yaml"
	"bytes"
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