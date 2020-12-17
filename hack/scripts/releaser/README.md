## Releaser

A dev-internal go-based tool to create nuclio releases

Releaser flow

1. Clone nuclio to a temp dir
2. Merge your dev branch changes onto your release branch (if there any)
3. Create a Github release out of your release branch
4. Wait for the Build + Push nuclio images to finish
6. Bump + release helm charts with the target version

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
4. use github token to avoid API rate limiting

### Use cases

1. Release from development

```shell script
 go run releaser \
  --target-version 1.5.0 \
  --helm-charts-release-version 0.7.3
```

2. Release from a 1.x.y branch

```shell script
go run releaser \
  --development-branch 1.4.x \
  --release-branch 1.4.x \
  --target-version 1.4.18 \
  --helm-charts-release-version 0.6.19
```

3. Bump + publish helm charts only

```shell script
go run releaser \
  --skip-create-release \
  --current-version 1.4.17 \
  --target-version 1.4.18 \
  --development-branch 1.4.x \
  --release-branch 1.4.x \
  --helm-charts-release-version 0.6.19
``` 

> To use Github token, simply run releaser with `--github-token <my-token>`