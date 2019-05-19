# Code Entry Types Reference

This document describes the various function code entry types.

#### In This Document

- [Basic Code Entry](#basic-code-entry)
- [External Code Entries](#external-code-entries)
- [Image Code Entries](#image-code-entry)
- [See also](#see-also)

## Basic code entry

In the basic code entry, the function configuration and the source code are taken from the passed spec. here's an example:
(The source code is passed as base64 string)
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  build:
    functionSourceCode: "cGFja2FnZSBtYWluDQoNCmltcG9ydCAoDQogICAgImdpdGh1Yi5jb20vbnVjbGlvL251Y2xpby1zZGstZ28iDQopDQoNCmZ1bmMgSGFuZGxlcihjb250ZXh0ICpudWNsaW8uQ29udGV4dCwgZXZlbnQgbnVjbGlvLkV2ZW50KSAoaW50ZXJmYWNle30sIGVycm9yKSB7DQogICAgcmV0dXJuIG5pbCwgbmlsDQp9"
    codeEntryType: "sourceCode"
```

## External code entries

There are two types of external code entries - Github and Archive.
When external code entry type is set, the function source code and configuration are being fetched from the code entry.
When deploying this way, the source code files will be fetched from the code entry and the function configuration will 
get enriched with the code entry's function configuration.
* The code entry's function configuration values will not override the passed configuration values.
* List values will be merged (such as meta.annotations, meta.labels etc...).
* The environment variables will also be merged.
* When using external code entry all of the source code files are saved and can be used by the handler.
**(see the note about using Go packages)

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

** Note (for Go): `In order to import packages from different source code files in Go use: import "github.com/nuclio/handler/<package_name>"`

## Image code entry
Example:
```yaml
spec:
  description: my Go function
  handler: main:Handler
  runtime: golang
  image: "https://my-image-url.com"
  build:
    codeEntryType: "image"
```

## See also
- [Function configuration](/docs/reference/function-configuration/function-configuration-reference.md)
