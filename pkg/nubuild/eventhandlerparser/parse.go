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

package eventhandlerparser

import (
	"fmt"
	"os"
	"path"

	"go/ast"
	"go/parser"
	"go/token"

	"github.com/pkg/errors"

	nuclio "github.com/nuclio/nuclio-sdk"
)

// EventHandlerParser parsers event handlers
type EventHandlerParser struct {
	logger nuclio.Logger
}

// NewEventHandlerParser returns new EventHandlerParser
func NewEventHandlerParser(logger nuclio.Logger) *EventHandlerParser {
	return &EventHandlerParser{logger}
}

func (ehp *EventHandlerParser) fieldType(field *ast.Field) string {
	switch field.Type.(type) {
	case *ast.StarExpr: // *nuclio.Context
		ptr := field.Type.(*ast.StarExpr)
		sel, ok := ptr.X.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		return sel.Sel.Name
	case *ast.SelectorExpr: // nuclio.Event
		sel := field.Type.(*ast.SelectorExpr)
		return sel.Sel.Name
	case *ast.InterfaceType: // interface{}
		ifc := field.Type.(*ast.InterfaceType)
		if ifc.Methods.NumFields() == 0 {
			return "interface{}"
		}
		// TODO: How to get interface name?
		return ""
	case *ast.Ident: // error
		idt := field.Type.(*ast.Ident)
		return idt.Name
	}
	return ""
}

// Example:
// func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {

func (ehp *EventHandlerParser) isEventHandlerFunc(fn *ast.FuncDecl) bool {
	name := fn.Name.String()

	if name[0] < 'A' || name[0] > 'Z' {
		return false
	}

	if fn.Type.Params.NumFields() != 2 {
		return false
	}

	if fn.Type.Results.NumFields() != 2 {
		return false
	}

	if ehp.fieldType(fn.Type.Params.List[0]) != "Context" {
		return false
	}

	if ehp.fieldType(fn.Type.Params.List[1]) != "Event" {
		return false
	}

	if ehp.fieldType(fn.Type.Results.List[0]) != "interface{}" {
		return false
	}

	if ehp.fieldType(fn.Type.Results.List[1]) != "error" {
		return true
	}

	return true
}

func (ehp *EventHandlerParser) findEventHandlers(file *ast.File) ([]string, error) {
	var eventHandlers []string

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if ehp.isEventHandlerFunc(fn) {
			eventHandlers = append(eventHandlers, fn.Name.String())
		}
	}
	return eventHandlers, nil
}

func (ehp *EventHandlerParser) toSlice(m map[string]bool) []string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// ParseEventHandlers return list of handler names in path
func (ehp *EventHandlerParser) ParseEventHandlers(eventHandlerPath string) ([]string, error) {
	pathInfo, err := os.Stat(eventHandlerPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get path information")
	}

	var filter func(os.FileInfo) bool

	// will hold the directory that will be read
	eventHandlerDir := eventHandlerPath

	// if the path points to a file, set the filter to one that will verify that only the given file
	// is parsed
	if !pathInfo.IsDir() {
		filter = func(fi os.FileInfo) bool {
			return fi.Name() == path.Base(eventHandlerPath)
		}

		eventHandlerDir = path.Dir(eventHandlerPath)
	}

	pkgs, err := parser.ParseDir(token.NewFileSet(), eventHandlerDir, filter, 0)
	if err != nil {
		ehp.logger.ErrorWith("Can't parse directory", "dir", eventHandlerDir, "error", err)
		return nil, errors.Wrapf(err, "can't parse %s", eventHandlerDir)
	}

	// We want unique list of handler names
	handlerNames := make(map[string]bool)

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			fileHandlers, err := ehp.findEventHandlers(file)
			if err != nil {
				ehp.logger.ErrorWith("can't parse file", "path", file.Name.String(), "error", err)
				return nil, errors.Wrapf(err, "error parsing %s", file.Name.String())
			}
			for _, handlerName := range fileHandlers {
				if handlerNames[handlerName] {
					return nil, fmt.Errorf("Duplicate handler name - %q", handlerName)
				}
				handlerNames[handlerName] = true
			}
		}
	}

	return ehp.toSlice(handlerNames), nil
}
