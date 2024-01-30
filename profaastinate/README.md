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

## Resources

### Other
- [PlantUML](https://plantuml.com/starting)
- [Docker](https://www.docker.com/)
- [Minikube](https://github.com/minekube)