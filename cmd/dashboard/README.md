# Dashboard

## HTTP API

### Listing all functions
#### Request 
* URL: `GET /functions`
* Headers: 
  * `x-nuclio-function-namespace`: Namespace (required)
  * `x-nuclio-project-name`: Filter by project name (optional)

#### Response
* Status code: 200
* Body:
```json
{
    "echo": {
        "metadata": {
            "name": "echo",
            "namespace": "nuclio",
            "labels": {
                "nuclio.io/project-name": "my-project-1"
            }
        },
        "spec": {
            "handler": "Handler",
            "runtime": "golang",
            "resources": {},
            "image": "localhost:5000/nuclio/processor-echo:ba5v992vlcq000a2b640",
            "httpPort": 30400,
            "replicas": 1,
            "version": -1,
            "alias": "latest",
            "build": {
                "path": "http://127.0.0.1:8070/sources/echo.go",
                "registry": "localhost:5000",
                "noBaseImagesPull": true
            },
            "runRegistry": "localhost:5000"
        },
        "status": {
            "state": "ready"
        }
    },
    "hello-world": {
        "metadata": {
            "name": "hello-world",
            "namespace": "nuclio"
        },
        "spec": {
            "runtime": "golang",
            "resources": {},
            "image": "localhost:5000/",
            "alias": "latest",
            "build": {
                "path": "https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go",
                "registry": "192.168.64.7:5000",
                "noBaseImagesPull": true
            },
            "runRegistry": "localhost:5000"
        },
        "status": {
            "state": "building"
        }
    }
}
```

### Getting a function by name
#### Request 
* URL: `GET /functions/<function name>`
* Headers: 
  * `x-nuclio-function-namespace`: Namespace (required)

#### Response
* Status code: 200
* Body:
```json
{
    "metadata": {
        "name": "echo",
        "namespace": "nuclio",
        "labels": {
            "nuclio.io/project-name": "my-project-1"
        }
    },
    "spec": {
        "handler": "Handler",
        "runtime": "golang",
        "resources": {},
        "image": "localhost:5000/nuclio/processor-echo:ba5v992vlcq000a2b640",
        "httpPort": 30400,
        "replicas": 1,
        "version": -1,
        "alias": "latest",
        "build": {
            "path": "http://127.0.0.1:8070/sources/echo.go",
            "registry": "localhost:5000",
            "noBaseImagesPull": true
        },
        "runRegistry": "localhost:5000"
    },
    "status": {
        "state": "ready"
    }
}
```

### Creating a function
To create a function, provide the following request and then periodically GET the function until `status.state` is set to `ready` or `error`. It is guaranteed that by the time the response is returned, getting the function will yield a body and not `404`.

#### Request 
* URL: `POST /functions`
* Headers: 
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
 {
    "metadata": {
        "name": "hello-world",
        "namespace": "nuclio"
    },
    "spec": {
        "runtime": "golang",
        "build": {
            "path": "https://raw.githubusercontent.com/nuclio/nuclio/master/hack/examples/golang/helloworld/helloworld.go",
            "registry": "192.168.64.7:5000",
            "noBaseImagesPull": true
        },
        "runRegistry": "localhost:5000"
    }
}
```
#### Response
* Status code: 202


### Updating a function
Updating a function is similar to creating a function. The only differences are:
* The method is `PUT` rather than `POST`
* You must provide certain fields (e.g. `spec.image`) which should be taken from a `GET` - update does not currently support augmentation. Whatever you pass in the body is the new function spec.

#### Request 
* URL: `PUT /functions`
* Headers: 
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
{
    "metadata": {
        "name": "hello-world",
        "namespace": "nuclio"
    },
    "spec": {
        "handler": "Handler",
        "runtime": "golang",
        "resources": {},
        "image": "localhost:5000/nuclio/processor-hello-world:latest",
        "version": -1,
        "alias": "latest",
        "replicas": 1,
        "build": {
            "path": "/var/folders/w7/45z_c5lx2n3571nf6hkdvqqw0000gn/T/nuclio-build-238269306/download/helloworld.go",
            "registry": "192.168.64.7:5000",
            "noBaseImagesPull": true
        },
        "runRegistry": "localhost:5000"
    },
    "status": {
        "state": "ready"
    }
}
```
#### Response
* Status code: 202

### Invoking a function
#### Request 
* URL: `POST /function_invocations`
* Headers: 
  * `x-nuclio-function-name`: Function name (required)
  * `x-nuclio-function-namespace`: Namespace (required)
  * `x-nuclio-function-path`: The path to invoke the function with (can be empty to invoke with `/`)
  * `x-nuclio-function-invoke-via`: One of `external-ip`, `loadbalancer` and `domain-name`
  * Any other header is passed transparently to the function
* Body: Raw body passed as is to the function

#### Response
* Status code: As returned by the function
* Headers: As returned by the function
* Body: As returned by the function


### Deleting a function
#### Request 
* URL: `DELETE /functions`
* Headers: 
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
 {
    "metadata": {
        "name": "hello-world",
        "namespace": "nuclio"
    }
}
```
#### Response
* Status code: 204

### Listing all projects
#### Request 
* URL: `GET /projects`
* Headers: 
  * `x-nuclio-project-namespace`: Namespace (required)

#### Response
* Status code: 200
* Body:
```json
{
    "my-project-1": {
        "metadata": {
            "name": "my-project-1",
            "namespace": "nuclio"
        },
        "spec": {
            "displayName": "My project #1",
            "description": "Some description"
        }
    },
    "my-project-2": {
        "metadata": {
            "name": "my-project-2",
            "namespace": "nuclio"
        },
        "spec": {
            "displayName": "My project #2",
            "description": "Some description"
        }
    }
}
```

### Getting a project by name
#### Request 
* URL: `GET /projects/<project name>`
* Headers: 
  * `x-nuclio-project-namespace`: Namespace (required)

#### Response
* Status code: 200
* Body:
```json
{
    "metadata": {
        "name": "my-project-1",
        "namespace": "nuclio"
    },
    "spec": {
        "displayName": "My project #1",
        "description": "Some description"
    }
}
```

### Creating a project
Creating a project is synchronous. By the time the response returns, the project has been created.

#### Request 
* URL: `POST /projects`
* Headers: 
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
 {
    "metadata": {
        "name": "my-project-1",
        "namespace": "nuclio"
    },
    "spec": {
        "displayName": "My project #1",
        "description": "Some description"
    }
}
```
#### Response
* Status code: 204


### Updating a project

#### Request 
* URL: `PUT /projects`
* Headers: 
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
{
    "metadata": {
        "name": "my-project-1",
        "namespace": "nuclio"
    },
    "spec": {
        "displayName": "My project #1",
        "description": "Some description"
    }
}
```
#### Response
* Status code: 200

### Deleting a project
Only projects with no functions can be deleted. Attempting to delete a project with functions will result in an error being returned.

#### Request 
* URL: `DELETE /projects`
* Headers: 
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
 {
    "metadata": {
        "name": "my-project-1",
        "namespace": "nuclio"
    }
}
```
#### Response
* Status code: 204
