# Go Build Tools

This subrepository holds the source for various packages and tools that support
Go's build system and the development of the Go programming language.

## Report Issues / Send Patches

This repository uses Gerrit for code changes. To contribute, see
https://golang.org/doc/contribute.html.

The main issue tracker for the blog is located at
https://github.com/golang/go/issues. Prefix your issue with
"`x/build/DIR: `" in the subject line.

## Overview

The main components of the Go build system are:

* The **dashboard**, in [app/](https://dev.golang.org/dir/build/app), serves
  https://build.golang.org/. It runs on App Engine and holds the state for
  which builds passed or failed, and stores the build failure logs for
  post-submit failures. (Trybot build failure logs are stored elsewhere).
  The dashboard does not execute any builds on its own.


* The **coordinator**, in
  [cmd/coordinator/](https://dev.golang.org/dir/build/cmd/coordinator/),
  serves https://farmer.golang.org/. It runs on GKE and coordinates the
  whole build system. It finds work to do (both pre-submit "TryBot" work,
  and post-submit work) and executes builds, allocating machines to run the
  builds. It is the owner of all machines.

* The Go package in [buildenv/](https://dev.golang.org/dir/build/buildenv/)
  contains constants for where the dashboard and coordinator run, for prod,
  staging, and local development.

* The **buildlet**, in
  [cmd/buildlet/](https://dev.golang.org/dir/build/cmd/buildlet/), is the
  HTTP server that runs on each worker machine to execute builds on the
  coordinator's behalf. This runs on every possible GOOS/GOARCH value. The
  buildlet binaries are stored on Google Cloud Storage and fetched
  per-build, so we can update the buildlet binary independently of the
  underlying machine images. The buildlet is the most insecure server
  possible: it has HTTP handlers to read & write arbitrary content to disk,
  and to execute any file on disk. It also has an SSH tunnel handler. The
  buildlet must never be exposed to the Internet. The coordinator provisions
  buildlets in one of three ways:

  1. by creating VMs on Google Compute Engine (GCE) with custom images
  configured to fetch & run the buildlet on boot, listening on port 80 in a
  private network.
  
  2. by running Linux containers (on either Google Kubernetes Engine
  or GCE with the Container-Optimized OS image), with the container
  images configured to fetch & run the buildlet on start, also
  listening on port 80 in a private network.

  3. by taking buildlets out of a pool of connected, dedicated machines. The
  buildlet can run in either *listen mode* (as on GCE and GKE) or in
  *reverse mode*. In reverse mode, the buildlet connects out to
  https://farmer.golang.org/ and registers itself with the coordinator. The
  TCP connection is then logically reversed (using
  [revdial](https://dev.golang.org/dir/build/revdial/) and when the
  coordinator needs to do a build, it makes HTTP requests to the coordinator
  over the already-open TCP connection.

  These three pools can be viewed at the coordinator's
  http://farmer.golang.org/#pools


* The [env/](https://dev.golang.org/dir/build/env/) directory describes
  build environments. It contains scripts to create VM images, Dockerfiles
  to create Kubernetes containers, and instructions and tools for dedicated
  machines.


* **maintner** in [maintner/](https://dev.golang.org/dir/build/maintner) is
  a library for slurping all of Go's GitHub and Gerrit state into memory.
  The daemon **maintnerd** in
  [maintner/maintnerd/](https://dev.golang.org/dir/build/maintner/maintnerd)
  runs on GKE and serves https://maintner.golang.org/. The daemon watches
  GitHub and Gerrit and apps to a mutation log whenever it sees new
  activity. The logs are stored on GCS and served to clients.


* The [godata package](https://godoc.org/golang.org/x/build/maintner/godata)
  in [maintner/godata/](https://dev.golang.org/dir/build/maintner/godata)
  provides a trivial API to let anybody write programs against
  Go's maintner corpus (all of our GitHub and Gerrit history), live up
  to the second. It takes a few seconds to load into memory and a few hundred
  MB of RAM after it downloads the mutation log from the network.


* **pubsubhelper** in
  [cmd/pubsubhelper/](https://dev.golang.org/dir/build/cmd/pubsubhelper/) is
  a dependency of maintnerd. It runs on GKE, is available at
  https://pubsubhelper.golang.org/, and runs an HTTP server to receive
  Webhook updates from GitHub on new activity and an SMTP server to receive
  new activity emails from Gerrit. It then is a pubsub system for maintnerd
  to subscribe to.


* The **gitmirror** server in
  [cmd/gitmirror/](https://dev.golang.org/dir/build/cmd/gitmirror) mirrors
  Gerrit to GitHub, and also serves a mirror of the Gerrit code to the
  coordinator for builds, so we don't overwhelm Gerrit and blow our quota.


* The Go **gopherbot** bot logic runs on GKE. The code is in
  [cmd/gopherbot](https://dev.golang.org/dir/build/cmd/gopherbot). It
  depends on maintner via the godata package.


* The **developer dashboard** at https://dev.golang.org/ runs on GKE.
  Its code is in [devapp/](https://dev.golang.org/dir/build/devapp/).
  It also depends on maintner via the godata package.


* **cmd/retrybuilds**: a Go client program to delete build results from the
    dashboard


### Adding a Go Builder

If you wish to run a Go builder, please email
[golang-dev@googlegroups.com](mailto:golang-dev@googlegroups.com) first. There
is documentation at https://golang.org/wiki/DashboardBuilders, but depending
on the type of builder, we may want to run it ourselves, after you prepare an
environment description (resulting in a VM image) of it. See the env directory.

