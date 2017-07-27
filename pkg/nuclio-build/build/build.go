package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/nuclio-build/util"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
)

type Options struct {
	Verbose         bool
	FunctionPath    string
	OutputType      string
	OutputName      string
	Version         string
	NuclioSourceDir string
	NuclioSourceURL string
	PushRegistry    string
}

type Builder struct {
	logger  nuclio.Logger
	options *Options
}

const (
	defaultBuilderImage     = "golang:1.8"
	processorConfigFileName = "processor.yaml"
	buildConfigFileName     = "build.yaml"
)

type config struct {
	Name    string `mapstructure:"name"`
	Handler string `mapstructure:"handler"`
	Build   struct {
		Image    string   `mapstructure:"image"`
		Packages []string `mapstructure:"packages"`
	} `mapstructure:"build"`
}

type buildStep struct {
	Message string
	Func    func() error
}

func NewBuilder(parentLogger nuclio.Logger, options *Options) *Builder {
	return &Builder{
		logger:  parentLogger.GetChild("builder").(nuclio.Logger),
		options: options,
	}
}

func (b *Builder) Build() error {
	config, err := b.readConfig(filepath.Join(b.options.FunctionPath, processorConfigFileName),
		filepath.Join(b.options.FunctionPath, buildConfigFileName))

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

func (b *Builder) readProcessorConfigFile(c *config, fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		c.Name = "handler"
		c.Handler = ""

		return nil
	}

	// try to read the configuration file
	return b.readConfigFile(c, "function", fileName)
}

func (b *Builder) readBuildConfigFile(c *config, fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		c.Build.Image = defaultBuilderImage
		c.Build.Packages = []string{}

		return nil
	}

	// try to read the configuration file
	return b.readConfigFile(c, "", fileName)
}

func adjactive(n int) string {
	switch n {
	case 0:
		return "no"
	case 1: // noop
	default:
		return "too many"
	}
	return "" // make compiler happy
}

func (b *Builder) readConfig(processorConfigPath, buildFile string) (*config, error) {
	c := config{}
	if err := b.readProcessorConfigFile(&c, processorConfigPath); err != nil {
		return nil, err
	}
	if len(c.Handler) == 0 || len(c.Name) == 0 {
		pkgs, handlers, err := util.ParseHandler(b.options.FunctionPath)
		if err != nil {
			return nil, errors.Wrapf(err, "can't find handlers in %q", b.options.FunctionPath)
		}
		if len(handlers) != 1 {
			adj := adjactive(len(handlers))
			return nil, errors.Wrapf(err, "%s handlers found in %q", adj, b.options.FunctionPath)
		}
		if len(pkgs) != 1 {
			adj := adjactive(len(pkgs))
			return nil, errors.Wrapf(err, "%s packages found in %q", adj, b.options.FunctionPath)
		}

		if len(c.Handler) == 0 {
			c.Handler = handlers[0]
		}
		if len(c.Name) == 0 {
			c.Name = pkgs[0]
		}
	}
	if err := b.readBuildConfigFile(&c, buildFile); err != nil {
		return nil, err
	}
	return &c, nil
}
