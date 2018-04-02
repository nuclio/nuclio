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

package functiontemplates

//import "github.com/nuclio/nuclio/pkg/functionconfig"

// indicate to go generate that it needs to run our codegen
//go:generate go run $GOPATH/src/github.com/nuclio/nuclio/cmd/codegen/main.go -p $GOPATH/src/github.com/nuclio/nuclio/hack/examples -o generated.go

// FunctionTemplates holds the function templates
//var FunctionTemplates = []*FunctionTemplate{
//	{
//		Name: "Hello World",
//		Configuration: functionconfig.Config{
//			Meta: functionconfig.Meta{
//				Labels: map[string]string{
//					"a": "b",
//					"c": "d",
//				},
//			},
//			Spec: functionconfig.Spec{
//				Handler: "main:Handler",
//				Runtime: "golang",
//			},
//		},
//		SourceCode: `
//package main
//
//import (
//	"github.com/nuclio/nuclio-sdk-go"
//)
//
//func Handler(context *nuclio.Context, event nuclio.Event) (interface{}, error) {
//	context.Logger.Info("This is an unstrucured %s", "log")
//
//	return nuclio.Response{
//		StatusCode:  200,
//		ContentType: "application/text",
//		Body:        []byte("Hello, from nuclio :]"),
//	}, nil
//}`,
//	},
//}
