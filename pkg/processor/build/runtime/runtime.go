package runtime

import "github.com/nuclio/nuclio-sdk"

type Runtime interface {

	// returns the image name of the default processor base image
	GetDefaultProcessorBaseImage() string

	// given a path holding a function (or functions) returns a list of all the handlers
	// in that directory
	DetectFunctionHandlers(functionPath string) ([]string, error)

	// given a staging directory, prepares anything it may need in that directory
	// towards building a functioning processor
	OnAfterStagingDirCreated(stagingDir string) error

	// generate the contents of the processor configuration file
	GetProcessorConfigFileContents() string

	// return a map of objects the runtime needs to copy into the processor image
	// the key can be a dir, a file or a url of a file
	// the value is an absolute path into the docker image
	GetProcessorImageObjectPaths() map[string]string
}

type Configuration interface {
	GetFunctionPath() string
}

type Factory interface {
	Create(logger nuclio.Logger, configuration Configuration) (Runtime, error)
}

type AbstractRuntime struct {
	Logger nuclio.Logger
	Configuration Configuration
}

func (ar *AbstractRuntime) OnAfterStagingDirCreated(stagingDir string) error {
	return nil
}

func (ar *AbstractRuntime) GetProcessorConfigFileContents() string {
	return ""
}

// return a map of objects the runtime needs to copy into the processor image
// the key can be a dir, a file or a url of a file
// the value is an absolute path into the docker image
func (ar *AbstractRuntime) GetProcessorImageObjectPaths() map[string]string {
	return nil
}
