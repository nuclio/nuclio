package platform

// Node represents a single node in the platform
type Node interface {

	// GetAddresses returns the list of addresses bound to the node
	GetAddresses() []Address
}
