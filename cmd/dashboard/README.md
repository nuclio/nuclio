# Dashboard HTTP API
Table of contents:
- [Function](#function)
- [Project](#project)
- [Function event](#function-event)
- [Function template](#function-template)
- [Misc](#misc)

## Function

### Listing all functions
#### Request
* URL: `GET /api/functions`
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
* URL: `GET /api/functions/<function name>`
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
* URL: `POST /api/functions`
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
* URL: `PUT /api/functions`
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
* URL: `POST /api/function_invocations`
* Headers:
  * `x-nuclio-function-name`: Function name (required)
  * `x-nuclio-function-namespace`: Namespace (required)
  * `x-nuclio-path`: The path to invoke the function with (can be empty to invoke with `/`)
  * `x-nuclio-invoke-via`: One of `external-ip`, `loadbalancer` and `domain-name`
  * Any other header is passed transparently to the function
* Body: Raw body passed as is to the function

#### Response
* Status code: As returned by the function
* Headers:As returned by the function
* Body: As returned by the function


### Deleting a function
#### Request
* URL: `DELETE /api/functions`
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

## Project

### Listing all projects
#### Request
* URL: `GET /api/projects`
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
* URL: `GET /api/projects/<project name>`
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
Creating a project is synchronous. By the time the response returns, the project has been created. If name is omitted, a UUID is generated.

#### Request
* URL: `POST /api/projects`
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
* Status code: 201


### Updating a project

#### Request
* URL: `PUT /api/projects`
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
* URL: `DELETE /api/projects`
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

## Function event
A function event allows users to store reusable events with which to test their functions, rather than invoking a function with ad-hoc data. A function event is bound to a function through the `nuclio.io/function-name` label providing a 1:N relationship between a function and function events.


### Trigger attributes
The function event contains per-trigger attributes. The `triggerKind` specifies which kind of trigger the function event should invoke through.

#### HTTP trigger (`http`)
* `method` (string): Any standard HTTP method (e.g. `GET`, `POST`, etc)
* `path` (string, optional): The path to invoke, defaulting to `/` (e.g. `/my/invoke/path`)
* `headers` (map, optional): A map of headers

### Listing all function events
#### Request
* URL: `GET /api/function_events`
* Headers:
  * `x-nuclio-function-event-namespace`: Namespace (required)
  * `x-nuclio-function-name`: Function name (optional)

#### Response
* Status code: 200
* Body:
```json
{
    "bb016570-348d-4ea8-8092-799ae8f27845": {
        "metadata": {
            "name": "bb016570-348d-4ea8-8092-799ae8f27845",
            "namespace": "nuclio",
            "labels": {
                "nuclio.io/function-name": "my-function"
            }
        },
        "spec": {
            "displayName": "My function event",
            "body": "some body",
            "triggerKind": "http",
            "attributes": {
                "headers": {
                    "x-header-1": "a",
                    "x-header-2": 1
                },
                "method": "GET",
                "path": "/some/path"
            }
        }
    },
    "named-fe1": {
        "metadata": {
            "name": "named-fe1",
            "namespace": "nuclio",
            "labels": {
                "nuclio.io/function-name": "my-function"
            }
        },
        "spec": {
            "displayName": "My named function event",
            "body": "a body",
            "triggerKind": "http",
            "attributes": {
                "headers": {
                    "x-header-1": "a",
                    "x-header-2": 1
                },
                "method": "GET",
                "path": "/some/path"
            }
        }
    }
}
```

### Getting a function event by name
#### Request
* URL: `GET /api/function_events/<function event name>`
* Headers:
  * `x-nuclio-function-event-namespace`: Namespace (required)

#### Response
* Status code: 200
* Body:
```json
{
    "metadata": {
        "name": "named-fe1",
        "namespace": "nuclio",
        "labels": {
            "nuclio.io/function-name": "my-function"
        }
    },
    "spec": {
        "displayName": "My named function event",
        "body": "a body",
        "triggerKind": "http",
        "attributes": {
            "headers": {
                "x-header-1": "a",
                "x-header-2": 1
            },
            "method": "GET",
            "path": "/some/path"
        }
    }
}
```

### Creating a function event
Creating a function event is synchronous. By the time the response returns, the function event has been created. If `name` is omitted, a UUID is generated.

#### Request
* URL: `POST /api/function_events`
* Headers:
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
 {
    "metadata": {
        "namespace": "nuclio",
        "labels": {
            "nuclio.io/function-name": "my-function"
        }
    },
    "spec": {
        "displayName": "My function event",
        "body": "some body",
        "attributes": {
            "headers": {
                "x-header-1": "a",
                "x-header-2": 1
            },
            "method": "GET",
            "path": "/some/path"
        }
    }
}
```

#### Response
* Status code: 201
* Body:
```json
{
    "metadata": {
        "name": "db11d276-4c6a-4200-b096-d9b8fe2031cd",
        "namespace": "nuclio",
        "labels": {
            "nuclio.io/function-name": "my-function"
        }
    },
    "spec": {
        "displayName": "My function event",
        "body": "some body",
        "triggerKind": "http",
        "attributes": {
            "headers": {
                "x-header-1": "a",
                "x-header-2": 1
            },
            "method": "GET",
            "path": "/some/path"
        }
    }
}
```

### Updating a function event

#### Request
* URL: `PUT /api/function_events`
* Headers:
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
{
    "metadata": {
        "name": "named-fe1",
        "namespace": "nuclio",
        "labels": {
            "nuclio.io/function-name": "my-function"
        }
    },
    "spec": {
        "displayName": "My updated named function event",
        "body": "a body",
        "triggerKind": "http",
        "attributes": {
            "headers": {
                "x-header-1": "a",
                "x-header-2": 1
            },
            "method": "GET",
            "path": "/some/path"
        }
    }
}
```

#### Response
* Status code: 202

### Deleting a function event

#### Request
* URL: `DELETE /api/function_events`
* Headers:
  * `Content-Type`: Must be set to `application/json`
* Body:
```json
{
    "metadata": {
        "name": "named-fe1",
        "namespace": "nuclio"
    }
}
```
#### Response
* Status code: 204

## Function template

### Listing all function templates
#### Request
* URL: `GET /api/function_templates`
* Headers:
  * `x-nuclio-filter-contents`: Substring that appears either in name or configuration of the function (optional)

#### Response
* Status code: 200
* Body:
```json
{
	"dates:878fefcf-a5ef-4c9b-8099-09d6c57cb426": {
		"metadata": {
			"name": "dates:878fefcf-a5ef-4c9b-8099-09d6c57cb426"
		},
		"spec": {
			"description": "Uses moment.js (which is installed as part of the build) to add a specified amount of time to \"now\", and returns this amount as a string.\n",
			"handler": "handler",
			"runtime": "nodejs",
			"resources": {},
			"build": {
				"functionSourceCode": "<base64 encoded string>",
				"commands": [
					"npm install --global moment"
				]
			}
		}
	},
	"encrypt:1ce3c946-056a-4ea2-99f2-fa8491d8647c": {
		"metadata": {
			"name": "encrypt:1ce3c946-056a-4ea2-99f2-fa8491d8647c"
		},
		"spec": {
			"description": "Uses a third-party Python package to encrypt the event body, and showcases build commands for installing both OS-level and Python packages.\n",
			"handler": "encrypt:encrypt",
			"runtime": "python",
			"resources": {},
			"build": {
				"functionSourceCode": "<base64 encoded string>",
				"commands": [
					"apk update",
					"apk add --no-cache gcc g++ make libffi-dev openssl-dev",
					"pip install simple-crypt"
				]
			}
		}
	}
}
```

## Misc

### Getting version
#### Request
* URL: `GET /api/versions`

#### Response
* Status code: 200
* Body:
```json
{
    "dashboard": {
        "arch": "amd64",
        "gitCommit": "<some commit hash>",
        "label": "latest",
        "os": "darwin"
    }
}
```

### Getting external IP addresses
The user must know through which URL a function can be invoked in case load balancing / ingresses are not used. If the user concatenates one of the external IP addresses returned by this endpoint with the function's HTTP port, as specified in its spec/status - this will form a valid invocation URL.

#### Request
* URL: `GET /api/external_ip_addresses`

#### Response
* Status code: 200
* Body:
```json
{
    "externalIPAddresses": {
        "addresses": [
            ""
        ]
    }
}
```

