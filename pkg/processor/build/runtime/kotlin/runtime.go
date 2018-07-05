/*
Copyright 2018 The Nuclio Authors.

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

package kotlin

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

type kotlin struct {
	*runtime.AbstractRuntime
}

// GetName returns the name of the runtime, including version if applicable
func (k *kotlin) GetName() string {
	return "kotlin"
}

// OnAfterStagingDirCreated will build jar if the source is a Java file
// It will set generatedJarPath field
func (k *kotlin) OnAfterStagingDirCreated(stagingDir string) error {

	// create a build script alongside the user's code. if user provided a script, it'll use that
	return k.createGradleBuildScript(stagingDir)
}

// GetProcessorDockerfileInfo returns information required to build the processor Dockerfile
func (k *kotlin) GetProcessorDockerfileInfo(versionInfo *version.Info) (*runtime.ProcessorDockerfileInfo, error) {
	processorDockerfileInfo := runtime.ProcessorDockerfileInfo{}

	// format the onbuild image
	processorDockerfileInfo.OnbuildImage = fmt.Sprintf("nuclio/handler-builder-kotlin-onbuild:%s-%s",
		versionInfo.Label,
		versionInfo.Arch)

	// set the default base image
	processorDockerfileInfo.BaseImage = "openjdk:9-jre-slim"
	processorDockerfileInfo.OnbuildArtifactPaths = map[string]string{
		"/home/gradle/bin/processor":                                  "/usr/local/bin/processor",
		"/home/gradle/src/wrapper/build/libs/nuclio-kotlin-wrapper.jar": "/opt/nuclio/nuclio-kotlin-wrapper.jar",
	}

	return &processorDockerfileInfo, nil
}

func (k *kotlin) createGradleBuildScript(stagingBuildDir string) error {
	gradleBuildScriptPath := path.Join(stagingBuildDir, "handler", "build.gradle")

	// if user supplied gradle build script - use it
	if common.IsFile(gradleBuildScriptPath) {
		k.Logger.DebugWith("Found user gradle build script, using it", "path", gradleBuildScriptPath)
		return nil
	}

	gradleBuildScriptTemplate, err := template.New("gradleBuildScript").Parse(k.getGradleBuildScriptTemplateContents())
	if err != nil {
		return errors.Wrap(err, "Failed to create gradle build script template")
	}

	buildFile, err := os.Create(gradleBuildScriptPath)
	if err != nil {
		return errors.Wrapf(err, "Failed to create gradle build script file @ %s", gradleBuildScriptPath)
	}

	defer buildFile.Close() // nolint: errcheck

	dependencies, err := k.parseDependencies(k.FunctionConfig.Spec.Build.Dependencies)
	if err != nil {
		return errors.Wrap(err, "Failed to parse dependencies")
	}

	buildAttributes, err := newBuildAttributes(k.FunctionConfig.Spec.Build.RuntimeAttributes)
	if err != nil {
		return errors.Wrap(err, "Failed to get build attributes repositories")
	}

	data := map[string]interface{}{
		"Dependencies": dependencies,
		"Repositories": buildAttributes.Repositories,
	}

	var gradleBuildScriptTemplateBuffer bytes.Buffer
	err = gradleBuildScriptTemplate.Execute(io.MultiWriter(&gradleBuildScriptTemplateBuffer, buildFile), data)

	k.Logger.DebugWith("Created gradle build script",
		"path", gradleBuildScriptPath,
		"content", gradleBuildScriptTemplateBuffer.String(),
		"data", data)

	return err
}

func (k *kotlin) getGradleBuildScriptTemplateContents() string {
	return `plugins {
  id 'com.github.johnrengelman.shadow' version '2.0.2'
  id 'java'
  id 'org.jetbrains.kotlin.jvm' version '1.2.41'
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
    compile "org.jetbrains.kotlin:kotlin-stdlib-jdk8"
}

compileKotlin {
    kotlinOptions.jvmTarget = "1.8"
}
compileTestKotlin {
    kotlinOptions.jvmTarget = "1.8"
}

shadowJar {
   baseName = 'user-handler'
   classifier = null  // Don't append "all" to jar name
}

task userHandler(dependsOn: shadowJar)
`
}

func (k *kotlin) parseDependencies(rawDependencies []string) ([]dependency, error) {
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
