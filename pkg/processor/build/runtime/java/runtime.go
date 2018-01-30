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
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/nuclio/nuclio/pkg/common"
	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/build/runtime"
	"github.com/nuclio/nuclio/pkg/processor/build/util"
	"github.com/nuclio/nuclio/pkg/version"

	"github.com/rs/xid"
)

const (
	buildFileName = "build.gradle"
)

var buildTemplateCode = `
plugins {
  id 'com.github.johnrengelman.shadow' version '2.0.2'
  id 'java'
}

repositories {
    mavenCentral()
}

dependencies {
	{{ range .Dependencies }}
	compile group: '{{.Group}}', name: '{{.Name}}', version: '{{.Version}}'
	{{ end }}

    compile files('./nuclio-sdk-1.0-SNAPSHOT.jar')
}

jar {
  manifest {
    attributes(
      'Main-Class': '{{.Handler}}'
    )
  }
}

// Output jar in this directory
tasks.withType(Jar) {
    destinationDir = file("$rootDir")
}

shadowJar {
   baseName = 'handler'
   classifier = null  // Don't append "all" to jar name
}
`

type java struct {
	*runtime.AbstractRuntime
	versionInfo *version.Info
	jarPath     string
}

// DetectFunctionHandlers returns a list of all the handlers
// in that directory given a path holding a function (or functions)
func (j *java) DetectFunctionHandlers(functionPath string) ([]string, error) {
	return []string{j.getFunctionHandler()}, nil
}

// GetProcessorImageObjectPaths returns a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (j *java) GetProcessorImageObjectPaths() map[string]string {
	return map[string]string{
		j.jarPath: path.Join("opt", "nuclio", "handler", path.Base(j.jarPath)),
	}
}

// GetExtension returns the source extension of the runtime (e.g. .go)
func (j *java) GetExtension() string {
	return "jar"
}

// GetName returns the name of the runtime, including version if applicable
func (j *java) GetName() string {
	return "java"
}

// OnAfterStagingDirCreated will build jar if the source is a Java file
// It will set generatedJarPath field
func (j *java) OnAfterStagingDirCreated(stagingDir string) error {
	buildPath := j.FunctionConfig.Spec.Build.Path
	if j.isFile(buildPath, ".jar") {
		j.jarPath = buildPath
		return nil
	}

	stagingBuildDir := path.Join(stagingDir, "java-build")
	j.Logger.InfoWith("Creating Jar", "buildDir", stagingBuildDir, "path", buildPath)

	if err := j.copySourceToStaging(buildPath, stagingBuildDir); err != nil {
		return err
	}

	err := j.createBuildFile(stagingBuildDir)
	if err != nil {
		return errors.Wrap(err, "Can't create build file")
	}

	dockerfilePath := path.Join(stagingBuildDir, "Dockerfile.nuclio-build-handler")
	if err := j.createDockerFile(dockerfilePath); err != nil {
		return errors.Wrap(err, "Can't create build docker file")
	}

	imageName := fmt.Sprintf("nuclio/handler-builder-java-%s", xid.New())

	if err := j.DockerClient.Build(&dockerclient.BuildOptions{
		ImageName:      imageName,
		DockerfilePath: dockerfilePath,
		ContextDir:     stagingBuildDir,
	}); err != nil {
		return errors.Wrap(err, "Failed to build handler")
	}

	defer j.DockerClient.RemoveImage(imageName)

	j.jarPath = path.Join(stagingBuildDir, "handler.jar")
	handlerBuildLogPath := path.Join(stagingBuildDir, "handler_build.log")

	objectsToCopy := map[string]string{
		"/nuclio-build/handler.jar": j.jarPath,
		"/handler_build.log":        handlerBuildLogPath,
	}

	if err := j.DockerClient.CopyObjectsFromImage(imageName, objectsToCopy, true); err != nil {
		return errors.Wrap(err, "Failed to copy objects from image")
	}

	// if handler doesn't exist, return why the build failed
	if !common.FileExists(j.jarPath) {
		// read the build log
		handlerBuildLogContents, err := ioutil.ReadFile(handlerBuildLogPath)
		if err != nil {
			return errors.Wrap(err, "Failed to read build log contents")
		}

		return errors.Errorf("Failed to build function:\n%s", string(handlerBuildLogContents))
	}

	return nil
}

func (j *java) getFunctionHandler() string {
	// "/path/to/staging/handler.jar" -> "handler.jar"
	functionFileName := path.Base(j.jarPath)
	return fmt.Sprintf("%s:%s", functionFileName, "handler")
}

func (j *java) GetProcessorBaseImageName() (string, error) {
	return fmt.Sprintf("nuclio/handler-java:%s-%s",
		j.versionInfo.Label,
		j.versionInfo.Arch), nil
}

// "reverser.jar:Reverser" -> "Reverser"
func (j *java) handlerClassName(handler string) string {
	fields := strings.Split(handler, ":")
	if len(fields) == 1 {
		return fields[0]
	}
	return fields[1]
}

func (j *java) createBuildFile(stagingBuildDir string) error {
	buildFilePath := path.Join(stagingBuildDir, buildFileName)
	if common.IsFile(buildFilePath) {
		j.Logger.InfoWith("Found user build file, using it", "path", buildFilePath)
		return nil
	}

	buildTemplate, err := template.New("build").Parse(buildTemplateCode)
	if err != nil {
		return err
	}

	buildFile, err := os.Create(buildFilePath)
	if err != nil {
		return err
	}

	defer buildFile.Close()

	data := map[string]interface{}{
		"Dependencies": j.FunctionConfig.Spec.Build.Dependencies,
		"Handler":      j.handlerClassName(j.FunctionConfig.Spec.Handler),
	}
	return buildTemplate.Execute(buildFile, data)
}

func (j *java) createDockerFile(dockerfilePath string) error {
	imageName := fmt.Sprintf("nuclio/handler-builder-java-onbuild:%s-%s", j.versionInfo.Label, j.versionInfo.Arch)
	if !j.FunctionConfig.Spec.Build.NoBaseImagesPull {
		// pull the onbuild image we need to build the processor builder
		if err := j.DockerClient.PullImage(imageName); err != nil {
			return errors.Wrap(err, "Failed to pull onbuild image for java")
		}
	}

	dockerFileContent := fmt.Sprintf("FROM %s", imageName)

	return ioutil.WriteFile(dockerfilePath, []byte(dockerFileContent), 0600)
}

func (j *java) isFile(filePath, extension string) bool {
	return common.IsFile(filePath) && strings.ToLower(path.Ext(filePath)) == strings.ToLower(extension)
}

func (j *java) createJavaSourceDir() error {
	// Java sources *must* be under src/main/java
	// TODO: Should we use gradle's sourceSets in current directory?
	// https://docs.gradle.org/current/userguide/java_plugin.html
	// (Probably no since there might be more than one Java file in the root directory)
	javaSrcDirPath := path.Join(j.StagingDir, "src/main/java")

	if err := os.MkdirAll(javaSrcDirPath, 0777); err != nil {
		return errors.Wrap(err, "Can't create Java source directory")
	}

	buildPath := j.FunctionConfig.Spec.Build.Path
	var filesToCopy []string
	var err error
	if common.IsFile(buildPath) {
		filesToCopy = append(filesToCopy, buildPath)
	} else {
		filesToCopy, err = filepath.Glob(path.Join(buildPath, "*.java"))
		if err != nil {
			return errors.Wrapf(err, "Can't find Java files in %q", buildPath)
		}
	}

	if len(filesToCopy) == 0 {
		return errors.Errorf("Can't find Java files in %q", buildPath)
	}

	for _, filePath := range filesToCopy {
		destPath := path.Join(javaSrcDirPath, path.Base(filePath))
		if err := util.CopyFile(filePath, destPath); err != nil {
			return errors.Wrap(err, "Can't copy file to Java source directory")
		}
	}

	return nil
}

func (j *java) copySourceToStaging(buildPath, stagingBuildDir string) error {
	switch {
	case common.IsDir(buildPath):
		if _, err := util.CopyDir(buildPath, stagingBuildDir); err != nil {
			return errors.Wrapf(err, "Can't copy sources %q -> %q", buildPath, stagingBuildDir)
		}
	case j.isFile(buildPath, ".java"):
		javaSrcDir := path.Join(stagingBuildDir, "src/main/java")
		if err := os.MkdirAll(javaSrcDir, 0777); err != nil {
			return err
		}
		destSrcPath := path.Join(javaSrcDir, path.Base(buildPath))
		if err := util.CopyFile(buildPath, destSrcPath); err != nil {
			return errors.Wrapf(err, "Can't copy source %q -> %q", buildPath, destSrcPath)
		}
	default:
		return errors.Errorf("Don't know how to build %q", buildPath)
	}

	return nil
}
