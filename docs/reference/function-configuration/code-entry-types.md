# Code Entry Types Reference

This document describes the various function code entry types.

#### In This Document

- [Basic Code Entry](#basic-code-entry)
- [External Code Entry Types](#external-code-entry-types)
- [Image Code Entry](#image-code-entry)
- [See also](#see-also)

## Basic code entry

In basic code entry type, the function configuration and the source code are taken from the passed spec. here's an example:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    functionSourceCode: "cGFja2FnZSBtYWluDQoNCmltcG9ydCAoDQogICAgImdpdGh1Yi5jb20vbnVjbGlvL251Y2xpby1zZGstZ28iDQopDQoNCmZ1bmMgSGFuZGxlcihjb250ZXh0ICpudWNsaW8uQ29udGV4dCwgZXZlbnQgbnVjbGlvLkV2ZW50KSAoaW50ZXJmYWNle30sIGVycm9yKSB7DQogICAgcmV0dXJuIG5pbCwgbmlsDQp9"
```

## External code entry types

Today, 3 external code entry types are supported - S3, Github and Archive.
When deploying a function with an external code entry type, the source code files will be fetched from the remote
 target - via github/s3 api or fetching and uncompressing the archive from the given url.
The source code will be taken from the given external source, and the given function configuration from the request
 will be enriched with the function configuration appearing in the remote code location.
The configuration merge will give precedence to the function configuration passed over the remote function configuration fields, using a merge strategy.
* Passed configuration values will override configuration in the remote source code (archive or other)
* List values will be merged (such as meta.annotations, meta.labels etc...), In particular, environment variables will be merged.
* When using external code entry all of the source code files are saved and can be used by the handler.
* External code entry types make use of `codeEntryAttributes` - a `headers` text2text map and a `workDir` string

Note: `spec.build.codeEntryType` is ignored when `spec.build.functionSourceCode` or `spec.image` are set;
 we infer the code-entry when these source fields are set (we first check image and then source code).

Archive code entry example:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "archive"
    path: "https://myhost.com/my-archive.zip"
    codeEntryAttributes:
      headers:
        X-V3io-Session-Key: "my-access-key"
      workDir: "/path/to/func"
```

Github code entry example:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "github"
    path: "https://github.com/ownername/myrepository"
    codeEntryAttributes:
      branch: "mybranch"
      headers:
        Authorization: "my-Github-access-key"
      workDir: "/path/to/func"
```

S3 code entry example:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    codeEntryType: "s3"
    codeEntryAttributes:
      s3Bucket: "my-s3-bucket"
      s3ItemKey: "my-folder/functions.zip"
      s3AccessKeyId: "my-@cc355-k3y"            # optional
      s3SecretAccessKey: "my-53cr3t-@cce55-k3y" # optional
      s3SessionToken: "my-s3ss10n-t0k3n"        # optional
      s3Region: "us-east-1"                     # optional (will be determined automatically when not mentioned)
      workDir: "/path/to/func"
```

** Note (for Go): `In order to import packages from different source code files in Go use: import "github.com/nuclio/handler/<package_name>"`

## Image code entry
Example:
```yaml
spec:
  description: my Go function
  image: example:latest
```

## See also
- [Function configuration](/docs/reference/function-configuration/function-configuration-reference.md)
