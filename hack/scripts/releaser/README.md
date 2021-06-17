## Releaser

A dev-internal Go-based tool to create Nuclio releases

Releaser flow

1. Clone Nuclio to a temp dir
2. Merge your dev branch changes onto your release branch (if there is any)
3. Create a GitHub release out of the _release_ branch
4. Wait for the Build + Push Nuclio images to finish
6. Bump + release Nuclio Helm charts with the target version

### Pre Requisites

1. `go mod download` - to ensure you have all go modules downloaded
2. You are running from development, to ensure you have the very latest version of `releaser.go`
3. if you have gpg and commit signature verification enabled:
    1. ensure you have `pinentry-mac` installed (`brew install gnupg pinentry-mac`)
    2. cd `~/.gnupg`
    3. `vi gpg-agent.conf` && add `pinentry-program /usr/local/bin/pinentry-mac`
    4. `echo "use-agent" > gpg.conf`
    5. `killall gpg-agent`
    6. now gnupg would use macOS keychain to prompt for a password and releaser would be able to use your git commands
4. use a scope-less GitHub token to avoid API rate limiting

### Use cases

- Release from development, bump patch version

```shell
 go run releaser.go \
  --bump-patch \
  --github-token <my-scopeless-token>
```

- Release from development, explicit target versions

```shell script
 go run releaser.go \
  --target-version 1.5.0 \
  --helm-charts-release-version 0.7.3
  --github-token <my-scopeless-token>
```

- Release from a 1.x.y branch

```shell script
go run releaser.go \
  --development-branch 1.4.x \
  --release-branch 1.4.x \
  --target-version 1.4.18 \
  --helm-charts-release-version 0.6.19
```

- Publish Nuclio Helm charts

Steps:

1. Create a PR against development with your changes, wait for it to pass CI
2. Merge to development
3. Run the following snippet

```shell script
go run releaser.go \
  --skip-create-release \
  --helm-charts-release-version x.y.z
```

> To use GitHub token, simply run releaser with `--github-token <my-token>`
> or set it via env `NUCLIO_RELEASER_GITHUB_TOKEN`
