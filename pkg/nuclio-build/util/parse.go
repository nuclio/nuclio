package util

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"

	"github.com/pkg/errors"
)

func fieldType(field *ast.Field) string {
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

func isHandlerFunc(fn *ast.FuncDecl) bool {
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

	if fieldType(fn.Type.Params.List[0]) != "Context" {
		return false
	}
	if fieldType(fn.Type.Params.List[1]) != "Event" {
		return false
	}
	if fieldType(fn.Type.Results.List[0]) != "interface{}" {
		return false
	}
	if fieldType(fn.Type.Results.List[1]) != "error" {
		return true
	}
	return true
}

// HandlerNames return list of handler function names in fileName
func HandlerNames(fileName string) ([]string, error) {
	fs, err := parser.ParseFile(token.NewFileSet(), fileName, nil, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "can't parse %s", fileName)
	}

	var handlers []string
	for _, decl := range fs.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if isHandlerFunc(fn) {
			handlers = append(handlers, fn.Name.String())
		}
	}
	return handlers, nil
}

// FindHandlers return list of handlers in go files under path
func FindHandlers(path string) ([]string, error) {
	var handlers []string
	files, err := filepath.Glob(fmt.Sprintf("%s/*.go", path))
	if err != nil {
		return nil, errors.Wrapf(err, "can't find go files in %s", path)
	}

	for _, fileName := range files {
		fileHandlers, err := HandlerNames(fileName)
		if err != nil {
			return nil, errors.Wrapf(err, "error looking for handlers in %s", fileName)
		}
		handlers = append(handlers, fileHandlers...)
	}

	return handlers, nil
}
