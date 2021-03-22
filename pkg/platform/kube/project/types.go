package project

import(
	"github.com/nuclio/nuclio/pkg/platform"
	nuclioio "github.com/nuclio/nuclio/pkg/platform/kube/apis/nuclio.io/v1beta1"
)

type Client interface {

	// Initializes the projects client
	Initialize(platform.Platform) error

	// Creates a new project
	Create(*nuclioio.NuclioProject) (*nuclioio.NuclioProject, error)

	// Updates a project
	Update(*nuclioio.NuclioProject) (*nuclioio.NuclioProject, error)

	// Deletes a project
	Delete(*platform.DeleteProjectOptions) error

	// Gets projects (specify "getProjectsOptions.Meta.Name" to get a single function)
	Get(*platform.GetProjectsOptions) ([]nuclioio.NuclioProject, error)
}
