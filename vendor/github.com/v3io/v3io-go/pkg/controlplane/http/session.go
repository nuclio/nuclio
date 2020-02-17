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

package v3iochttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/v3io/v3io-go/pkg/controlplane"
	"github.com/v3io/v3io-go/pkg/errors"

	"github.com/nuclio/logger"
	"github.com/valyala/fasthttp"
)

type session struct {
	logger     logger.Logger
	httpClient *fasthttp.Client
	cookies    map[string]string
	endpoints  []string
}

func NewSession(parentLogger logger.Logger,
	newSessionInput *v3ioc.NewSessionInput) (v3ioc.Session, error) {

	newSession := session{
		logger:    parentLogger.GetChild("http"),
		cookies:   map[string]string{},
		endpoints: newSessionInput.Endpoints,
		httpClient: &fasthttp.Client{
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	if len(newSessionInput.AccessKey) > 0 {
		newSession.logger.DebugWithCtx(newSessionInput.Ctx, "Access key found. Will use it to create new session")

		// Generate cookie from access key
		cookieValue := fmt.Sprintf(`j:{"sid": "%s"}`, newSessionInput.AccessKey)
		newSession.cookies["session"] = fmt.Sprintf("session=%s;", url.PathEscape(cookieValue))

	} else {

		// Create new session using username and password
		var output v3ioc.ControlPlaneOutput
		responseSessionAttributes := v3ioc.SessionAttributes{}
		newSessionInput.Plane = "control"

		// try to create the resource
		err := newSession.createResource(newSessionInput.Ctx,
			"sessions",
			"session",
			&newSessionInput.ControlPlaneInput,
			&newSessionInput.SessionAttributes,
			&output,
			&responseSessionAttributes)

		if err != nil {
			return nil, err
		}

		newSession.logger.DebugWithCtx(newSessionInput.Ctx, "Session created", "ID", output.ID)
	}

	return &newSession, nil
}

// CreateUserSync creates a user (blocking)
func (s *session) CreateUserSync(createUserInput *v3ioc.CreateUserInput) (*v3ioc.CreateUserOutput, error) {

	// prepare session response resource
	createUserOutput := v3ioc.CreateUserOutput{}

	// try to create the resource
	err := s.createResource(createUserInput.Ctx,
		"users",
		"user",
		&createUserInput.ControlPlaneInput,
		&createUserInput.UserAttributes,
		&createUserOutput.ControlPlaneOutput,
		&createUserOutput.UserAttributes)

	if err != nil {
		return nil, err
	}

	return &createUserOutput, nil
}

// DeleteUserSync deletes a user (blocking)
func (s *session) DeleteUserSync(deleteUserInput *v3ioc.DeleteUserInput) error {

	// try to create the resource
	return s.deleteResource(deleteUserInput.Ctx,
		"users",
		"user",
		&deleteUserInput.ControlPlaneInput)
}

// CreateContainerSync creates a container (blocking)
func (s *session) CreateContainerSync(
	createContainerInput *v3ioc.CreateContainerInput) (*v3ioc.CreateContainerOutput, error) {

	// prepare session response resource
	createContainerOutput := v3ioc.CreateContainerOutput{}

	// try to create the resource
	err := s.createResource(createContainerInput.Ctx,
		"containers",
		"container",
		&createContainerInput.ControlPlaneInput,
		&createContainerInput.ContainerAttributes,
		&createContainerOutput.ControlPlaneOutput,
		&createContainerOutput.ContainerAttributes)

	if err != nil {
		return nil, err
	}

	return &createContainerOutput, nil
}

// DeleteUserSync deletes a container (blocking)
func (s *session) DeleteContainerSync(deleteContainerInput *v3ioc.DeleteContainerInput) error {

	// try to create the resource
	return s.deleteResource(deleteContainerInput.Ctx,
		"containers",
		"container",
		&deleteContainerInput.ControlPlaneInput)
}

// UpdateClusterInfoSync updates the cluster_info record of the cluster (blocking)
func (s *session) UpdateClusterInfoSync(
	updateClusterInfoInput *v3ioc.UpdateClusterInfoInput) (*v3ioc.UpdateClusterInfoOutput, error) {

	// prepare session response resource
	updateClusterInfoOutput := v3ioc.UpdateClusterInfoOutput{}

	// prepare detail update path
	detailPath := fmt.Sprintf("cluster_info/%s", updateClusterInfoInput.ID)

	// try to update the resource
	err := s.updateResource(updateClusterInfoInput.Ctx,
		detailPath,
		"cluster_info",
		&updateClusterInfoInput.ControlPlaneInput,
		&updateClusterInfoInput.ClusterInfoAttributes,
		&updateClusterInfoOutput.ControlPlaneOutput,
		&updateClusterInfoOutput.ClusterInfoAttributes)

	if err != nil {
		return nil, err
	}

	return &updateClusterInfoOutput, nil
}

// CreateEventSync emits an event (blocking)
func (s *session) CreateEventSync(createEventInput *v3ioc.CreateEventInput) error {

	// try to create the resource
	err := s.createResource(createEventInput.Ctx,
		"manual_events",
		"event",
		&createEventInput.ControlPlaneInput,
		&createEventInput.EventAttributes,
		nil,
		nil)

	return err
}

// CreateAccessKeySync creates an access key (blocking)
func (s *session) CreateAccessKeySync(createAccessKeyInput *v3ioc.CreateAccessKeyInput) (*v3ioc.CreateAccessKeyOutput, error) {

	// prepare session response resource
	createAccessKeyOutput := v3ioc.CreateAccessKeyOutput{}

	// try to create the resource
	err := s.createResource(createAccessKeyInput.Ctx,
		"access_keys",
		"access_key",
		&createAccessKeyInput.ControlPlaneInput,
		&createAccessKeyInput.AccessKeyAttributes,
		&createAccessKeyOutput.ControlPlaneOutput,
		&createAccessKeyOutput.AccessKeyAttributes)

	if err != nil {
		return nil, err
	}

	return &createAccessKeyOutput, nil
}

// DeleteAccessKeySync deletes an access key (blocking)
func (s *session) DeleteAccessKeySync(deleteAccessKeyInput *v3ioc.DeleteAccessKeyInput) error {

	// try to create the resource
	return s.deleteResource(deleteAccessKeyInput.Ctx,
		"access_keys",
		"access_key",
		&deleteAccessKeyInput.ControlPlaneInput)
}

func (s *session) createResource(ctx context.Context,
	path string,
	kind string,
	controlPlaneInput *v3ioc.ControlPlaneInput,
	requestAttributes interface{},
	controlPlaneOutput *v3ioc.ControlPlaneOutput,
	responseAttributes interface{}) error {

	return s.createOrUpdateResource(ctx,
		path,
		http.MethodPost,
		kind,
		controlPlaneInput,
		requestAttributes,
		controlPlaneOutput,
		responseAttributes)
}

func (s *session) updateResource(ctx context.Context,
	path string,
	kind string,
	controlPlaneInput *v3ioc.ControlPlaneInput,
	requestAttributes interface{},
	responseID *v3ioc.ControlPlaneOutput,
	responseAttributes interface{}) error {

	return s.createOrUpdateResource(ctx,
		path,
		http.MethodPut,
		kind,
		controlPlaneInput,
		requestAttributes,
		responseID,
		responseAttributes)
}

func (s *session) createOrUpdateResource(ctx context.Context,
	path string,
	httpMethod string,
	kind string,
	controlPlaneInput *v3ioc.ControlPlaneInput,
	requestAttributes interface{},
	controlPlaneOutput *v3ioc.ControlPlaneOutput,
	responseAttributes interface{}) error {

	// allocate request
	httpRequest := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(httpRequest)

	// choose whether to take numeric or string ID
	var resourceID interface{}
	if controlPlaneInput.IDNumeric != 0 {
		resourceID = controlPlaneInput.IDNumeric
	} else {
		resourceID = controlPlaneInput.ID
	}

	jsonAPIRequest := jsonapiResource{
		Data: jsonapiData{
			ID:         resourceID,
			Type:       kind,
			Attributes: requestAttributes,
		},
	}

	if err := json.NewEncoder(httpRequest.BodyWriter()).Encode(&jsonAPIRequest); err != nil {
		return err
	}

	responseInstance, err := s.sendRequest(ctx,
		&request{
			method:      httpMethod,
			path:        "api/" + path,
			httpRequest: httpRequest,
		}, controlPlaneInput.Timeout)

	if err != nil {
		return err
	}

	// if we got cookies, set them
	if len(responseInstance.cookies) > 0 {
		s.cookies = responseInstance.cookies
	}

	// unmarshal
	if responseInstance.body != nil && controlPlaneOutput != nil {
		responseBuffer := bytes.NewBuffer(responseInstance.body)

		jsonAPIResponse := jsonapiResource{
			Data: jsonapiData{
				Attributes: responseAttributes,
			},
		}

		if err := json.NewDecoder(responseBuffer).Decode(&jsonAPIResponse); err != nil {
			return err
		}

		switch typedResponseID := jsonAPIResponse.Data.ID.(type) {
		case string:
			controlPlaneOutput.ID = typedResponseID
		case float64:
			controlPlaneOutput.IDNumeric = int(typedResponseID)
		}
	}

	return nil
}

func (s *session) deleteResource(ctx context.Context,
	path string,
	kind string,
	controlPlaneInput *v3ioc.ControlPlaneInput) error {

	// allocate request
	httpRequest := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(httpRequest)

	var resourceID string

	switch typedResourceID := s.getResourceIDFromControlPlaneInput(controlPlaneInput).(type) {
	case string:
		resourceID = typedResourceID
	case int:
		resourceID = strconv.Itoa(typedResourceID)
	}

	_, err := s.sendRequest(ctx,
		&request{
			method:      http.MethodDelete,
			path:        fmt.Sprintf("api/%s/%s", path, resourceID),
			httpRequest: httpRequest,
		}, controlPlaneInput.Timeout)

	return err
}

func (s *session) sendRequest(ctx context.Context, request *request, timeout time.Duration) (*response, error) {
	request.httpRequest.Header.SetMethod(request.method)
	request.httpRequest.Header.SetContentType("application/json")
	request.httpRequest.SetRequestURI(fmt.Sprintf("%s/%s", s.endpoints[0], request.path))

	// set cookies
	for _, cookieValue := range s.cookies {
		request.httpRequest.Header.Set("Cookie", cookieValue)
	}

	// acquire response
	httpResponse := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(httpResponse)

	s.logger.DebugWithCtx(ctx,
		"Sending request",
		"method", request.method,
		"path", request.path,
		"headers", request.headers,
		"cookies", s.cookies,
		"body", string(request.httpRequest.Body()))

	// send the request
	var err error

	if timeout.Nanoseconds() == 0 {
		err = s.httpClient.Do(request.httpRequest, httpResponse)
	} else {
		err = s.httpClient.DoTimeout(request.httpRequest, httpResponse, timeout)
	}

	if err != nil {
		if err.Error() == "timeout" {
			return nil, v3ioerrors.ErrTimeout
		}

		return nil, err
	}

	// create a response
	responseInstance := response{
		statusCode: httpResponse.StatusCode(),
		body:       httpResponse.Body(),
	}

	// check if we got any cookies to set
	httpResponse.Header.VisitAllCookie(func(key []byte, value []byte) {
		responseInstance.cookies = map[string]string{}
		responseInstance.cookies[string(key)] = string(value)
	})

	// log the response
	s.logger.DebugWithCtx(ctx, "Got response",
		"statusCode", responseInstance.statusCode,
		"headers", responseInstance.headers,
		"cookies", responseInstance.cookies,
		"body", string(responseInstance.body))

	if !request.allowErrors {
		if responseInstance.statusCode >= 300 {
			return nil, v3ioerrors.NewErrorWithStatusCode(
				fmt.Errorf("Failed to execute HTTP request %s/%s.\nResponse code: %d",
					s.endpoints[0], request.path, responseInstance.statusCode),
				responseInstance.statusCode)
		}
	}

	return &responseInstance, nil
}

func (s *session) getResourceIDFromControlPlaneInput(controlPlaneInput *v3ioc.ControlPlaneInput) interface{} {

	// choose whether to take numeric or string ID
	if controlPlaneInput.IDNumeric != 0 {
		return controlPlaneInput.IDNumeric
	}

	return controlPlaneInput.ID
}
