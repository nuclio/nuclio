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

package dockerclient

import (
	"encoding/json"
)

// LogInOptions are options for logging in
type LogInOptions struct {
	Username string
	Password string
	URL      string
}

// BuildOptions are options for building a docker image
type BuildOptions struct {
	ImageName      string
	ContextDir     string
	DockerfilePath string
	NoCache        bool
}

// RunOptions are options for running a docker image
type RunOptions struct {
	Ports         map[int]int
	ContainerName string
	NetworkType   string
	Env           map[string]string
	Labels        map[string]string
	Volumes       map[string]string
}

// GetContainerOptions are options for container search
type GetContainerOptions struct {
	Labels map[string]string
}

// ContainerJSONBase contains response of Engine API:
// GET "/containers/{name:.*}/json"
type Container struct {
	ID              string `json:"Id"`
	Created         string
	Path            string
	Args            []string
	State           *ContainerState
	Image           string
	ResolvConfPath  string
	HostnamePath    string
	HostsPath       string
	LogPath         string
	Name            string
	RestartCount    int
	Driver          string
	Platform        string
	MountLabel      string
	ProcessLabel    string
	AppArmorProfile string
	ExecIDs         []string
	HostConfig      *HostConfig
	Mounts          []MountPoint
	Config          *Config
	NetworkSettings *NetworkSettings
}

// ContainerState stores container's running state
// it's part of ContainerJSONBase and will return by "inspect" command
type ContainerState struct {
	Status     string // String representation of the container state. Can be one of "created", "running", "paused", "restarting", "removing", "exited", or "dead"
	Running    bool
	Paused     bool
	Restarting bool
	OOMKilled  bool
	Dead       bool
	Pid        int
	ExitCode   int
	Error      string
	StartedAt  string
	FinishedAt string
}

// MountPoint represents a mount point configuration inside the container.
// This is used for reporting the mountpoints in use by a container.
type MountPoint struct {
	Source      string
	Destination string
	Mode        string
	RW          bool
}

// HostConfig the non-portable Config structure of a container.
// Here, "non-portable" means "dependent of the host we are running on".
// Portable information *should* appear in Config.
type HostConfig struct {
	// Applicable to all platforms
	Binds           []string      // List of volume bindings for this container
	ContainerIDFile string        // File (path) where the containerId is written
	LogConfig       LogConfig     // Configuration of the logs for this container
	NetworkMode     string        // Network mode to use for the container
	PortBindings    PortMap       // Port mapping between the exposed port (container) and the host
	RestartPolicy   RestartPolicy // Restart policy to be used for the container
	AutoRemove      bool          // Automatically remove container when it exits
	VolumeDriver    string        // Name of the volume driver used to mount volumes
	VolumesFrom     []string      // List of volumes to take from other container

	// Applicable to UNIX platforms
	CapAdd          StrSlice          // List of kernel capabilities to add to the container
	CapDrop         StrSlice          // List of kernel capabilities to remove from the container
	DNS             []string          `json:"Dns"`        // List of DNS server to lookup
	DNSOptions      []string          `json:"DnsOptions"` // List of DNSOption to look for
	DNSSearch       []string          `json:"DnsSearch"`  // List of DNSSearch to look for
	ExtraHosts      []string          // List of extra hosts
	GroupAdd        []string          // List of additional groups that the container process will run as
	IpcMode         string            // IPC namespace to use for the container
	Cgroup          string            // Cgroup to use for the container
	Links           []string          // List of links (in the name:alias form)
	OomScoreAdj     int               // Container preference for OOM-killing
	PidMode         string            // PID namespace to use for the container
	Privileged      bool              // Is the container in privileged mode
	PublishAllPorts bool              // Should docker publish all exposed port for the container
	ReadonlyRootfs  bool              // Is the container root filesystem in read-only
	SecurityOpt     []string          // List of string values to customize labels for MLS systems, such as SELinux.
	StorageOpt      map[string]string `json:",omitempty"` // Storage driver options per container.
	Tmpfs           map[string]string `json:",omitempty"` // List of tmpfs (mounts) used for the container
	UTSMode         string            // UTS namespace to use for the container
	UsernsMode      string            // The user namespace to use for the container
	ShmSize         int64             // Total shm memory usage
	Sysctls         map[string]string `json:",omitempty"` // List of Namespaced sysctls used for the container
	Runtime         string            `json:",omitempty"` // Runtime to use with this container

	// Applicable to Windows
	ConsoleSize [2]uint // Initial console size (height,width)
	Isolation   string  // Isolation technology of the container (e.g. default, hyperv)

	// Run a custom init inside the container, if null, use the daemon's configured settings
	Init *bool `json:",omitempty"`
}

// LogConfig represents the logging configuration of the container.
type LogConfig struct {
	Type   string
	Config map[string]string
}

// RestartPolicy represents the restart policies of the container.
type RestartPolicy struct {
	Name              string
	MaximumRetryCount int
}

// PortBinding represents a binding between a Host IP address and a Host Port
type PortBinding struct {
	// HostIP is the host IP Address
	HostIP string `json:"HostIp"`
	// HostPort is the host port number
	HostPort string
}

// PortMap is a collection of PortBinding indexed by Port
type PortMap map[Port][]PortBinding

// PortSet is a collection of structs indexed by Port
type PortSet map[Port]struct{}

// Port is a string containing port number and protocol in the format "80/tcp"
type Port string

// Config contains the configuration data about a container.
// It should hold only portable information about the container.
// Here, "portable" means "independent from the host we are running on".
// Non-portable information *should* appear in HostConfig.
// All fields added to this struct must be marked `omitempty` to keep getting
// predictable hashes from the old `v1Compatibility` configuration.
type Config struct {
	Hostname     string              // Hostname
	Domainname   string              // Domainname
	User         string              // User that will run the command(s) inside the container, also support user:group
	AttachStdin  bool                // Attach the standard input, makes possible user interaction
	AttachStdout bool                // Attach the standard output
	AttachStderr bool                // Attach the standard error
	Tty          bool                // Attach standard streams to a tty, including stdin if it is not closed.
	OpenStdin    bool                // Open stdin
	StdinOnce    bool                // If true, close stdin after the 1 attached client disconnects.
	Env          []string            // List of environment variable to set in the container
	Cmd          StrSlice            // Command to run when starting the container
	Image        string              // Name of the image as it was passed by the operator (e.g. could be symbolic)
	Volumes      map[string]struct{} // List of volumes (mounts) used for the container
	WorkingDir   string              // Current directory (PWD) in the command will be launched
	Entrypoint   StrSlice            // Entrypoint to run when starting the container
	OnBuild      []string            // ONBUILD metadata that were defined on the image Dockerfile
	Labels       map[string]string   // List of labels set to this container
}

// StrSlice represents a string or an array of strings.
// We need to override the json decoder to accept both options.
type StrSlice []string

// UnmarshalJSON decodes the byte slice whether it's a string or an array of
// strings. This method is needed to implement json.Unmarshaler.
func (e *StrSlice) UnmarshalJSON(b []byte) error {
	if len(b) == 0 {
		// With no input, we preserve the existing value by returning nil and
		// leaving the target alone. This allows defining default values for
		// the type.
		return nil
	}

	p := make([]string, 0, 1)
	if err := json.Unmarshal(b, &p); err != nil {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		p = append(p, s)
	}

	*e = p
	return nil
}

// NetworkSettings exposes the network settings in the api
type NetworkSettings struct {
	NetworkSettingsBase
	DefaultNetworkSettings
	Networks map[string]*EndpointSettings
}

// NetworkSettingsBase holds basic information about networks
type NetworkSettingsBase struct {
	Bridge                 string  // Bridge is the Bridge name the network uses(e.g. `docker0`)
	SandboxID              string  // SandboxID uniquely represents a container's network stack
	HairpinMode            bool    // HairpinMode specifies if hairpin NAT should be enabled on the virtual interface
	LinkLocalIPv6Address   string  // LinkLocalIPv6Address is an IPv6 unicast address using the link-local prefix
	LinkLocalIPv6PrefixLen int     // LinkLocalIPv6PrefixLen is the prefix length of an IPv6 unicast address
	Ports                  PortMap // Ports is a collection of PortBinding indexed by Port
	SandboxKey             string  // SandboxKey identifies the sandbox
	SecondaryIPAddresses   []Address
	SecondaryIPv6Addresses []Address
}

// DefaultNetworkSettings holds network information
// during the 2 release deprecation period.
// It will be removed in Docker 1.11.
type DefaultNetworkSettings struct {
	EndpointID          string // EndpointID uniquely represents a service endpoint in a Sandbox
	Gateway             string // Gateway holds the gateway address for the network
	GlobalIPv6Address   string // GlobalIPv6Address holds network's global IPv6 address
	GlobalIPv6PrefixLen int    // GlobalIPv6PrefixLen represents mask length of network's global IPv6 address
	IPAddress           string // IPAddress holds the IPv4 address for the network
	IPPrefixLen         int    // IPPrefixLen represents mask length of network's IPv4 address
	IPv6Gateway         string // IPv6Gateway holds gateway address specific for IPv6
	MacAddress          string // MacAddress holds the MAC address for the network
}

// EndpointSettings stores the network endpoint details
type EndpointSettings struct {
	// Configurations
	Links   []string
	Aliases []string
	// Operational data
	NetworkID           string
	EndpointID          string
	Gateway             string
	IPAddress           string
	IPPrefixLen         int
	IPv6Gateway         string
	GlobalIPv6Address   string
	GlobalIPv6PrefixLen int
	MacAddress          string
	DriverOpts          map[string]string
}

// Address represents an IP address
type Address struct {
	Addr      string
	PrefixLen int
}
