# Nexus

A tool that allows to manage asynchronous requests in nuclio. 
The purpose of it is to save resources by running multiple functions of the same container at the same time, avoiding coldstarts.

## Overview

### Scheduler Domains 
Contains the logic for the different schedulers to function.

| Domain   | Path                            | Domain Description                                                |
|----------|---------------------------------|-------------------------------------------------------------------|
| Bulk     | [/pkg-nexus/bulk](bulk)         | Purpose: BulkScheduler tries to reduce the amount of colds-starts |
| Deadline | [/pkg-nexus/deadline](deadline) | Purpose: ensure that tasks are executed before their deadline     |
| Idle     | [/pkg-nexus/idle](idle)         | Purpose: use the resources as much as possible                    |


### General Domains

| Domain         | Path                                        | Domain Description                                                                                              |
|----------------|---------------------------------------------|-----------------------------------------------------------------------------------------------------------------|
| Common         | [/pkg-nexus/common](common)                 | Provides utils, envs, models which are composited or injected into other domains                                |
| Elastic Deploy | [/pkg-nexus/elastic-deploy](elastic-deploy) | Allows to pause and resume function containers dynamically to reduce system resources                           |
| Load Balancer  | [/pkg-nexus/load-balancer](load-balancer)   | Optimally distributes system resources to prevent bottlenecks and ensuring a responsive, fault-tolerant system. |

## Resources

### Nuclio inside Docker
- [Go Docker Client](https://github.com/fsouza/go-dockerclient)
- [Docker API Documentation](https://docs.docker.com/engine/api/v1.41/)

### Testing
- [Testify Suite](https://pkg.go.dev/github.com/stretchr/testify/suite)
- [Guide for Testing in Golang](https://medium.com/nerd-for-tech/writing-unit-tests-in-golang-part3-test-suite-6cca903be9ab)

### Other
- [Chi Router](https://github.com/go-chi/chi)
- [Go Utils](https://github.com/shirou/gopsutil)