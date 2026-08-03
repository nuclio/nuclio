package registryhelpers

// AuthConfig carries the images and provider secret the auth/merge init containers need.
type AuthConfig struct {
	AWSCLIImage                string
	AzureCLIImage              string
	GCloudCLIImage             string
	RegistryProviderSecretName string
	PythonImage                string
	PythonImagePullPolicy      string
}