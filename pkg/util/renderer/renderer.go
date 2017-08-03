package renderer

import "io"

type TableRow []string

type Renderer struct {
	output io.Writer
}

func NewRenderer(output io.Writer) (*Renderer, error) {
	return &Renderer{
		output: output,
	}, nil
}

func (r *Renderer) RenderTable(header TableRow, records []TableRow) {

}
