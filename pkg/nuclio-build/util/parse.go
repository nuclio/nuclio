package util

import (
	"go/ast"
	"go/parser"
	"go/token"

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

func findHandlers(file *ast.File) ([]string, error) {
	var handlers []string

	for _, decl := range file.Decls {
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

func toSlice(m map[string]bool) []string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// ParseHandler return list of packages and handler names in path
func ParseHandler(path string) ([]string, []string, error) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "can't parse %s", path)
	}

	// We want unique list of package names
	pkgNames := make(map[string]bool)
	var handlerNames []string

	for _, pkg := range pkgs {
		pkgNames[pkg.Name] = true
		for _, file := range pkg.Files {
			fileHandlers, err := findHandlers(file)
			if err != nil {
				return nil, nil, errors.Wrapf(err, "error parsing %s", file.Name.String())
			}
			handlerNames = append(handlerNames, fileHandlers...)
		}
	}

	return toSlice(pkgNames), handlerNames, nil
}
