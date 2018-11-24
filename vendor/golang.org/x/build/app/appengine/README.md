# build.golang.org App Engine App

Update with

```sh
gcloud config set project golang-org
gcloud app deploy --no-promote -v {build|build-test} app.yaml
```

or, to not affect your gcloud state, use:

```
gcloud app --account=username@google.com --project=golang-org deploy --no-promote -v build app.yaml
```

Using -v build will run as build.golang.org.
Using -v build-test will run as build-test.golang.org.
