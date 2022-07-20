# API Gateway

#### In this document

- [No Authentication](#none-auth)
    - [Create](#create-none)
    - [Invoke](#invoke-none)
- [Basic Authentication](#basic-auth)
    - [Create](#create-basic)
    - [Invoke](#invoke-basic)
- [Canary Function](#canary-function)

<a id="none-auth"></a>
## No Authentication

<a id="create-none"></a>
### Create API Gateways

You can create an api gateway with basic authentication by sending a POST request to the following endpoint:

```
<nuclio-host-name>/api/api_gateways
```
With the following request body:
```json
{
    "spec": {
        "name": "<apigateway-name>",
        "description": "some description",
        "path": "/some/path", 
        "authenticationMode": "none",
        "upstreams": [
            {
                "kind": "nucliofunction",
                "nucliofunction": {
                    "name": "function-name-to-invoke"
                },
                "percentage": 0
            }
        ],
        "host": "<apigateway-name>-<project-name>.<nuclio-host-name>"
    },
    "metadata": {
        "labels": {
            "nuclio.io/project-name": "default"
        },
        "name": "<apigateway-name>"
    }
}
```

<a id="invoke-none"></a>
### Invoke API Gateways

To invoke it, simply send a request to the created API Gateway ingress (e.g.: `<apigateway-name>-<project-name>.<nuclio-host-name>`,
specified on the request body `spec.host` )

<a id="basic-auth"></a>
## Basic Authentication

You may protect your function by setting the API Gateway with [basic authentication](https://en.wikipedia.org/wiki/Basic_access_authentication). 
That means, to access a function, client would need to provide both username & password.

<a id="create-basic"></a>
### Create API Gateways

You can create an api gateway with basic authentication by sending a POST request to the following endpoint:

```
<nuclio-host-name>/api/api_gateways
```
With the following request body:
```json
{
    "spec": {
        "name": "<apigateway-name>",
        "description": "some description",
        "path": "/some/path",
        "authenticationMode": "basicAuth",
        "upstreams": [
            {
                "kind": "nucliofunction",
                "nucliofunction": {
                    "name": "function-name-to-invoke"
                },
                "percentage": 0
            }
        ],
        "host": "<apigateway-name>-<project-name>.<nuclio-host-name>",
        "authentication": {
            "basicAuth": {
                "username": "some-username",
                "password": "some-password"
            }
        }
    },
    "metadata": {
        "labels": {
            "nuclio.io/project-name": "default"
        },
        "name": "<apigateway-name>"
    }
}
```

<a id="invoke-basic"></a>
### Invoke API Gateways

To invoke it, simply send a request to the created API Gateway ingress (e.g.: `<apigateway-name>-<project-name>.<nuclio-host-name>`,
specified on the request body `spec.host`) with the following header:
```
key: Authorization
value: Basic base64encode("username:password")
```

*Example*:

For the following credentials:
```
"username": "some-username"
"password": "some-password"
```
base64 encoding of "some-username:some-password" is `c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk`(`echo "some-username:some-password" | base64 -d`), 
so the resulting header is:

```
Authorization: Basic c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk
```

Invoking the function without the above header will respond with `401 Authorization Required`

<a id="canary-function"></a>
## Canary Function

You can control the percentage of traffic that goes to a canary function by changing the percentage of the upstream.

Add a canary function to an api gateway by adding another upstream to the api gateway, and set its `"percentage"` to a value between 0 and 100.
Make sure to set the percentage of the first function accordingly.

For instance, if you have two functions, `function-1` and `function-2`, and you want to have 80% of the traffic go to `function-1` and 20% of the traffic go to `function-2`, you can do the following:

```json
{
    "spec": {
        "name": "<apigateway-name>",
        "description": "some description",
        "path": "/some/path", 
        "authenticationMode": "none",
        "upstreams": [
            {
                "kind": "nucliofunction",
                "nucliofunction": {
                    "name": "function-1"
                },
                "percentage": 80
            },
            {
              "kind": "nucliofunction",
              "nucliofunction": {
                "name": "function-2"
              },
              "percentage": 20
            }
        ],
        "host": "<apigateway-name>-<project-name>.<nuclio-host-name>"
    },
    "metadata": {
        "labels": {
            "nuclio.io/project-name": "default"
        },
        "name": "<apigateway-name>"
    }
}
```