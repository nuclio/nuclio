# Code-Entry Types

This document describes the Nuclio function code-entry types and related configuration fields.

#### In This Document

- [Overview](#overview)
  - [Determining the code-entry type](#code-entry-type-determine)
  - [Configuring the code-entry type from the dashboard](#dashboard-configuration)
- [Function-image code-entry type (`image`)](#code-entry-type-image)
- [Function source-code entry types](#func-source-code-entry-types)
  - [Encoded source-code string code-entry type (`sourceCode`)](#code-entry-type-sourcecode)
  - [Source-code file code-entry type](#code-entry-type-codefile)
- [External function-code entry types](#external-func-code-entry-types)
  - [Git code-entry type (`git`)](#code-entry-type-git)
  - [GitHub code-entry type (`github`)](#code-entry-type-github)
  - [Archive-file code-entry type (`archive`)](#code-entry-type-archive)
  - [AWS S3 code-entry type (`s3`)](#code-entry-type-s3)
- [See also](#see-also)

## Overview

As part of the [function specification](/docs/reference/function-configuration/function-configuration-reference.md#specification) (`spec`), you must configure one of the following code-entry types and related information that points either to a pre-built function image or to code from which to build such an image:

- Function image (`image`) &mdash; set the `spec.image` configuration field to the name of a function container image. See [Function-image code-entry type (`image`)](#code-entry-type-image).

- Function source code &mdash; provide the function source code either by setting the `spec.build.functionSourceCode` configuration field to an [encoded source-code string](#code-entry-type-sourcecode) (`sourceCode`), or by setting the  `spec.build.path` field to a URL for downloading a [function source-code file](#code-entry-type-codefile). See [Function source-code entry types](#func-source-code-entry-types).

- External function code &mdash; set the `spec.build.codeEntryType` configuration field to a code-entry type for downloading the function's source code and optional additional configuration ("function code") from an external source &mdash; [GitHub repository](#code-entry-type-github) (`github`), [archive file](#code-entry-type-archive) (`archive`), or [AWS S3 bucket](#code-entry-type-s3) (`s3`) &mdash; and configure the required download information.
  See [External function-code entry types](#external-func-code-entry-types).

> **Go Note**<br/>
> To import packages in Go source code, use the following syntax:<br/>
> `import "github.com/nuclio/handler/<package_name>"`

<a id="code-entry-type-determine"></a>
### Determining the code-entry type

The code-entry type is determined by using the following processing logic:

1. If [`spec.image`](/docs/reference/function-configuration/function-configuration-reference.md#spec.image) is set, the implied code-entry type is [function image](#code-entry-type-image) (`image`) and the configured function image is used. The `spec.build.codeEntryType`, `spec.build.functionSourceCode`, and `spec.build.path` fields are ignored.

   > **Note:** When you build and deploy a Nuclio function, the `spec.image` field is automatically updated to the name of the function's container image, so to use a different code-entry type for a redeployed function you must first reset the `spec.image` configuration field. This is handled implicitly when deploying a function from the Nuclio dashboard.

2. If [`spec.build.functionSourceCode`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.functionSourceCode) is set (and `spec.image` isn't set), the implied code-entry type is [encoded source-code string](#code-entry-type-sourcecode) (`sourceCode`) and the function is built from the configured source code. The `spec.build.codeEntryType` and `spec.build.path` fields are ignored.

3. If [`spec.build.codeEntryType`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.codeEntryType) is set (and `spec.image` and `spec.build.functionSourceCode` aren't set), the value of the code-entry field determines the [external function-code code-entry type](#external-func-code-entry-types) (`archive`, `github`, or `s3`).

4. If [`spec.build.path`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.path) is set (and `spec.image`, `spec.build.functionSourceCode`, and `spec.build.codeEntryType` aren't set), the implied code-entry type is [source-code-file](#code-entry-type-codefile) and the function is built from the configured source code.

<a id="dashboard-configuration"></a>
### Configuring the code-entry type from the dashboard

When deploying a function from the Nuclio dashboard, you select the code-entry type from the **Code entry type** drop-down list in the **Code** function tab.
Additional configuration parameters are displayed according to the selected entry type.
When you select to deploy the function, the respective function-configuration parameters are automatically updated based on your dashboard configuration.
The dashboard notes in this reference refer to fields in the **Code** function dashboard tab.

<a id="code-entry-type-image"></a>
## Function-image code-entry type (`image`)

Set the [`spec.image`](/docs/reference/function-configuration/function-configuration-reference.md#spec.image) function-configuration field to the name of a function container image (`[<host name>.]<namespace>.<repository>[:<tag>]`) to deploy the function from this image.

> **Note:** When `spec.image` is set, the implied code-entry type is `image` and `spec.build.codeEntryType` and `spec.build.path` are ignored. See [Determining the code-entry type](#code-entry-type-determine).

> **Dashboard Note:** To configure a function image from the dashboard, select `Image` from the **Code entry type** list, and then enter the image name in the **URL** field.

<a id="code-entry-type-image-example"></a>
### Example

```yaml
spec:
  description: my Go function
  image: mydockeruser/my-func:latest
```

<a id="func-source-code-entry-types"></a>
## Function source-code entry types

Use either of the following methods to provide the function source code:

- Set the `spec.build.functionSourceCode` configuration field to the function's source code, encoded as a Base64 string. This implies the `sourceCode` code-entry type. See [Encoded source-code string code-entry type (`sourceCode`)](#code-entry-type-sourcecode).
- Set the `spec.build.path` configuration field to a URL for downloading a function source-code file. See [Source-code file code-entry type](#code-entry-type-codefile).

<a id="code-entry-type-sourcecode"></a>
### Encoded source-code string code-entry type (`sourceCode`)

Set the [`spec.build.functionSourceCode`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.functionSourceCode) function-configuration field to the function's source code, encoded as a Base64-encoded string, to build the function image from this code.

> **Note:** When `spec.build.functionSourceCode` is set and `spec.image` isn't set, the implied code-entry type is `sourceCode` and `spec.build.codeEntryType` and `spec.build.path` are ignored. When `spec.image` is set, `spec.build.functionSourceCode` is ignored. See [Determining the code-entry type](#code-entry-type-determine).

> **Dashboard Note:** To configure the function source code from the dashboard, select `Edit online` from the **Code entry type** list (default), and then edit the code in the unnamed function-code text box, as needed.
> When you select to deploy the function, the source code will automatically be encoded as a Base64 encoded string.

<a id="code-entry-type-sourcecode-example"></a>
#### Example

```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    functionSourceCode: "cGFja2FnZSBtYWluDQoNCmltcG9ydCAoDQogICAgImdpdGh1Yi5jb20vbnVjbGlvL251Y2xpby1zZGstZ28iDQopDQoNCmZ1bmMgSGFuZGxlcihjb250ZXh0ICpudWNsaW8uQ29udGV4dCwgZXZlbnQgbnVjbGlvLkV2ZW50KSAoaW50ZXJmYWNle30sIGVycm9yKSB7DQogICAgcmV0dXJuIG5pbCwgbmlsDQp9"
```

<a id="code-entry-type-codefile"></a>
### Source-code file code-entry type

Set the `spec.build.path` function-configuration field to a URL for downloading a function source-code file.

> **Note:** This code-entry type is applicable only when `spec.image`, `spec.build.functionSourceCode`, and `spec.build.codeEntryType` aren't set. See [Determining the code-entry type](#code-entry-type-determine).

> **Dashboard Note:** The source-code file code-entry type isn't supported from the dashboard.

<a id="code-entry-type-codefile-example"></a>
#### Example

```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    path: "https://www.my-host.com/my-function.go"
```

<a id="external-func-code-entry-types"></a>
## External function-code entry types

Set the [`spec.build.codeEntryType`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.codeEntryType) function-configuration field to one of the following code-entry types to download the function code from the respective external source:

- `github` &mdash; download the code from a GitHub repository. See [GitHub code-entry type (`github`)](#code-entry-type-github).
- `archive` &mdash; download the code as an archive file from an Iguazio Data Science Platform data container (authenticated) or from any URL that doesn't require download authentication. See [Archive-file code-entry type (`archive`)](#code-entry-type-archive).
- `s3` &mdash; download the code as an archive file from an AWS S3 bucket. See [AWS S3 code-entry type (`s3`)](#code-entry-type-s3).

Additional information for performing the download &mdash; such as the download URL or authentication information &mdash; is provided in dedicated configuration fields for each code-entry type, as detailed in the documentation of each code-entry type.

> **Note:**
> - When `spec.image` or `spec.build.functionSourceCode` are set, `spec.build.codeEntryType` is ignored. See [Determining the code-entry type](#code-entry-type-determine).
> - <a id="archive-file-formats"></a>The `archive` and `s3` code-entry types support the following archive-file formats: **\*.jar**, **\*.rar**, **\*.tar**, **\*.tar.bz2**, **\*.tar.lz4**, **\*.tar.gz**, **\*.tar.sz**, **\*.tar.xz**, **\*.zip**
> - The downloaded code files are saved and can be used by the function handler.

> **Dashboard Note:** To configure an external function-code source from the dashboard, select the relevant code-entry type &mdash; `Archive`, `Git`, `GitHub`, or `S3` &mdash; from the **Code entry type** list.

The downloaded function code can optionally contain a **function.yaml** file with function configuration for enriching the original configuration (in the configuration file that sets the code-entry type) according to the following merge strategy:

- Field values that are set only in the downloaded configuration are added to the original configuration.
- List and map field values &mdash; such as `meta.labels` and `spec.env` &mdash;are merged by adding any values that are set only in the downloaded configuration to the values that are set in the original configuration.
- In case of a conflict &mdash; i.e., if the original and downloaded configurations set different values for the same element &mdash; the original configuration takes precedence and the value in the downloaded configuration is ignored.

<a id="code-entry-type-git"></a>
### Git code-entry type (`git`)

Set the [`spec.build.codeEntryType`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.codeEntryType) function-configuration field to `git` (dashboard: **Code entry type** = `Git`) to clone the function code from a Git repository. The following configuration fields provide additional information for performing the cloning:

- `spec.build` &mdash;
  - `path` (dashboard: **URL**) (Required) &mdash; the URL of the Git repository that contains the function code.
  - `codeEntryAttributes` &mdash;

      // must use one of the following as git reference
      - `branch` (dashboard: **Branch**) &mdash; the Git repository branch from which to download the function code.
      - `tag` (dashboard: **Tag**) &mdash; the Git repository tag from which to download the function code.
      - `reference` (dashboard: **Reference**) &mdash; the Git repository reference from which to download the function code.

      - `username` (dashboard: **Username**) (Optional) Git username
      - `password` (dashboard: **Password**) (Optional) Git password
      - `workDir` (dashboard: **Work directory**) (Optional) &mdash; the relative path to the function-code directory within the configured repository.
      The default work directory is the root directory of the git repository (`"/"`).

<a id="code-entry-type-git-example"></a>
#### Examples

Using Branch:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "git"
    path: "https://bitbucket.org/<my-user>/<my-repo>"
    codeEntryAttributes:
      workDir: "/go-function"
      branch: "go-func"

      # Uncomment in case of a private repository
      #
      #  username: "myusername"
      #  password: "mypassword"
```

Using Tag:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "git"
    path: "https://bitbucket.org/<my-user>/<my-repo>"
    codeEntryAttributes:
      workDir: "/go-function"
      tag: "0.0.1"
```

Using Full Reference:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "git"
    path: "https://bitbucket.org/<my-user>/<my-repo>"
    codeEntryAttributes:
      workDir: "/go-function"
      reference: "refs/heads/go-func"
```

<a id="code-entry-type-github"></a>
### GitHub code-entry type (`github`)

Set the [`spec.build.codeEntryType`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.codeEntryType) function-configuration field to `github` (dashboard: **Code entry type** = `GitHub`) to download the function code from a GitHub repository. The following configuration fields provide additional information for performing the download:

- `spec.build` &mdash;
  - `path` (dashboard: **URL**) (Required) &mdash; the URL of the GitHub repository that contains the function code.
  - `codeEntryAttributes` &mdash;
      - `branch` (dashboard: **Branch**) (Required) &mdash; the GitHub repository branch from which to download the function code.
      - `headers.Authorization` (dashboard: **Token**) (Optional) &mdash; a GitHub access token for download authentication.
      - `workDir` (dashboard: **Work directory**) (Optional) &mdash; the relative path to the function-code directory within the configured repository branch.
      The default work directory is the root directory of the GitHub repository (`"/"`).

<a id="code-entry-type-github-example"></a>
#### Example

```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "github"
    path: "https://github.com/my-organization/my-repository"
    codeEntryAttributes:
      branch: "my-branch"
      headers:
        Authorization: "my-Github-access-token"
      workDir: "/go/myfunc"
```

<a id="code-entry-type-archive"></a>
### Archive-file code-entry type (`archive`)

Set the [`spec.build.codeEntryType`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.codeEntryType) function-configuration field to `archive` (dashboard: **Code entry type** = `Archive`) to download [an archive file](#archive-file-formats) of the function code from one of the following sources:

- An [Iguazio Data Science Platform](https://www.iguazio.com) ("platform") data container. Downloads from this source require user authentication.
- Any URL that doesn't require user authentication to perform the download.

The following configuration fields provide additional information for performing the download:

- `spec.build` &mdash;
  - `path` (dashboard: **URL**) (Required) &mdash; a URL for downloading the archive file.<br/>
    To download an archive file from an Iguazio Data Science Platform data container, the URL should be set to `<API URL of the platform's web-APIs service>/<container name>/<path to archive file>`, and a respective data-access key must be provided in the `spec.build.codeEntryAttributes.headers.X-V3io-Session-Key` field.
  - `codeEntryAttributes` &mdash;
      - `headers.X-V3io-Session-Key` (dashboard: **Access key**) (Required for a platform archive file) &mdash; an Iguazio Data Science Platform access key, which is required when the download URL (`spec.build.path`) refers to an archive file in a platform data container.
      - `workDir` (dashboard: **Work directory**) (Optional) &mdash; the relative path to the function-code directory within the extracted archive-file directory.
        The default work directory is the root of the extracted archive-file directory (`"/"`).

<a id="code-entry-type-archive-example"></a>
#### Example

```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "archive"
    path: "https://webapi.default-tenant.app.mycluster.iguazio.com/users/myuser/my-functions.zip"
    codeEntryAttributes:
      headers:
        X-V3io-Session-Key: "my-platform-access-key"
      workDir: "/go/myfunc"
```

<a id="code-entry-type-s3"></a>
### AWS S3 code-entry type (`s3`)

Set the [`spec.build.codeEntryType`](/docs/reference/function-configuration/function-configuration-reference.md#spec.build.codeEntryType) function-configuration field to `s3` (dashboard: **Code entry type** = `S3`) to download [an archive file](#archive-file-formats) of the function code from an Amazon Simple Storage Service (AWS S3) bucket. The following configuration fields provide additional information for performing the download:

- `spec.build.codeEntryAttributes` &mdash;
  - `s3Bucket` (dashboard: **Bucket**) (Required) &mdash; the name of the S3 bucket that contains the archive file.
  - `s3ItemKey` (dashboard: **Item key**) (Required) &mdash; the relative path to the archive file within the bucket.
  - `s3AccessKeyId` (dashboard: **Access key ID**) (Optional) &mdash; an S3 access key ID for download authentication.
  - `s3SecretAccessKey` (dashboard: **Secret access key**) (Optional) &mdash; an S3 secret access key for download authentication.
  - `s3SessionToken` (dashboard: **Session token**) (Optional) &mdash; an S3 session token for download authentication.
  - `s3Region` (dashboard: **Region**) (Optional) &mdash; the AWS Region of the configured bucket. When this parameter isn't provided, it's implicitly deduced.
  - `workDir` (dashboard: **Work directory**) (Optional) &mdash; the relative path to the function-code directory within the extracted archive-file directory.
      The default work directory is the root of the extracted archive-file directory (`"/"`).

<a id="code-entry-type-s3-example"></a>
#### Example

```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "s3"
    codeEntryAttributes:
      s3Bucket: "my-s3-bucket"
      s3ItemKey: "my-folder/my-functions.zip"
      s3AccessKeyId: "my-@cc355-k3y"
      s3SecretAccessKey: "my-53cr3t-@cce55-k3y"
      s3SessionToken: "my-s3ss10n-t0k3n"
      s3Region: "us-east-1"
      workDir: "/go/myfunc"
```

## See also

- [Function-Configuration Reference](/docs/reference/function-configuration/function-configuration-reference.md)
