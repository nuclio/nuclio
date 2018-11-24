// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The rundockerbuildlet command loops forever and creates and cleans
// up Docker containers running reverse buildlets. It keeps a fixed
// number of them running at a time. See x/build/env/linux-arm64/packet/README
// for one example user.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	image    = flag.String("image", "", "docker image to run; required.")
	numInst  = flag.Int("n", 1, "number of containers to keep running at once")
	basename = flag.String("basename", "builder", "prefix before the builder number to use for the container names and host names")
	memory   = flag.String("memory", "3g", "memory limit flag for docker run")
	keyFile  = flag.String("key", "/etc/gobuild.key", "go build key file")
)

var (
	buildKey     []byte
	scalewayMeta = new(scalewayMetadata)
)

func main() {
	flag.Parse()

	key, err := ioutil.ReadFile(*keyFile)
	if err != nil {
		log.Fatalf("error reading build key from --key=%s: %v", *keyFile, err)
	}
	buildKey = bytes.TrimSpace(key)

	if *image == "" {
		log.Fatalf("docker --image is required")
	}

	if _, err := os.Stat("/usr/local/bin/oc-metadata"); err == nil {
		initScalewayMeta()
	}

	log.Printf("Started. Will keep %d copies of %s running.", *numInst, *image)
	for {
		if err := checkFix(); err != nil {
			log.Print(err)
		}
		time.Sleep(time.Second) // TODO: docker wait on the running containers?
	}
}

func checkFix() error {
	running := map[string]bool{}

	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.ID}} {{.Names}} {{.Status}}").Output()
	if err != nil {
		return fmt.Errorf("error running docker ps: %v", err)
	}
	// Out is like:
	// b1dc9ec2e646 packet14 Up 23 minutes
	// eeb458938447 packet11 Exited (0) About a minute ago
	// ...
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		f := strings.SplitN(line, " ", 3)
		if len(f) < 3 {
			continue
		}
		container, name, status := f[0], f[1], f[2]
		prefix := *basename
		if scalewayMeta != nil {
			// scaleway containers are named after their instance.
			prefix = scalewayMeta.Hostname
		}
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if strings.HasPrefix(status, "Exited") {
			removeContainer(container)
		}
		running[name] = strings.HasPrefix(status, "Up")
	}

	for num := 1; num <= *numInst; num++ {
		var name string
		if scalewayMeta != nil && scalewayMeta.Hostname != "" {
			// The -name passed to 'docker run' should match the
			// c1 instance hostname for debugability.
			// There should only be one running container per c1 instance.
			name = scalewayMeta.Hostname
		} else {
			name = fmt.Sprintf("%s%02d", *basename, num)
		}
		if running[name] {
			continue
		}

		// Just in case we have a container that exists but is not "running"
		// check if it exists and remove it before creating a new one.
		out, err = exec.Command("docker", "ps", "-a", "--filter", "name="+name, "--format", "{{.CreatedAt}}").Output()
		if err == nil && len(bytes.TrimSpace(out)) > 0 {
			// The format for the output is the create time and date:
			// 2017-07-24 17:07:39 +0000 UTC
			// To avoid a race with a container that is "Created" but not yet running
			// check how long ago the container was created.
			// If it's longer than minute, remove it.
			created, err := time.Parse("2006-01-02 15:04:05 -0700 MST", strings.TrimSpace(string(out)))
			if err != nil {
				log.Printf("converting output %q for container %s to time failed: %v", out, name, err)
				continue
			}
			dur := time.Since(created)
			if dur.Minutes() > 0 {
				removeContainer(name)
			}

			log.Printf("Container %s is already being created, duration %s", name, dur.String())
			continue
		}

		log.Printf("Creating %s ...", name)
		keyFile := fmt.Sprintf("/tmp/buildkey%02d/gobuildkey", num)
		if err := os.MkdirAll(filepath.Dir(keyFile), 0700); err != nil {
			return err
		}
		if err := ioutil.WriteFile(keyFile, buildKey, 0600); err != nil {
			return err
		}
		out, err := exec.Command("docker", "run",
			"-d",
			"--memory="+*memory,
			"--name="+name,
			"-v", filepath.Dir(keyFile)+":/buildkey/",
			"-e", "HOSTNAME="+name,
			"--tmpfs=/workdir:rw,exec",
			*image).CombinedOutput()
		if err != nil {
			log.Printf("Error creating %s: %v, %s", name, err, out)
			continue
		}
		log.Printf("Created %v", name)
	}
	return nil
}

type scalewayMetadata struct {
	Name     string   `json:"name"`
	Hostname string   `json:"hostname"`
	Tags     []string `json:"tags"`
}

func initScalewayMeta() {
	const metaURL = "http://169.254.42.42/conf?format=json"
	res, err := http.Get(metaURL)
	if err != nil {
		log.Fatalf("failed to get scaleway metadata: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("failed to get scaleway metadata from %s: %v", metaURL, res.Status)
	}
	if err := json.NewDecoder(res.Body).Decode(scalewayMeta); err != nil {
		log.Fatalf("invalid JSON from scaleway metadata URL %s: %v", metaURL, err)
	}
}

func removeContainer(container string) {
	if out, err := exec.Command("docker", "rm", "-f", container).CombinedOutput(); err != nil {
		log.Printf("error running docker rm -f %s: %v, %s", container, err, out)
		return
	}
	log.Printf("Removed container %s", container)
}
