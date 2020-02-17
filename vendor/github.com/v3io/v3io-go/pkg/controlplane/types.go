/*
Copyright 2018 The v3io Authors.

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

package v3ioc

import (
	"context"
	"time"
)

// ControlplaneSession allows operations over a controlplane session
type Session interface {
	// CreateUserSync creates a user (blocking)
	CreateUserSync(*CreateUserInput) (*CreateUserOutput, error)

	// DeleteUserSync deletes a user (blocking)
	DeleteUserSync(*DeleteUserInput) error

	// CreateContainerSync creates a container (blocking)
	CreateContainerSync(*CreateContainerInput) (*CreateContainerOutput, error)

	// DeleteUserSync deletes a user (blocking)
	DeleteContainerSync(*DeleteContainerInput) error

	// UpdateClusterInfoSync updates a cluster info record (blocking)
	UpdateClusterInfoSync(*UpdateClusterInfoInput) (*UpdateClusterInfoOutput, error)

	// CreateEventSync emits new event (blocking)
	CreateEventSync(*CreateEventInput) error

	// CreateAccessKeySync creates an access key (blocking)
	CreateAccessKeySync(*CreateAccessKeyInput) (*CreateAccessKeyOutput, error)

	// DeleteAccessKeySync deletes an access key (blocking)
	DeleteAccessKeySync(*DeleteAccessKeyInput) error
}

type ControlPlaneInput struct {
	ID        string
	IDNumeric int
	Ctx       context.Context
	Timeout   time.Duration
}

type ControlPlaneOutput struct {
	ID        string
	IDNumeric int
	Ctx       context.Context
}

// NewSessionInput specifies how to create a session
type NewSessionInput struct {
	ControlPlaneInput
	Endpoints []string
	AccessKey string
	SessionAttributes
}

// CreateUserInput specifies how to create a user
type CreateUserInput struct {
	ControlPlaneInput
	UserAttributes
}

// CreateUserOutput holds the response from creating a user
type CreateUserOutput struct {
	ControlPlaneOutput
	UserAttributes
}

// DeleteUserInput specifies how to delete a user
type DeleteUserInput struct {
	ControlPlaneInput
}

// CreateContainerInput specifies how to create a container
type CreateContainerInput struct {
	ControlPlaneInput
	ContainerAttributes
}

// CreateContainerOutput holds the response from creating a container
type CreateContainerOutput struct {
	ControlPlaneOutput
	ContainerAttributes
}

// DeleteContainerInput specifies how to delete a container
type DeleteContainerInput struct {
	ControlPlaneInput
}

// UpdateClusterInfoInput specifies how to update a cluster info record
type UpdateClusterInfoInput struct {
	ControlPlaneInput
	ClusterInfoAttributes
}

// UpdateClusterInfoOutput holds the response from updating a cluster info
type UpdateClusterInfoOutput struct {
	ControlPlaneOutput
	ClusterInfoAttributes
}

// CreateEventInput specifies how to create an event
type CreateEventInput struct {
	ControlPlaneInput
	EventAttributes
}

// CreateAccessKeyInput specifies how to create an access key
type CreateAccessKeyInput struct {
	ControlPlaneInput
	AccessKeyAttributes
}

// CreateAccessKeyOutput holds the response from creating an access key
type CreateAccessKeyOutput struct {
	ControlPlaneOutput
	AccessKeyAttributes
}

// DeleteAccessKeyInput specifies how to delete an access key
type DeleteAccessKeyInput struct {
	ControlPlaneInput
}
