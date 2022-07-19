# API Gateway with nuctl

#### In this document

- [Without Authentication](#none-auth)
- [Basic Authentication](#basic-auth)

You can create API Gateways using `nuctl` - the Nuclio cli tool.

<a id="none-auth"></a>
### Without Authentication
```
$ nuctl create apigateway <api-gateway-name> \
			--host <api-gateway-name>-<project-name>.<nuclio-host-name> \
			--path "/some/path" \
			--description "some-description" \
			--function some-function-name \
			--authentication-mode "none" \
			--namespace <namespace>
```

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
