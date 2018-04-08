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

package dockercreds

import (
	"encoding/json"
	"io/ioutil"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
)

type dockerCred struct {
	path                   string
	dockerCreds            *DockerCreds
	defaultRefreshInterval *time.Duration
	credentials            Credentials
}

func newDockerCred(dockerCreds *DockerCreds, path string,
	defaultRefreshInterval *time.Duration) (*dockerCred, error) {
	fallbackDuration := time.Hour * 12

	if defaultRefreshInterval == nil {
		defaultRefreshInterval = &fallbackDuration
	}

	newDockerCred := &dockerCred{
		path:                   path,
		dockerCreds:            dockerCreds,
		defaultRefreshInterval: defaultRefreshInterval,
	}

	if err := newDockerCred.initialize(); err != nil {
		return nil, err
	}

	return newDockerCred, nil
}

func (dc *dockerCred) initialize() error {
	var err error

	fileName := path.Base(dc.path)

	contents, err := ioutil.ReadFile(dc.path)
	if err != nil {
		return errors.Wrapf(err, "Failed to read docker key file @ %s", dc.path)
	}

	// try to read legacy format
	dc.credentials, err = dc.readLegacySecretFormat(fileName, contents)

	// failed, try to read docker registry format (new style, where credentials are under "auths"
	if err != nil {
		dc.credentials, err = dc.readKubernetesDockerRegistrySecretAuthsFormat(contents)
	}

	// failed, try to read docker registry format (old style, where credentials are under root
	if err != nil {
		dc.credentials, err = dc.readKubernetesDockerRegistrySecretNoAuthsFormat(contents)
	}

	// we're out of supported formats, bail
	if err != nil {
		return errors.Wrap(err, "Failed to read secret")
	}

	// if we didn't get a refresh interval in the cred file name, try the default
	if dc.credentials.RefreshInterval == nil {
		dc.credentials.RefreshInterval = dc.defaultRefreshInterval
	}

	// if user didn't specify "https://" in the url, add it. otherwise don't
	if !strings.HasPrefix(dc.credentials.URL, "https://") {
		dc.credentials.URL = "https://" + dc.credentials.URL
	}

	dc.dockerCreds.logger.InfoWith("Initializing docker credential",
		"path", dc.path,
		"username", dc.credentials.Username,
		"passwordLen", len(dc.credentials.Password),
		"url", dc.credentials.URL,
		"refreshInterval", dc.credentials.RefreshInterval)

	// try to login
	if err = dc.login(); err != nil {

		// warn about failed login, but still try to refresh
		dc.dockerCreds.logger.WarnWith("Failed to log in",
			"err", err,
			"path", dc.path)
	}

	if dc.credentials.RefreshInterval != nil {
		dc.refreshCredentials(*dc.credentials.RefreshInterval)
	}

	return nil
}

func (dc *dockerCred) readKubernetesDockerRegistrySecretAuthsFormat(contents []byte) (Credentials, error) {
	var parsedSecret Credentials

	// declare the marshalled auth
	unmarshalledSecret := struct {
		Auths map[string]struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"auths"`
	}{}

	if err := json.Unmarshal(contents, &unmarshalledSecret); err != nil {
		return parsedSecret, errors.Wrap(err, "Failed to unmarshal secret")
	}

	// if we have more than one key, this is unexpected, bail
	if len(unmarshalledSecret.Auths) != 1 {
		return parsedSecret, errors.New("Expected only one key under 'auths'")
	}

	// set secret (will iterate once)
	for url, secret := range unmarshalledSecret.Auths {
		parsedSecret.URL = url
		parsedSecret.Username = secret.Username
		parsedSecret.Password = secret.Password
	}

	return parsedSecret, nil
}

func (dc *dockerCred) readKubernetesDockerRegistrySecretNoAuthsFormat(contents []byte) (Credentials, error) {
	var parsedSecret Credentials

	// declare the marshalled auth
	unmarshalledSecret := map[string]struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}{}

	if err := json.Unmarshal(contents, &unmarshalledSecret); err != nil {
		return parsedSecret, errors.Wrap(err, "Failed to unmarshal secret")
	}

	// if we have more than one key, this is unexpected, bail
	if len(unmarshalledSecret) != 1 {
		return parsedSecret, errors.New("Expected only one key")
	}

	// set secret (will iterate once)
	for url, secret := range unmarshalledSecret {
		parsedSecret.URL = url
		parsedSecret.Username = secret.Username
		parsedSecret.Password = secret.Password
	}

	return parsedSecret, nil
}

func (dc *dockerCred) readLegacySecretFormat(fileName string, contents []byte) (Credentials, error) {
	var parsedSecret Credentials
	var err error
	var refreshIntervalString string

	// get the URL and username - check if this is the legacy secret format (file name is encoded as
	// username---registry---interval.json
	parsedSecret.Username, parsedSecret.URL, refreshIntervalString, err = extractMetaFromKeyPath(fileName)
	if err != nil {
		return parsedSecret, errors.Wrap(err, "Failed to read legacy format secret")
	}

	parsedSecret.RefreshInterval, err = parseRefreshInterval(refreshIntervalString)
	if err != nil {

		// if failed, we still want to try with the default refresh interval
		dc.dockerCreds.logger.WarnWith("Failed to read given refresh interval, trying default",
			"err", err,
			"path", dc.path,
			"refreshInterval", refreshIntervalString)
	}

	parsedSecret.Password = string(contents)

	return parsedSecret, nil
}

func extractMetaFromKeyPath(keyPath string) (string, string, string, error) {
	dockerKeyBase := path.Base(keyPath)
	dockerKeyExt := path.Ext(dockerKeyBase)

	// get just the file name
	dockerKeyFileWithoutExt := dockerKeyBase[:len(dockerKeyBase)-len(dockerKeyExt)]

	// expect user@url in the ext
	meta := strings.Split(dockerKeyFileWithoutExt, "---")
	if len(meta) < 2 || len(meta) > 3 {
		return "", "", "", errors.New("Expected file name to contain either user---url or user--url---refresh")
	}

	if len(meta[0]) == 0 {
		return "", "", "", errors.New("Username is empty")
	}

	if len(meta[1]) == 0 {
		return "", "", "", errors.New("URL is empty")
	}

	// return the user and URL
	if len(meta) == 2 {
		return meta[0], meta[1], "", nil
	}

	return meta[0], meta[1], meta[2], nil
}

func (dc *dockerCred) login() error {
	dc.dockerCreds.logger.DebugWith("Logging in",
		"url", dc.path,
		"user", dc.credentials.Username)

	// try to login
	return dc.dockerCreds.dockerClient.LogIn(&dockerclient.LogInOptions{
		Username: dc.credentials.Username,
		Password: dc.credentials.Password,
		URL:      dc.credentials.URL,
	})
}

func parseRefreshInterval(refreshInterval string) (*time.Duration, error) {
	if refreshInterval == "" {
		return nil, nil
	}

	refreshIntervalDuration, err := time.ParseDuration(refreshInterval)

	if err != nil {
		return nil, errors.Wrap(err, "Failed to parse refresh interval duration")
	}

	return &refreshIntervalDuration, nil
}

func (dc *dockerCred) refreshCredentials(refreshInterval time.Duration) {
	dc.dockerCreds.logger.InfoWith("Refreshing credentials periodically",
		"path", dc.path,
		"refreshInterval", refreshInterval)

	refreshTicker := time.NewTicker(refreshInterval)

	go func() {
		for range refreshTicker.C {
			dc.login() // nolint: errcheck
		}
	}()
}
