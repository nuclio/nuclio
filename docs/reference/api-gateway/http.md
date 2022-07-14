# API Gateway with http requests

#### In this document

- [Without Authentication](#none-auth)
    - [Create](#create-none)
    - [Invoke](#invoke-none)
- [Basic Authentication](#basic-auth)
    - [Create](#create-basic)
    - [Invoke](#invoke-basic)
- [Access Key Authentication](#access-key-auth)
    - [Create](#create-access-key)
    - [Invoke](#invoke-access-key)
- [OAuth2 Authentication](#oath2-auth)
    - [Create](#create-oath2)
    - [Invoke](#invoke-oath2)
- [Canary Function](#canary-function)

- <a id="none-auth"></a>
## Without Authentication

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
                    "name": "function-name-to-invoke",
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

To invoke it, simply send GET/POST request to `<apigateway-name>-<project-name>.<nuclio-host-name>` (aka - `spec.host` )
If your function accepts a request body, you can send it as a request body.

<a id="basic-auth"></a>
## Basic Authentication

Basic authentication is a way to authenticate users by providing a username and password.


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

To invoke it, send GET/POST request to `<apigateway-name>-<project-name>.<nuclio-host-name>` (aka - `spec.host` )
with the following header:
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
base64 encoding of "some-username:some-password" is `c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk`, so the resulting header is:
```
Authorization: Basic c29tZS11c2VybmFtZTpzb21lLXBhc3N3b3Jk
```

Invoking the function without the above header will respond with `401 Authorization Required`

<a id="access-key-auth"></a>
## Access Key Authentication

<a id="create-access-key"></a>
### Create API Gateways

Access key authentication is a way to authenticate users by providing an access key.

To create an api gateway with access key authentication, change the `"authenticationMode"` in the POST request body's spec to the `"accessKey"`

*TOMER - Which access key is used??*

<a id="invoke-access-key"></a>
### Invoke API Gateways

<a id="oauth2-auth"></a>
## OAuth2

<a id="create-oath2"></a>
### Create API Gateways

O

<a id="invoke-oath2"></a>
### Invoke API Gateways

<a id="canary-function"></a>
## Canary Function

You can add a canary function to an api gateway by adding another upstream to the api gateway.

You can control the percentage of traffic that goes to the canary function by changing the percentage of the upstream.

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
                    "name": "function-1",
                },
                "percentage": 80
            },
            {
              "kind": "nucliofunction",
              "nucliofunction": {
                "name": "function-2",
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