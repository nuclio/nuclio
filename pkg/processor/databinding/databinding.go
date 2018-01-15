package databinding

import "github.com/nuclio/nuclio-sdk"

type DataBinding interface {

	// Start will start the data binding, connecting to the remote resource
	Start() error

	// Stop will stop the data binding, cleaning up resources and tearing down connections
	Stop() error

	// GetContextObject will return the object that is injected into the context
	GetContextObject() (interface{}, error)
}

type AbstractDataBinding struct {
	Logger nuclio.Logger
}

// Start will start the data binding, connecting to the remote resource
func (adb *AbstractDataBinding) Start() error {
	return nil
}

// Stop will stop the data binding, cleaning up resources and tearing down connections
func (adb *AbstractDataBinding) Stop() error {
	return nil
}

// GetContextObject will return the object that is injected into the context
func (adb *AbstractDataBinding) GetContextObject() (interface{}, error) {
	return nil, nil
}
