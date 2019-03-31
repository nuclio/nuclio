# UI field validation

## Overview

This document describes the different restrictions for various input fields of function configuration in Nuclio UI.

## Dict of RegEx patterns used in this document

| Key  | Value  | Reference/source |
| :--- | :----- | :-------- |
| `configMapKey` | `^(?=[\S\s]{1,253}$)[-._a-zA-Z0-9]+$` | [K8s](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go#L334) |
| `container` | `^(?!.*--)(?!.*__)(?=.*[a-z])[a-z0-9][a-z0-9-_]*[a-z0-9]$\|^[a-z]$` | [Iguazio platform](https://github.com/iguazio/zebo/blob/development/py/services/container_provisioning/__init__.py#L670) |
| `dns1123Label` | `^(?=[\S\s]{1,63}$)[a-z0-9]([-a-z0-9]*[a-z0-9])?$` | [K8s](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go#L116) |
| `dns1123Subdomain` | `^(?=[\S\s]{1,253}$)[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$` | [K8s](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go#L137) |
| `dockerReference` | `^(([a-zA-Z0-9]\|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])(\.([a-zA-Z0-9]\|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]))*(\:\d+)?\/)?[a-z0-9]+(([._]\|__\|[-]*)[a-z0-9]+)*(\/[a-z0-9]+(([._]\|__\|[-]*)[a-z0-9]+)*)*(\:[\w][\w.-]{0,127})?(\@[A-Za-z][A-Za-z0-9]*([-_+.][A-Za-z][A-Za-z0-9]*)*\:[0-9a-fA-F]{32,})?$` | [Docker](https://github.com/docker/distribution/blob/master/reference/regexp.go) |
| `envVarName` | `^(?\!\.$)(?!\.\.[\S\s]*$)[-._a-zA-Z][-._a-zA-Z0-9]*$` | [K8s](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go#L318) |
| `prefixedQualifiedName` | `^(?!kubernetes.io\/)(?!k8s.io\/)((?=[\S\s]{1,253}\/)([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\/))?(?=[\S\s]{1,63}$)([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$` | [K8s](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go#L42) |
| `qualifiedName` | `^(?=[\S\s]{1,63}$)([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$` | [K8s](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go#L36) |
| `wildcardDns1123Subdomain` | `^(?=[\S\s]{1,253}$)\*\.[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$` | [K8s](https://github.com/kubernetes/apimachinery/blob/master/pkg/util/validation/validation.go#L182) |

## Function Configuration Input Validation

| Panel | Field | Is unique? | Validation pattern | Max length | Tooltip on hovering text box |
|:----- |:----- | :--------- | :----------------- | :--------- | :--------------------------- |
| Annotations | Key | Yes | `prefixedQualifiedName` | 317 | Annotation key must not be more than 63 characters and must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName', 'my.name_123', or '123-abc'). An optional prefix could be prepended and must be no more than 253 characters, followed by a forward-slash, and must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'sub-domain.example.com/MyName'). |
| Annotations | Value | No | - | - | |
| Build | Image name | No | `dockerReference` | 255 |  |
| Environment Variables | Key | Yes | `envVarName`| - | Environment variable key must consist of alphanumeric characters, '_', '-', or '.', and must not start with a digit (e.g. 'my.env-name', 'MY_ENV.NAME', 'MyEnvName1'). Must not equal to '.' or '..', or start with '..'. |
| Environment Variables (ConfigMap) | ConfigMap key | Yes | `configMapKey`|253|Config key must not be more than 253 characters and must consist of alphanumeric characters, '-', '_' or '.' (e.g. 'key.name', 'KEY_NAME', 'key-name'). |
| Labels | Key | Yes | `prefixedQualifiedName` | 317 | Label key must not be more than 63 characters and must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName', 'my.name_123', or '123-abc'). An optional prefix could be prepended and must be no more than 253 characters, followed by a forward-slash, and must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character (e.g. 'sub-domain.example.com/MyName'). |
| Labels | Value | No | `qualifiedName` | 63 | Label value must not be more than 63 characters and must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyValue', 'my_value', or '12345'). |
| Volumes | Name | Yes | `dns1123Label` | 63 | Volume name must not be more than 63 characters and must consist of lower-case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc'). |
| Volumes | Mount Path | Yes | - | 255 | |
| Volumes (V3IO) | Container Name | No | `container` | 128 | |
| Volumes (V3IO) | Sub Path | No | - | 255 | |
