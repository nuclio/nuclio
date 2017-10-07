package kube

import (
	"os"
	"path/filepath"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/platform"

	"github.com/mitchellh/go-homedir"
	"github.com/nuclio/nuclio-sdk"
)

type Platform struct {
	*platform.AbstractPlatform
	deployer       *deployer
	getter         *getter
	updater        *updater
	deleter        *deleter
	kubeconfigPath string
	consumer       *consumer
}

// NewPlatform instantiates a new kubernetes platform
func NewPlatform(parentLogger nuclio.Logger, kubeconfigPath string) (*Platform, error) {
	newPlatform := &Platform{}

	// create base
	newAbstractPlatform, err := platform.NewAbstractPlatform(parentLogger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create abstract platform")
	}

	// init platform
	newPlatform.AbstractPlatform = newAbstractPlatform
	newPlatform.kubeconfigPath = kubeconfigPath

	// create consumer
	newPlatform.consumer, err = newConsumer(newPlatform.Logger, kubeconfigPath)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create consumer")
	}

	// create deployer
	newPlatform.deployer, err = newDeployer(newAbstractPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create deployer")
	}

	// create getter
	newPlatform.getter, err = newGetter(newAbstractPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create getter")
	}

	// create deleter
	newPlatform.deleter, err = newDeleter(newAbstractPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create deleter")
	}

	// create updater
	newPlatform.updater, err = newUpdater(newAbstractPlatform.Logger, newPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create updater")
	}

	return newPlatform, nil
}

// Deploy will deploy a processor image to the platform (optionally building it, if source is provided)
func (p *Platform) DeployFunction(deployOptions *platform.DeployOptions) (*platform.DeployResult, error) {

	// wrap the deployer's deploy with the base HandleDeployFunction to provide lots of
	// common functionality
	return p.HandleDeployFunction(deployOptions, func() (*platform.DeployResult, error) {
		return p.deployer.deploy(p.consumer, deployOptions)
	})
}

// GetFunctions will return deployed functions
func (p *Platform) GetFunctions(getOptions *platform.GetOptions) ([]platform.Function, error) {
	return p.getter.get(p.consumer, getOptions)
}

// UpdateFunction will update a previously deployed function
func (p *Platform) UpdateFunction(updateOptions *platform.UpdateOptions) error {
	return p.updater.update(p.consumer, updateOptions)
}

// DeleteFunction will delete a previously deployed function
func (p *Platform) DeleteFunction(deleteOptions *platform.DeleteOptions) error {
	return p.deleter.delete(p.consumer, deleteOptions)
}

func IsInCluster() bool {
	return len(os.Getenv("KUBERNETES_SERVICE_HOST")) != 0 && len(os.Getenv("KUBERNETES_SERVICE_PORT")) != 0
}

func GetKubeconfigPath(platformConfiguration interface{}) string {
	var kubeconfigPath string

	// if kubeconfig is passed in the options, use that
	if platformConfiguration != nil {

		// it might not be a kube configuration
		if _, ok := platformConfiguration.(*Configuration); ok {
			kubeconfigPath = platformConfiguration.(*Configuration).KubeconfigPath
		}
	}

	// do we still not have a kubeconfig path? try environment variable
	if kubeconfigPath == "" {
		kubeconfigPath = os.Getenv("KUBECONFIG")
	}

	// still don't? try looking @ home directory
	if kubeconfigPath == "" {
		kubeconfigPath = getKubeconfigFromHomeDir()
	}

	return kubeconfigPath
}

func getKubeconfigFromHomeDir() string {
	homeDir, err := homedir.Dir()
	if err != nil {
		return ""
	}

	homeKubeConfigPath := filepath.Join(homeDir, ".kube", "config")

	// if the file exists @ home, use it
	_, err = os.Stat(homeKubeConfigPath)
	if err == nil {
		return homeKubeConfigPath
	}

	return ""
}
