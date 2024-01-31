# Proofaastinate

This project is a proof of concept for a serverless framework which is able to dynamically scale functions based on their resource usage.
Proofaastinate is build on top of [Nuclio](https://nuclio.io) and uses the [Nexus](../pkg/nexus) package to manage the function requests.

## Overview

| Name        | Path                          | Description                                                                                        |
|-------------|-------------------------------|----------------------------------------------------------------------------------------------------|
| Deployment  | [/deployment](./deployment)   | Helps to build profaastinate inside docker or minikube                                             |
| deprecated  | [/deprecated](./deprecated)   | Contains deprecated legacy code.                                                                   |
| Docs        | [/docs](./docs)               | Contains documentation like uml diagrams to understand the code base better                        |
| Evaluation  | [/evaluation](./evaluation)   | Deprecated evulation REST Server which allowed to verify certain requests.                         |
| Prototyping | [/prototyping](./prototyping) | Acts as a playground to try out packages and ideas, before implementing features inside the Nexus  |
| Tests       | [/tests](./tests)             | Contains commands for checking the software for lint and test errors, which are also stored there. |

## Getting started

### Prerequisites
Make your self familiar with [Nuclio](https://nuclio.io), then with [Docker](https://www.docker.com/) incase you want to run profaastinate inside docker.
Otherwise you can use [Minikube](https://github.com/minekube) to run profaastinate inside a kubernetes cluster. 
Check out the [deployment](./deployment) folder for more information.

### Setup on Docker
```shell
sh ./deployment/docker/setup.sh
```

### Setup on Minikube
```shell
sh ./deployment/minikube/setup.sh
```
## Usage
1. Deploy Function <br>Deploy a nuclio function via cli or dashboard. For more information check out the [nuclio documentation](https://nuclio.io/docs/latest/tasks/deploying-functions/).
2. Send Request <br>Send a request to the function. For more information check out the [nuclio documentation](https://nuclio.io/docs/latest/tasks/invoking-functions/).
```shell 
curl --location 'http://localhost:8070/api/function_invocations' \
--header 'x-nuclio-function-name: test-hello-world' \
--header 'X-Profaastinate-Process-Deadline: 100' \
--header 'Content-Type: application/json' \
--data ''
```
3. Check the logs <br>Check the logs of the function to see the result of the request. For more information check out the [nuclio documentation](https://nuclio.io/docs/latest/tasks/monitoring-functions/).
```shell
nuclioctl get function test-hello-world -n nuclio
```


## Resources

### Other
- [PlantUML](https://plantuml.com/starting)
- [Docker](https://www.docker.com/)
- [Minikube](https://github.com/minekube)