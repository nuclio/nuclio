package docker

import (
	"fmt"
	docker "github.com/fsouza/go-dockerclient"
)

type DockerDeployer struct {
	*docker.Client

	baseContainerName string
}

func NewDockerDeployer(baseContainerName string) *DockerDeployer {
	return &DockerDeployer{
		baseContainerName: baseContainerName,
	}
}

func (ds *DockerDeployer) Initialize() {
	ds.Client, _ = docker.NewClientFromEnv()

	imgs, err := ds.ListContainers(docker.ListContainersOptions{All: true})
	if err != nil {
		panic(err)
	}
	for _, container := range imgs {
		fmt.Println("ID: ", container.ID)
		fmt.Println("RepoTags: ", container.Names)
		fmt.Println("Created: ", container.Created)
		fmt.Println("Size: ", container.Image)
	}
}

func (ds *DockerDeployer) Start(functionName string) error {
	container := ds.getFunctionContainer(functionName, false)
	if ds.IsRunning(functionName) {
		fmt.Printf("Container %s has been running already\n", ds.getContainerName(functionName))
		return nil
	}

	fmt.Println("Container state: ", container.State)
	if container.State == "paused" {
		fmt.Println("Try to start container", container.ID)
		err := ds.StartContainer(container.ID, nil)
		if err != nil {
			return err
		}
		fmt.Printf("Container %s started\n", ds.getContainerName(functionName))
		return nil
	}

	fmt.Println("Container state: ", container.State, "does not match any of the expected states")
	return nil
}

func (ds *DockerDeployer) Pause(functionName string) error {
	container := ds.getFunctionContainer(functionName, true)
	if container.State == "paused" {
		fmt.Printf("Container %s has been paused already\n", ds.getContainerName(functionName))
		return nil
	}

	container = ds.getFunctionContainer(functionName, false)
	if container.State != "running" {
		err := ds.PauseContainer(container.ID)
		if err != nil {
			return err
		}
		fmt.Println("Container %s paused", ds.getContainerName(functionName))
		return nil
	}

	fmt.Println("Container state: ", container.State, "does not match any of the expected states")
	return nil
}

func (ds *DockerDeployer) IsRunning(functionName string) bool {
	container := ds.getFunctionContainer(functionName, false)
	return container.State == "running"
}

func (ds *DockerDeployer) getFunctionContainer(functionName string, stopped bool) *docker.APIContainers {
	options := &docker.ListContainersOptions{
		Filters: map[string][]string{"name": {ds.getContainerName(functionName)}},
		Limit:   1,
	}

	container, err := ds.ListContainers(*options)
	if err != nil {
		fmt.Println("Error getting container: ", err)
		return nil
	}

	if len(container) == 0 {
		fmt.Println("No container existing: ", err)
		return nil
	}
	return &container[0]
}

func (ds *DockerDeployer) getContainerName(functionName string) string {
	return ds.baseContainerName + functionName
}
