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
	// Must be in sync with pkg/processor/runtime/java/build.gradle and build dokcers
	userHandlerJarName = "user-handler.jar"
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

shadowJar {
   baseName = 'handler'
   classifier = null  // Don't append "all" to jar name
}

task nuclioJar(dependsOn: shadowJar)
`

type java struct {
	*runtime.AbstractRuntime
	versionInfo    *version.Info
	handlerJarPath string
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
		j.handlerJarPath: path.Join("opt", "nuclio", path.Base(j.handlerJarPath)),
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
	j.Logger.DebugWith(
		"Java OnAfterStagingDirCreated", "buildPath", buildPath, "stagingDir", stagingDir)

	handlerBuildDir := path.Join(stagingDir, "java-handler-build")
	if err := os.Mkdir(handlerBuildDir, 0777); err != nil {
		return errors.Wrap(err, "Can't create handler build dir")
	}

	userJarPath := path.Join(handlerBuildDir, userHandlerJarName)
	if err := j.buildUserJar(buildPath, userJarPath); err != nil {
		return err
	}

	var err error
	handlerImageName := fmt.Sprintf("nuclio/handler-builder-java-onbuild:%s-%s", j.versionInfo.Label, j.versionInfo.Arch)
	j.handlerJarPath, err = j.runDockerJavaBuild(handlerBuildDir, handlerImageName)

	return err
}

func (j *java) getFunctionHandler() string {
	return "Handler"
}

func (j *java) GetProcessorBaseImage() (string, error) {
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

	var buf bytes.Buffer
	out := io.MultiWriter(&buf, buildFile)
	err = buildTemplate.Execute(out, data)

	j.Logger.DebugWith("Created gradle build file", "path", buildFilePath, "content", buf.String())

	return err
}

func (j *java) createDockerFile(dockerfilePath string, imageName string) error {
	if !j.FunctionConfig.Spec.Build.NoBaseImagesPull {
		// pull the onbuild image we need to build the processor builder
		if err := j.DockerClient.PullImage(imageName); err != nil {
			return errors.Wrap(err, "Failed to pull onbuild image for java")
		}
	}

	dockerFileContent := fmt.Sprintf("FROM %s", imageName)
	j.Logger.DebugWith("Creating docker file", "content", dockerFileContent)

	return ioutil.WriteFile(dockerfilePath, []byte(dockerFileContent), 0600)
}

func (j *java) isFileWithExtension(filePath string, extension string) bool {
	return common.IsFile(filePath) && strings.ToLower(path.Ext(filePath)) == strings.ToLower(extension)
}

/* copyJavaSources copies java sources to the staging directory.

If buildPath is a directory, it copies it "as is" to stagingBuildDir.
Otherwise if the input is a java source file it'll be copied to <stagingBuildDir>/src/main/java.
Any other option for buildPath will result in an error
*/
func (j *java) copyJavaSources(buildPath string, stagingBuildDir string) error {
	switch {
	case common.IsDir(buildPath):
		if _, err := util.CopyDir(buildPath, stagingBuildDir); err != nil {
			return errors.Wrapf(err, "Can't copy sources %q -> %q", buildPath, stagingBuildDir)
		}
	case j.isFileWithExtension(buildPath, ".java"):
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

func (j *java) findHandlerJar(dirName string) (string, error) {
	var jarFiles []string
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if j.isFileWithExtension(path, ".jar") && !j.isSDKJar(path) {
			jarFiles = append(jarFiles, path)
		}
		return nil
	}

	if err := filepath.Walk(dirName, walkFunc); err != nil {
		return "", err
	}

	switch len(jarFiles) {
	case 1:
		return jarFiles[0], nil
	case 0:
		return "", nil
	default:
		return "", errors.Errorf("too many jar files: %v", jarFiles)
	}
}

func (j *java) isSDKJar(jarPath string) bool {
	return j.isFileWithExtension(jarPath, ".jar") && strings.HasPrefix(path.Base(jarPath), "nuclio-sdk-")
}

func (j *java) buildUserJar(buildPath, userJarPath string) error {
	// If it's a jar - use it
	if j.isFileWithExtension(buildPath, ".jar") {
		j.Logger.InfoWith("Using existing jar", "path", buildPath)
		return util.CopyFile(buildPath, userJarPath)
	}

	// If we have handler.jar in this directory - use it
	jarFilePath := path.Join(buildPath, "handler.jar")
	if common.IsFile(jarFilePath) {
		j.Logger.InfoWith("Using existing jar", "path", jarFilePath)
		return util.CopyFile(jarFilePath, userJarPath)
	}

	var err error

	// If it's a directory with a single jar file - use it (case of archives)
	jarFilePath, err = j.findHandlerJar(buildPath)
	if jarFilePath != "" && err == nil {
		j.Logger.InfoWith("Using existing jar", "path", jarFilePath)
		return util.CopyFile(jarFilePath, userJarPath)
	}

	return j.buildUserJarFromSource(buildPath, userJarPath)
}

func (j *java) buildUserJarFromSource(buildPath, userJarPath string) error {
	userBuildDir := path.Join(j.StagingDir, "java-user-build")

	if err := j.copyJavaSources(buildPath, userBuildDir); err != nil {
		return err
	}

	err := j.createBuildFile(userBuildDir)
	if err != nil {
		return errors.Wrap(err, "Can't create build file")
	}
	userImageName := fmt.Sprintf("nuclio/user-builder-java-onbuild:%s-%s", j.versionInfo.Label, j.versionInfo.Arch)
	userJarFilePath, err := j.runDockerJavaBuild(userBuildDir, userImageName)
	if err != nil {
		return err
	}

	return util.CopyFile(userJarFilePath, userJarPath)
}

// Run a build using buildImageName, return output jar name and error
func (j *java) runDockerJavaBuild(contextDir, onBuildImageName string) (string, error) {
	dockerfilePath := path.Join(contextDir, "Dockerfile.nuclio-build")
	if err := j.createDockerFile(dockerfilePath, onBuildImageName); err != nil {
		return "", errors.Wrap(err, "Can't create build docker file")
	}

	imageName := fmt.Sprintf("nuclio/handler-builder-java-%s", xid.New())

	if err := j.DockerClient.Build(&dockerclient.BuildOptions{
		Image:          imageName,
		DockerfilePath: dockerfilePath,
		ContextDir:     contextDir,
	}); err != nil {
		return "", errors.Wrap(err, "Failed to build handler")
	}

	defer j.DockerClient.RemoveImage(imageName)

	outputDirName := "nuclio-build"
	objectsToCopy := map[string]string{
		fmt.Sprintf("/%s", outputDirName): contextDir,
	}

	if err := j.DockerClient.CopyObjectsFromImage(imageName, objectsToCopy, true); err != nil {
		return "", errors.Wrap(err, "Failed to copy objects from image")
	}

	buildOutputDir := path.Join(contextDir, outputDirName)

	jarPath, err := j.findHandlerJar(path.Join(buildOutputDir, "build"))
	switch {
	case err != nil:
		return "", errors.Wrapf(err, "Can't find handler jar in %s", buildOutputDir)
	case jarPath == "": // not found, probably build error
		handlerBuildLogPath := path.Join(buildOutputDir, "build.log")
		handlerBuildLogContents, err := ioutil.ReadFile(handlerBuildLogPath)
		if err != nil {
			return "", errors.Wrap(err, "Failed to read build log contents")
		}

		return "", errors.Errorf("Failed to build function:\n%s", string(handlerBuildLogContents))
	}

	return jarPath, nil
}
