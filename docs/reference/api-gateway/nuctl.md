# API Gateway with nuctl

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


<a id="none-auth"></a>
## Without Authentication

<a id="create-none"></a>
### Create API Gateways

<a id="invoke-none"></a>
### Invoke API Gateways

<a id="basic-auth"></a>
## Basic Authentication

Basic authentication is a way to authenticate users by providing a username and password.
You can create an api gateway with basic authentication by running the following command:

```
$ nuctl create apigateway <api-gateway-name> \
			--host <api-gateway-name>-<project-name>.<nuclio-host-name> \
			--path "/some/path" \
			--description "some-description" \
			--function some-function-name \
			--authentication-mode "basicAuth" \
			--basic-auth-username <some-username> \
			--basic-auth-password <some-password> \
			--namespace <namespace>
```

<a id="create-basic"></a>
### Create API Gateways

<a id="invoke-basic"></a>
### Invoke API Gateways

<a id="access-key-auth"></a>
## Access Key Authentication

<a id="create-access-key"></a>
### Create API Gateways

<a id="invoke-access-key"></a>
### Invoke API Gateways

<a id="oauth2-auth"></a>
## OAuth2

<a id="create-oath2"></a>
### Create API Gateways

<a id="invoke-oath2"></a>
### Invoke API Gateways
