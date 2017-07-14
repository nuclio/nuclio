package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio-sdk/logger"
	"github.com/spf13/viper"
	"github.com/pkg/errors"
	"github.com/nuclio/nuclio-tools/cmd/nuclio-build/util"
)

type Options struct {
	Verbose         bool
	FunctionPath    string
	OutputType      string
	OutputName      string
	Version         string
	NuclioSourceDir string
	NuclioSourceURL string
}

type Builder struct {
	logger  logger.Logger
	options *Options
}

const (
	defaultBuilderImage = "golang:1.8"
	functionFileName    = "function.yml"
	buildFileName       = "build.yml"
)

type config struct {
	Name    string `mapstructure:"name"`
	Handler string `mapstructure:"handler"`
	Build struct {
		Image    string   `mapstructure:"image"`
		Packages []string `mapstructure:"packages"`
	} `mapstructure:"build"`
}

type buildStep struct {
	Message string
	Func    func() error
}

func NewBuilder(parentLogger logger.Logger, options *Options) *Builder {
	return &Builder{
		logger:  parentLogger.GetChild("builder").(logger.Logger),
		options: options,
	}
}

func (b *Builder) Build() error {
	config, err := b.readConfig(filepath.Join(b.options.FunctionPath, functionFileName),
		filepath.Join(b.options.FunctionPath, buildFileName))

	if err != nil {
		return errors.Wrap(err, "Unable to read Config")
	}

	b.logger.Info("Building run environment")

	env, err := newEnv(b.logger, config, b.options)
	if err != nil {
		return errors.Wrap(err, "Failed to create env")
	}

	if err := b.buildDockerSteps(env, b.options.OutputType == "docker"); err != nil {
		return err
	}

	if b.options.OutputType == "binary" {
		if err := util.CopyFile(env.getBinaryPath(), env.outputName); err != nil {
			return err
		}
	}

	b.logger.InfoWith("Outputting",
		"output_type", b.options.OutputType,
		"output_name", env.outputName)

	return nil
}

func (b *Builder) buildDockerSteps(env *env, outputToImage bool) error {
	b.logger.Debug("Creating docker helper")

	docker, err := newDockerHelper(b.logger, env)
	if err != nil {
		return errors.Wrap(err, "Error building docker helper")
	}

	defer docker.close()

	buildSteps := []buildStep{
		{Message: "Running docker onbuild",
			Func: docker.createOnBuildImage},
		{Message: "Running docker binary build",
			Func: docker.createBuilderImage},
	}

	if outputToImage {
		buildSteps = append(buildSteps, buildStep{
			Message: "Creating output container " + env.outputName,
			Func:    docker.createProcessorImage})
	}

	for _, step := range buildSteps {
		b.logger.InfoWith("Running build step", "message", step.Message)
		if err := step.Func(); err != nil {
			return errors.Wrap(err, "Error while "+step.Message)
		}
	}

	return nil
}

func (b *Builder) readConfigFile(c *config, key string, fileName string) error {
	b.logger.DebugWith("Reading config file", "path", fileName, "key", key)

	v := viper.New()
	v.SetConfigFile(fileName)
	if err := v.ReadInConfig(); err != nil {
		return errors.Wrapf(err, "Unable to read %q configuration", fileName)
	}

	if key != "" {
		v = v.Sub(key)

		if v == nil {
			return fmt.Errorf("Configuration file %s has no key %s", fileName, key)
		}
	}

	if err := v.Unmarshal(c); err != nil {
		return errors.Wrapf(err, "Unable to unmarshal %q configuration", fileName)
	}
	return nil
}

func (b *Builder) readFunctionFile(c *config, fileName string) error {
	return b.readConfigFile(c, "function", fileName)
}

func (b *Builder) readBuildFile(c *config, fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		c.Build.Image = defaultBuilderImage
	}
	return b.readConfigFile(c, "", fileName)
}

func (b *Builder) readConfig(functionFile, buildFile string) (*config, error) {
	c := config{}
	if err := b.readFunctionFile(&c, functionFile); err != nil {
		return nil, err
	}
	if err := b.readBuildFile(&c, buildFile); err != nil {
		return nil, err
	}
	return &c, nil
}
