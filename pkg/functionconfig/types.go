package functionconfig

import "k8s.io/api/core/v1"

// DataBinding holds configuration for a databinding
type DataBinding struct {
	Name    string            `json:"name,omitempty"`
	Class   string            `json:"class"`
	URL     string            `json:"url"`
	Path    string            `json:"path,omitempty"`
	Query   string            `json:"query,omitempty"`
	Secret  string            `json:"secret,omitempty"`
	Options map[string]string `json:"options,omitempty"`
}

// Trigger holds configuration for a trigger
type Trigger struct {
	Class         string                 `json:"class"`
	Kind          string                 `json:"kind"`
	Disabled      bool                   `json:"disabled,omitempty"`
	MaxWorkers    int                    `json:"maxWorkers,omitempty"`
	URL           string                 `json:"url,omitempty"`
	Paths         []string               `json:"paths,omitempty"`
	NumPartitions int                    `json:"numPartitions,omitempty"`
	User          string                 `json:"user,omitempty"`
	Secret        string                 `json:"secret,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
}

// GetIngresses returns the ingresses of a trigger, if applicable
func (t *Trigger) GetIngresses() (ingresses map[string]Ingress) {
	ingresses = map[string]Ingress{}

	if t.Kind == "http" {
		if encodedIngresses, found := t.Attributes["ingresses"]; found {

			// iterate over the encoded ingresses map and created ingress structures
			for encodedIngressName, encodedIngress := range encodedIngresses.(map[string]interface{}) {
				encodedIngressMap := encodedIngress.(map[string]interface{})

				ingress := Ingress{}

				// try to convert host
				if host, ok := encodedIngressMap["host"].(string); ok {
					ingress.Host = host
				}

				// try to convert paths - this can arrive as []string or []interface{}
				switch typedPaths := encodedIngressMap["paths"].(type) {
				case []string:
					ingress.Paths = typedPaths
				case []interface{}:
					for _, path := range typedPaths {
						ingress.Paths = append(ingress.Paths, path.(string))
					}
				}

				ingresses[encodedIngressName] = ingress
			}
		}
	}

	return
}

// GetIngressesFromTriggers returns all ingresses from a map of triggers
func GetIngressesFromTriggers(triggers map[string]Trigger) (ingresses map[string]Ingress) {
	ingresses = map[string]Ingress{}

	// helper to extend maps
	extendIngressMap := func(dest, source map[string]Ingress) map[string]Ingress {
		for name, ingress := range source {
			dest[name] = ingress
		}

		return dest
	}

	for _, trigger := range triggers {
		ingresses = extendIngressMap(ingresses, trigger.GetIngresses())
	}

	return ingresses
}

// Ingress holds configuration for an ingress - an entity that can route HTTP requests
// to the function
type Ingress struct {
	Host  string
	Paths []string
}

// Build holds all configuration parameters related to building a function
type Build struct {
	Path               string            `json:"path,omitempty"`
	FunctionConfigPath string            `json:"functionConfigPath,omitempty"`
	OutputType         string            `json:"outputType,omitempty"`
	NuclioSourceDir    string            `json:"nuclioSourceDir,omitempty"`
	NuclioSourceURL    string            `json:"nuclioSourceURL,omitempty"`
	Registry           string            `json:"registry,omitempty"`
	ImageName          string            `json:"imageName,omitempty"`
	ImageVersion       string            `json:"imageVersion,omitempty"`
	NoBaseImagesPull   bool              `json:"noBaseImagesPull,omitempty"`
	BaseImageName      string            `json:"baseImageName,omitempty"`
	Commands           []string          `json:"commands,omitempty"`
	ScriptPaths        []string          `json:"scriptPaths,omitempty"`
	AddedObjectPaths   map[string]string `json:"addedPaths,omitempty"`
}

// Spec holds all parameters related to a function's configuration
type Spec struct {
	Description  string                  `json:"description,omitempty"`
	Disabled     bool                    `json:"disable,omitempty"`
	Publish      bool                    `json:"publish,omitempty"`
	Handler      string                  `json:"handler,omitempty"`
	Runtime      string                  `json:"runtime,omitempty"`
	Env          []v1.EnvVar             `json:"env,omitempty"`
	Resources    v1.ResourceRequirements `json:"resources,omitempty"`
	ImageName    string                  `json:"image,omitempty"`
	HTTPPort     int                     `json:"httpPort,omitempty"`
	Replicas     int                     `json:"replicas,omitempty"`
	MinReplicas  int                     `json:"minReplicas,omitempty"`
	MaxReplicas  int                     `json:"maxReplicas,omitempty"`
	DataBindings map[string]DataBinding  `json:"dataBindings,omitempty"`
	Triggers     map[string]Trigger      `json:"triggers,omitempty"`
	Version      int                     `json:"version,omitempty"`
	Alias        string                  `json:"alias,omitempty"`
	Build        Build                   `json:"build,omitempty"`
	RunRegistry  string                  `json:"runRegistry,omitempty"`
}

// Meta identifies a function
type Meta struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Labels      map[string]string
	Annotations map[string]string
}

// Config holds the configuration of a function - meta and spec
type Config struct {
	Meta Meta
	Spec Spec `json:"spec,omitempty"`
}

// NewConfig creates a new configuration structure
func NewConfig() *Config {
	return &Config{
		Meta: Meta{
			Namespace: "default",
		},
		Spec: Spec{
			Replicas: 1,
			Build: Build{
				NuclioSourceURL: "https://github.com/nuclio/nuclio.git",
				OutputType:      "docker",
				ImageVersion:    "latest",
			},
		},
	}
}
