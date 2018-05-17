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
	"io"
	"os"
	"path"
	"text/template"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
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

	repositories, err := j.getBuildRepositories()
	if err != nil {
		return errors.Wrap(err, "Failed to get build repositories")
	}

	data := map[string]interface{}{
		"Dependencies": dependencies,
		"Repositories": repositories,
	}

	var gradleBuildScriptTemplateBuffer bytes.Buffer
	err = gradleBuildScriptTemplate.Execute(io.MultiWriter(&gradleBuildScriptTemplateBuffer, buildFile), data)

	j.Logger.DebugWith("Created gradle build script",
		"path", gradleBuildScriptPath,
		"content", gradleBuildScriptTemplateBuffer.String())

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

// GetProcessorDockerfilePath returns the contents of the appropriate Dockerfile, with which we'll build
// the processor image
func (j *java) GetProcessorDockerfileContents() string {
	return `ARG NUCLIO_LABEL=latest
ARG NUCLIO_ARCH=amd64
ARG NUCLIO_BASE_IMAGE=openjdk:9-jre-slim
ARG NUCLIO_ONBUILD_IMAGE=nuclio/handler-builder-java-onbuild:${NUCLIO_LABEL}-${NUCLIO_ARCH}

# Supplies processor, handler.jar
FROM ${NUCLIO_ONBUILD_IMAGE} as builder

# Supplies uhttpc, used for healthcheck
FROM nuclio/uhttpc:0.0.1-amd64 as uhttpc

# From the base image
FROM ${NUCLIO_BASE_IMAGE}

# Copy required objects from the suppliers
COPY --from=builder /home/gradle/bin/processor /usr/local/bin/processor
COPY --from=builder /home/gradle/src/wrapper/build/libs/nuclio-java-wrapper.jar /opt/nuclio/nuclio-java-wrapper.jar
COPY --from=uhttpc /home/nuclio/bin/uhttpc /usr/local/bin/uhttpc

# Readiness probe
HEALTHCHECK --interval=1s --timeout=3s CMD /usr/local/bin/uhttpc --url http://localhost:8082/ready || exit 1

# Run processor with configuration and platform configuration
CMD [ "processor", "--config", "/etc/nuclio/config/processor/processor.yaml", "--platform-config", "/etc/nuclio/config/platform/platform.yaml" ]
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

func (j *java) getBuildRepositories() ([]string, error) {

	// try to get repositories
	if repositories, hasRepositories := j.FunctionConfig.Spec.Build.RuntimeAttributes["repositories"]; hasRepositories {

		if typedRepositories, validRepositories := repositories.([]string); validRepositories {
			return typedRepositories, nil
		}

		return nil, errors.New("Build repositories must be a list of strings")
	}

	return []string{"mavenCentral()"}, nil
}
