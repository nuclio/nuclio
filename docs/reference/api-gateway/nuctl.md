# API Gateway with Nuctl

#### In This Section

- [No Authentication](#none-auth)
- [Basic Authentication](#basic-auth)
- [Delete an API Gateway](#delete)

You can create API Gateways using `nuctl` - the Nuclio CLI tool.

<a id="none-auth"></a>
### No authentication
```
$ nuctl create apigateway <api-gateway-name> \
			--host <api-gateway-name>-<project-name>.<nuclio-host-name> \
			--path "/some/path" \
			--description "some-description" \
			--function some-function-name \
			--authentication-mode "none" \
			--namespace <namespace>
```

For invoking the function using the api gateway, see [invoking API Gateways](./http.md#invoke-none).

<a id="basic-auth"></a>
## Basic authentication
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

To invoke the function using the API gateway, see [invoking API Gateways with basic authentication](./http.md#invoke-basic).

<a id="delete"></a>
## Delete an API Gateway

To delete an API Gateway with nuctl, run the following command:
```
$ nuctl --namespace <namespace> delete apigateway <api-gateway-name>
```