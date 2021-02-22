// +build function_templates_generator

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

// This program generates function template sources in pkg/dashboard/functiontemplates/generated.go
// It can be invoked by running go generate

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dashboard/functiontemplates"
	"github.com/nuclio/nuclio/pkg/functionconfig"
	"github.com/nuclio/nuclio/pkg/processor/build"
	"github.com/nuclio/nuclio/pkg/processor/build/inlineparser"

	"github.com/ghodss/yaml"
	"github.com/gobuffalo/flect"
	"github.com/nuclio/errors"
	"github.com/nuclio/logger"
	"github.com/nuclio/zap"
	yamlv2 "gopkg.in/yaml.v2"
)

var funcMap = template.FuncMap{

	// used inside the template to pack configuration objects into text (so that they're printed nicely)
	"marshalConfig": func(data interface{}) string {
		bytes, _ := yaml.Marshal(data)
		return string(bytes)
	},

	// function source code (and marshalled configurations) may contain backticks. since they're written inside raw,
	// backtick-quoted strings in the generated code, those must be escaped away
	"escapeBackticks": func(s string) string {
		return strings.Replace(s, "`", "`"+" + \"`\" + "+"`", -1)
	},

	"join": strings.Join,
}

var packageTemplate = template.
	Must(template.New("").
		Funcs(funcMap).
		Parse(`// Code generated by go generate; DO NOT EDIT.

/*
This machine-generated file contains the configuration and source code for function templates,
which may be retrieved from the dashboard's HTTP API by sending a GET request to /function_templates.

The following functions are included for each supported runtime:
{{- range $runtime, $functions := .FunctionsByRuntime }}
{{- if $functions }}
{{ printf "%s (%d):" $runtime (len $functions) | printf "%-15s" }} {{ join $functions ", " }}
{{- end }}
{{- end }}
*/

package functiontemplates

import (
	"github.com/nuclio/nuclio/pkg/functionconfig"

	"github.com/ghodss/yaml"
)

var GeneratedFunctionTemplates = []*generatedFunctionTemplate{
{{- range .FunctionTemplates }}
	{
		Name: "{{ .Name }}",
		Configuration: unmarshalConfig(` + "`" + `{{ marshalConfig .FunctionConfig | escapeBackticks }}` + "`" + `),
		SourceCode: ` + "`" + `{{ escapeBackticks .SourceCode }}` + "`" + `,
	},
{{- end }}
}

// no error checking is performed here. this is guaranteed to work, because the strings fed to this function
// are marshalled representations of actual configuration objects that were created while generating this file
func unmarshalConfig(marshalledConfig string) functionconfig.Config {
	config := functionconfig.Config{}

	err := yaml.Unmarshal([]byte(marshalledConfig), &config)
	if err != nil {
		panic("failed to unmarshal marshaled config")
	}

	return config
}
`))

type Runtime struct {
	Name          string
	InlineParser  *inlineparser.InlineParser
	FileExtension string
}

type Generator struct {
	logger      logger.Logger
	examplesDir string
	outputPath  string
	runtimes    []*Runtime
	functions   map[string][]string
}

func (g *Generator) generate() error {
	if err := g.verifyPaths(); err != nil {
		return errors.Wrap(err, "Failed to verify paths")
	}

	functionDirs, err := g.detectFunctionDirs()
	if err != nil {
		return errors.Wrap(err, "Failed to detect functions in given examples directory")
	}

	functionTemplates, err := g.buildFunctionTemplates(functionDirs)
	if err != nil {
		return errors.Wrap(err, "Failed to build function templates")
	}

	if err = g.writeOutputFile(functionTemplates); err != nil {
		return errors.Wrap(err, "Failed to write output file")
	}

	g.logger.Info("Done")

	return nil
}

func (g *Generator) verifyPaths() error {
	if !common.IsDir(g.examplesDir) {
		return errors.Errorf("Given examples directory is not a directory: %s", g.examplesDir)
	}

	g.logger.DebugWith("Verified examples directory exists", "path", g.examplesDir)

	return nil
}

func (g *Generator) detectFunctionDirs() ([]string, error) {
	var functionDirs []string

	g.logger.DebugWith("Looking for function directories inside runtime directories", "runtimes", g.runtimes)

	for _, runtime := range g.runtimes {
		g.functions[runtime.Name] = []string{}
		runtimeDir := filepath.Join(g.examplesDir, runtime.Name)

		// traverse each runtime directory, look for function dirs inside it
		err := filepath.Walk(runtimeDir, func(path string, info os.FileInfo, err error) error {

			// handle any failure to walk over a specific file
			if err != nil {
				g.logger.WarnWith("Failed to walk over file at path", "path", path)
				return errors.Wrapf(err, "Failed to walk over file at path %s", path)
			}

			if runtimeDir == path {

				// skipping runtime directory itself
				return nil
			}

			if info.IsDir() {

				// list function dir files
				files, err := ioutil.ReadDir(path)
				if err != nil {
					return errors.Wrapf(err, "Failed to read function dir files", "path", path)
				}

				// make sure the path is a function dir
				if g.isFunctionDir(runtime, files) {
					g.logger.DebugWith("Found function directory",
						"runtime", runtime,
						"name", filepath.Base(path))

					// append the function directory to our slice
					functionDirs = append(functionDirs, path)
				}
			}

			// otherwise do nothing
			return nil
		})

		if err != nil {
			return nil, errors.Wrapf(err, "Failed to walk %s runtime directory", runtime)
		}
	}

	return functionDirs, nil
}

func (g *Generator) isFunctionDir(runtime *Runtime, functionDirFiles []os.FileInfo) bool {
	for _, file := range functionDirFiles {

		// directory has at least one file related to function's runtime or a function.yaml
		if strings.HasSuffix(file.Name(), runtime.FileExtension) || file.Name() == build.FunctionConfigFileName {
			return true
		}
	}
	return false
}

func (g *Generator) buildFunctionTemplates(functionDirs []string) ([]*functiontemplates.FunctionTemplate, error) {
	var functionTemplates []*functiontemplates.FunctionTemplate

	g.logger.DebugWith("Building function templates", "numFunctions", len(functionDirs))

	for _, functionDir := range functionDirs {
		runtime := g.resolveFunctionRuntimeByFunctionPath(functionDir)
		configuration, sourceCode, err := g.getFunctionConfigAndSource(functionDir)
		if err != nil {
			g.logger.WarnWith("Failed to get function configuration and source code",
				"err", err,
				"functionDir", functionDir)

			return nil, errors.Wrap(err, "Failed to get function configuration and source code")
		}
		functionName := filepath.Base(functionDir)

		if functionName == "empty" {
			g.logger.WarnWith("Skipping empty function template", "runtimeName", runtime.Name)
			continue
		}

		if configuration.Spec.Description == "" {
			g.logger.WarnWith("Skipping function with no description", "name", functionName)
			continue
		}

		functionTemplate := g.createFunctionTemplate(configuration, functionName, sourceCode)

		g.logger.InfoWith("Appending function template",
			"functionName", functionName,
			"runtime", configuration.Spec.Runtime)
		functionTemplates = append(functionTemplates, functionTemplate)

		// HACK: allow python 3.6, 3.7, 3.8 share the same functions to avoid specific examples per runtime version
		runtimeName, runtimeVersion := common.GetRuntimeNameAndVersion(configuration.Spec.Runtime)
		if runtimeName == "python" &&
			runtimeVersion == "3.6" &&
			functionName == "helloworld" {

			// add helloworld function example to python 3.7 and 3.8
			for _, runtimeCopy := range []string{"python:3.7", "python:3.8"} {
				configurationCopy := *configuration
				configurationCopy.Spec.Runtime = runtimeCopy

				g.logger.InfoWith("Appending function template",
					"functionName", functionName,
					"runtime", configurationCopy.Spec.Runtime)
				functionTemplates = append(functionTemplates, g.createFunctionTemplate(&configurationCopy,
					functionName,
					sourceCode))
			}
		}
		g.functions[runtime.Name] = append(g.functions[runtime.Name], functionName)
	}

	return functionTemplates, nil
}

func (g *Generator) resolveFunctionRuntimeByFunctionPath(path string) *Runtime {
	for _, runtime := range g.runtimes {
		if common.StringInSlice(runtime.Name, strings.Split(path, "/")) {
			return runtime
		}
	}
	return nil
}

func (g *Generator) createFunctionTemplate(functionConfiguration *functionconfig.Config,
	functionName string,
	functionSourceCode string) *functiontemplates.FunctionTemplate {
	return &functiontemplates.FunctionTemplate{
		FunctionConfig: functionConfiguration,
		SourceCode:     functionSourceCode,
		Name: fmt.Sprintf("%s:%s",
			functionName,
			flect.Dasherize(functionConfiguration.Spec.Runtime)),
	}

}

func (g *Generator) getFunctionConfigAndSource(functionDir string) (*functionconfig.Config, string, error) {

	configuration := functionconfig.Config{}
	sourceCode := ""
	runtime := g.resolveFunctionRuntimeByFunctionPath(functionDir)
	if runtime == nil {
		return nil, "", errors.Errorf("Failed to determine runtime", "functionDir", functionDir)
	}

	// we'll know later not to look for an inline config if this is set
	configFileExists := false

	// first, look for a function.yaml file. parse it if found
	configPath := filepath.Join(functionDir, build.FunctionConfigFileName)

	if common.IsFile(configPath) {
		configFileExists = true

		configContents, err := ioutil.ReadFile(configPath)
		if err != nil {
			return nil, "", errors.Wrapf(err, "Failed to read function configuration file at %s", configPath)
		}

		if err = yaml.Unmarshal(configContents, &configuration); err != nil {
			return nil, "", errors.Wrapf(err, "Failed to unmarshal function configuration file at %s", configPath)
		}
	}

	// look for the first non-function.yaml file - this is our source code
	// (multiple-source function templates not yet supported)
	files, err := ioutil.ReadDir(functionDir)
	if err != nil {
		return nil, "", errors.Wrapf(err, "Failed to list function directory at %s", functionDir)
	}

	for _, file := range files {
		if file.Name() != build.FunctionConfigFileName {

			// we found our source code, read it
			sourcePath := filepath.Join(functionDir, file.Name())

			sourceBytes, err := ioutil.ReadFile(sourcePath)
			if err != nil {
				return nil, "", errors.Wrapf(err, "Failed to read function source code at %s", sourcePath)
			}

			if len(sourceBytes) == 0 {
				return nil, "", errors.Errorf("Function source code at %s is empty", sourcePath)
			}

			sourceCode = string(sourceBytes)

			// if there was no function.yaml, parse the inline config from the source code
			// TODO: delete it from source too
			if !configFileExists {
				if err := g.parseInlineConfiguration(sourcePath, &configuration, runtime); err != nil {
					return nil, "", errors.Wrapf(err,
						"Failed to parse inline configuration from source at %s",
						sourcePath)
				}
			}

			// stop looking at other files
			break
		}
	}

	// make sure we found source code
	if sourceCode == "" {
		return nil, "", errors.Errorf("No source files found in function directory at %s", functionDir)
	}

	// set runtime explicitly on all function configs that don't have one, i.e. for UI to consume
	if configuration.Spec.Runtime == "" {
		configuration.Spec.Runtime = runtime.Name
	}

	return &configuration, sourceCode, nil
}

func (g *Generator) parseInlineConfiguration(sourcePath string,
	configuration *functionconfig.Config,
	runtime *Runtime) error {

	blocks, err := runtime.InlineParser.Parse(sourcePath)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse inline configuration at %s", sourcePath)
	}

	configureBlock, found := blocks["configure"]
	if !found {
		g.logger.DebugWith("No configure block found in source code, returning empty config", "sourcePath", sourcePath)

		return nil
	}

	unmarshalledInlineConfigYAML, found := configureBlock.Contents[build.FunctionConfigFileName]
	if !found {
		return errors.Errorf("No function.yaml file found inside configure block at %s", sourcePath)
	}

	// must use yaml.v2 here since yaml.Marshal will err (not sure why)
	marshalledYAMLContents, err := yamlv2.Marshal(unmarshalledInlineConfigYAML)
	if err != nil {
		return errors.Wrapf(err, "Failed to marshal inline config from source at %s", sourcePath)
	}

	if err = yaml.Unmarshal(marshalledYAMLContents, configuration); err != nil {
		return errors.Wrapf(err, "Failed to unmarshal inline config from source at %s", sourcePath)
	}

	return nil
}

func (g *Generator) writeOutputFile(functionTemplates []*functiontemplates.FunctionTemplate) error {
	g.logger.DebugWith("Writing output file", "path", g.outputPath, "numFunctions", len(functionTemplates))

	outputFile, err := os.Create(g.outputPath)
	if err != nil {
		return errors.Wrap(err, "Failed to create output file")
	}

	defer func() {
		if err := outputFile.Close(); err != nil {
			panic("failed to close output file")
		}
	}()

	if err = packageTemplate.Execute(outputFile, struct {
		FunctionTemplates  []*functiontemplates.FunctionTemplate
		FunctionsByRuntime map[string][]string
	}{
		FunctionTemplates:  functionTemplates,
		FunctionsByRuntime: g.functions,
	}); err != nil {
		return errors.Wrap(err, "Failed to execute template")
	}

	outputFileInfo, err := outputFile.Stat()
	if err != nil {
		return errors.Wrap(err, "Failed to stat output file")
	}

	g.logger.InfoWith("Output file written successfully", "len", outputFileInfo.Size())

	return nil
}

func newGenerator(logger logger.Logger, examplesDir string, outputPath string) (*Generator, error) {
	newGenerator := Generator{
		logger:      logger,
		examplesDir: examplesDir,
		outputPath:  outputPath,
	}

	slashSlashParser := inlineparser.NewParser(logger, "//")
	poundParser := inlineparser.NewParser(logger, "#")

	// TODO: support java parser too i guess
	newGenerator.runtimes = []*Runtime{
		{
			InlineParser:  slashSlashParser,
			FileExtension: ".go",
			Name:          "golang",
		},
		{
			InlineParser:  slashSlashParser,
			FileExtension: ".js",
			Name:          "nodejs",
		},
		{
			InlineParser:  slashSlashParser,
			FileExtension: ".cs",
			Name:          "dotnetcore",
		},
		{
			InlineParser:  poundParser,
			FileExtension: ".py",
			Name:          "python",
		},
		{
			InlineParser:  poundParser,
			FileExtension: ".sh",
			Name:          "shell",
		},
	}

	newGenerator.functions = map[string][]string{}

	return &newGenerator, nil
}

func main() {
	examplesDir := flag.String("p", "hack/examples", "Path to examples directory")
	outputPath := flag.String("o", "pkg/dashboard/functiontemplates/generated.go", "Path to output file")
	flag.Parse()

	if err := func() error {
		loggerInstance, err := nucliozap.NewNuclioZapCmd("generator", nucliozap.DebugLevel)
		if err != nil {
			return errors.Wrap(err, "Failed to create logger")
		}

		generator, err := newGenerator(loggerInstance, *examplesDir, *outputPath)
		if err != nil {
			return errors.Wrap(err, "Failed to create generator")
		}

		if err = generator.generate(); err != nil {
			return errors.Wrap(err, "Failed to generate function template sources")
		}

		return nil
	}(); err != nil {
		errors.PrintErrorStack(os.Stderr, err, 5)

		os.Exit(1)
	}
}
