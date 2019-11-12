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

package java

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"text/template"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/version"
)

type java struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (j *java) GetName() string {
	return "java"
}

// OnAfterStagingDirCreated will build jar if the source is a Java file
// It will set generatedJarPath field
func (j *java) OnAfterStagingDirCreated(stagingDir string) error {
	// create a build script alongside the user's code. if user provided a script, it'll use that
	return j.createGradleBuildScript(stagingDir)
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (j *java) GetProcessorDockerfileInfo(versionInfo *version.Info,
	registryURL string) (*runtime.ProcessorDockerfileInfo, error) {

	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}
	processorDockerfileInfo.BaseImage = "openjdk:9-jre-slim"

	// fill onbuild artifact
	artifact := runtime.Artifact{
		Name: "java-onbuild",
		Image: fmt.Sprintf("%s/nuclio/handler-builder-java-onbuild:%s-%s",
			j.GetRegistry(registryURL),
			versionInfo.Label,
			versionInfo.Arch),
		Paths: map[string]string{
			"/home/gradle/bin/processor":                                  "/usr/local/bin/processor",
			"/home/gradle/src/wrapper/build/libs/nuclio-java-wrapper.jar": "/opt/nuclio/nuclio-java-wrapper.jar",
		},
	}
	processorDockerfileInfo.OnbuildArtifacts = []runtime.Artifact{artifact}

	return &processorDockerfileInfo, nil
}

func (j *java) createGradleBuildScript(stagingBuildDir string) error {
	gradleBuildScriptPath := path.Join(stagingBuildDir, "handler", "build.gradle")

	// if user supplied gradle build script - use it
	if common.IsFile(gradleBuildScriptPath) {
		j.Logger.DebugWith("Found user gradle build script, using it", "path", gradleBuildScriptPath)
		return nil
	}

	gradleBuildScriptTemplate, err := template.New("gradleBuildScript").Parse(j.getGradleBuildScriptTemplateContents())
	if err != nil {
		return errors.Wrap(err, "Failed to create gradle build script template")
	}

	buildFile, err := os.Create(gradleBuildScriptPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to create gradle build script file @ %s", gradleBuildScriptPath)
	}

	defer buildFile.Close() // nolint: errcheck

	dependencies, err := j.parseDependencies(j.FunctionConfig.Spec.Build.Dependencies)
	if err != nil {
		return errors.Wrap(err, "Failed to parse dependencies")
	}

	buildAttributes, err := newBuildAttributes(j.FunctionConfig.Spec.Build.RuntimeAttributes)
	if err != nil {
		return errors.Wrap(err, "Failed to get build attributes repositories")
	}

	data := map[string]interface{}{
		"Dependencies": dependencies,
		"Repositories": buildAttributes.Repositories,
	}

	var gradleBuildScriptTemplateBuffer bytes.Buffer
	err = gradleBuildScriptTemplate.Execute(io.MultiWriter(&gradleBuildScriptTemplateBuffer, buildFile), data)

	j.Logger.DebugWith("Created gradle build script",
		"path", gradleBuildScriptPath,
		"content", gradleBuildScriptTemplateBuffer.String(),
		"data", data)

	return err
}

func (j *java) getGradleBuildScriptTemplateContents() string {
	return `plugins {
  id 'com.github.johnrengelman.shadow' version '2.0.2'
  id 'java'
}

repositories {
	{{ range .Repositories }}
	{{ . }}
	{{ end }}
}

dependencies {
	{{ range .Dependencies }}
	compile group: '{{.Group}}', name: '{{.Name}}', version: '{{.Version}}'
	{{ end }}

    compile files('./nuclio-sdk-1.0-SNAPSHOT.jar')
}

shadowJar {
   baseName = 'user-handler'
   classifier = null  // Don't append "all" to jar name
}

task userHandler(dependsOn: shadowJar)
`
}

func (j *java) parseDependencies(rawDependencies []string) ([]dependency, error) {
	var dependencies []dependency

	for _, rawDependency := range rawDependencies {
		dependency, err := newDependency(rawDependency)
		if err != nil {
			return nil, errors.Wrap(err, "Failed to create dependency")
		}

		dependencies = append(dependencies, *dependency)
	}

	return dependencies, nil
}
