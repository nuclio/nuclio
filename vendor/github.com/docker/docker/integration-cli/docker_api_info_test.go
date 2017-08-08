package main

import (
	"net/http"

	"github.com/docker/docker/pkg/integration/checker"
	"github.com/go-check/check"
)

func (s *DockerSuite) TestInfoAPI(c *check.C) {
	endpoint := "/info"

	status, body, err := sockRequest("GET", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	// always shown fields
	stringsToCheck := []string{
		"ID",
		"Containers",
		"ContainersRunning",
		"ContainersPaused",
		"ContainersStopped",
		"Images",
		"LoggingDriver",
		"OperatingSystem",
		"NCPU",
		"OSType",
		"Architecture",
		"MemTotal",
		"KernelVersion",
		"Driver",
		"ServerVersion",
		"SecurityOptions"}

	out := string(body)
	for _, linePrefix := range stringsToCheck {
		c.Assert(out, checker.Contains, linePrefix)
	}
}

func (s *DockerSuite) TestInfoAPIVersioned(c *check.C) {
	testRequires(c, DaemonIsLinux) // Windows only supports 1.25 or later
	endpoint := "/v1.20/info"

	status, body, err := sockRequest("GET", endpoint, nil)
	c.Assert(status, checker.Equals, http.StatusOK)
	c.Assert(err, checker.IsNil)

	out := string(body)
	c.Assert(out, checker.Contains, "ExecutionDriver")
	c.Assert(out, checker.Contains, "not supported")
}
