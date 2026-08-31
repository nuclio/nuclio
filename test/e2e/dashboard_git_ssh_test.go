//go:build test_integration && test_local

/*
Copyright 2023 The Nuclio Authors.

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

package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nuclio/nuclio/test/e2e/gitssh"
	"github.com/stretchr/testify/require"
)

func TestDashboardGitSSH(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker is required for the dashboard Git SSH smoke test")
	}

	dashboardImage := os.Getenv("NUCLIO_DASHBOARD_IMAGE")
	baseImage := os.Getenv("NUCLIO_TEST_BASE_IMAGE")
	if dashboardImage == "" || baseImage == "" {
		t.Skip("NUCLIO_DASHBOARD_IMAGE and NUCLIO_TEST_BASE_IMAGE are required")
	}

	fixture := gitssh.New(t, "host.docker.internal")
	hostPort := findFreePort(t)
	dashboardName := fmt.Sprintf("nuclio-dashboard-git-ssh-%d", time.Now().UnixNano())
	dashboardURL := fmt.Sprintf("http://127.0.0.1:%d", hostPort)

	removeDockerContainer(t, dashboardName)
	t.Cleanup(func() {
		removeDockerContainer(t, dashboardName)
	})

	output, err := exec.Command("docker", "run", "-d", "--rm",
		"--name", dashboardName,
		"--add-host", "host.docker.internal:host-gateway",
		"-p", fmt.Sprintf("%d:8070", hostPort),
		"-v", "/var/run/docker.sock:/var/run/docker.sock",
		"-e", "NUCLIO_DASHBOARD_NO_PULL_BASE_IMAGES=true",
		dashboardImage).CombinedOutput()
	require.NoError(t, err, "failed to start dashboard: %s", output)

	client := &http.Client{Timeout: 5 * time.Second}
	waitForDashboard(t, client, dashboardURL)

	functionName := fmt.Sprintf("dashboard-git-ssh-%d", time.Now().UnixNano())
	deleteFunction := func() {
		requestBody, marshalErr := json.Marshal(map[string]interface{}{
			"metadata": map[string]string{
				"name":      functionName,
				"namespace": "nuclio",
			},
		})
		if marshalErr != nil {
			return
		}
		request, requestErr := http.NewRequest(http.MethodDelete, dashboardURL+"/api/functions", strings.NewReader(string(requestBody)))
		if requestErr != nil {
			return
		}
		request.Header.Set("Content-Type", "application/json")
		response, requestErr := client.Do(request)
		if requestErr == nil {
			_ = response.Body.Close()
		}
	}
	t.Cleanup(deleteFunction)

	requestBody := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      functionName,
			"namespace": "nuclio",
			"labels": map[string]string{
				"nuclio.io/project-name": "default",
			},
		},
		"spec": map[string]interface{}{
			"runtime": "shell",
			"handler": "handler.sh:main",
			"triggers": map[string]interface{}{
				"default-http": map[string]string{"kind": "http"},
			},
			"build": map[string]interface{}{
				"path":          fixture.RepositoryURL,
				"codeEntryType": "git",
				"codeEntryAttributes": map[string]string{
					"branch":        "ssh-test",
					"sshPrivateKey": fixture.PrivateKey,
					"sshKnownHosts": fixture.KnownHosts,
				},
				"baseImage":        baseImage,
				"noBaseImagesPull": true,
			},
		},
	}
	requestJSON, err := json.Marshal(requestBody)
	require.NoError(t, err)

	response, err := client.Post(dashboardURL+"/api/functions", "application/json", strings.NewReader(string(requestJSON)))
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusAccepted, response.StatusCode, "dashboard rejected the function request")

	functionStatus := waitForFunction(t, client, dashboardURL, functionName)
	require.Equal(t, "ready", functionStatus.State, "dashboard function build failed: %s", functionStatus.Message)

	invokeURL := ""
	if len(functionStatus.ExternalInvocationURLs) > 0 {
		invokeURL = functionStatus.ExternalInvocationURLs[0]
	} else if len(functionStatus.InternalInvocationURLs) > 0 {
		invokeURL = functionStatus.InternalInvocationURLs[0]
	}
	require.NotEmpty(t, invokeURL)

	request, err := http.NewRequest(http.MethodPost, dashboardURL+"/api/function_invocations", strings.NewReader("hello-from-dashboard"))
	require.NoError(t, err)
	request.Header.Set("x-nuclio-function-name", functionName)
	request.Header.Set("x-nuclio-function-namespace", "nuclio")
	request.Header.Set("x-nuclio-invoke-url", invokeURL)
	request.Header.Set("x-nuclio-invoke-timeout", "30s")
	response, err = client.Do(request)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, response.Body)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
}

type functionStatus struct {
	State                  string   `json:"state"`
	Message                string   `json:"message"`
	ExternalInvocationURLs []string `json:"externalInvocationUrls"`
	InternalInvocationURLs []string `json:"internalInvocationUrls"`
}

type functionResponse struct {
	Status functionStatus `json:"status"`
}

func waitForDashboard(t *testing.T, client *http.Client, dashboardURL string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(dashboardURL + "/api/functions")
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	t.Fatal("timed out waiting for dashboard API")
}

func waitForFunction(t *testing.T, client *http.Client, dashboardURL, functionName string) functionStatus {
	t.Helper()
	deadline := time.Now().Add(5 * time.Minute)
	for time.Now().Before(deadline) {
		request, err := http.NewRequest(http.MethodGet, dashboardURL+"/api/functions/"+functionName, nil)
		if err == nil {
			request.Header.Set("x-nuclio-function-namespace", "nuclio")
			response, requestErr := client.Do(request)
			if requestErr == nil {
				body, readErr := io.ReadAll(response.Body)
				_ = response.Body.Close()
				if readErr == nil && response.StatusCode == http.StatusOK {
					var function functionResponse
					if json.Unmarshal(body, &function) == nil {
						if function.Status.State == "ready" || function.Status.State == "error" {
							return function.Status
						}
					}
				}
			}
		}
		time.Sleep(2 * time.Second)
	}

	t.Fatal("timed out waiting for function build")
	return functionStatus{}
}

func findFreePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func removeDockerContainer(t *testing.T, name string) {
	t.Helper()
	command := exec.Command("docker", "rm", "-f", name)
	_ = command.Run()
}
