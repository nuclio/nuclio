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
	"io/ioutil"
	"path"
	"strings"
	"time"

	"github.com/nuclio/nuclio/pkg/dockerclient"
	"github.com/nuclio/nuclio/pkg/errors"
)

type dockerCred struct {
	path string
	dockerCreds *DockerCreds
	defaultRefreshInterval *time.Duration
	username string
	password string
	url string
}

func newDockerCred(dockerCreds *DockerCreds, path string,
	defaultRefreshInterval *time.Duration) (*dockerCred, error) {
	newDockerCred := &dockerCred{
		path: path,
		dockerCreds: dockerCreds,
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

	password, err := ioutil.ReadFile(dc.path)
	if err != nil {
		return errors.Wrapf(err, "Failed to read docker key file @ %s", dc.path)
	}

	// get the URL and username
	username, url, refreshIntervalString, err := extractMetaFromKeyPath(fileName)
	if err != nil {
		return errors.Wrapf(err, "Failed to get user / url from path @ %s", dc.path)
	}

	dc.dockerCreds.logger.InfoWith("Initializing docker credential",
		"path", dc.path,
		"username", username,
		"passwordLen", len(password),
		"url", url,
		"refreshInterval", refreshIntervalString)

	// save in members
	dc.username = username
	dc.password = string(password)
	dc.url = url

	// try to login
	if err := dc.login(); err != nil {
		return err
	}

	refreshInterval, err := parseRefreshInterval(refreshIntervalString)
	if err != nil {

		// if failed, we still want to try with the default refresh interval
		dc.dockerCreds.logger.WarnWith("Failed to read given refresh interval, trying default",
			"err", err,
			"path", dc.path,
			"refreshInterval", refreshIntervalString)
	}

	// if we didn't get a refresh interval in the cred file name, try the default
	if refreshInterval == nil {
		refreshInterval = dc.defaultRefreshInterval
	}

	if refreshInterval != nil {
		dc.refreshCredentials(*refreshInterval)
	}

	return nil
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
	} else {
		return meta[0], meta[1], meta[2], nil
	}
}

func (dc *dockerCred) login() error {
	dc.dockerCreds.logger.DebugWith("Logging in",
		"url", dc.path,
		"user", dc.username)

	// try to login
	return dc.dockerCreds.dockerClient.LogIn(&dockerclient.LogInOptions{
		Username: dc.username,
		Password: string(dc.password),
		URL:      "https://" + dc.url,
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
		for {
			select {
			case <- refreshTicker.C:
				dc.login()
			}
		}
	}()
}
