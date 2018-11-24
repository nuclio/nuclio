# vcs-test

We run a version control server for testing at `vcs-test.golang.org`.

## Machine initialization

The machine should just run. You should not need these instructions very often.
In particular you do not need them just to make a change to `vcweb`.
Skip ahead to the next section.

The VM runs in the builder project “symbolic-datum-552” in zone `us-central1-a`,
where it has a reserved static IP address named `vcs-test`.

To destroy the current VM (if any) and rebuild a fresh one in its place, run:

	./rebuild-server.sh && ./rebuild-systemd.sh && ./redeploy-vcweb.sh

You should not need to do this unless you have changed rebuild-server.sh and want to test it.

To delete the VM's current systemd configuration for `vcweb` and upload the configuration
from the local directory (specifically, `vcweb.service` and `vcweb*.socket`), run:

	./rebuild-systemd.sh && ./redeploy-vcweb.sh

You should not need to do this unless you have changed the systemd configuration files.

## vcweb

The Go program that runs the actual server is in the subdirectory `vcweb`.
For local development:

	go build -o vcweb.exe ./vcweb && ./vcweb.exe

It maintains files in `/tmp/vcweb` and serves localhost:8088.

Once you are happy with local testing, deploy to the VM by running `./redeploy-vcweb.sh`.

## Repositories

The server can serve Bazaar, Fossil, Git, Mercurial, and Subversion repositories.
The root of each repository is `http://vcs-test.golang.org/VCS/REPONAME`,
where `VCS` is the version control system's command name (`bzr` for Bazaar, and so on),
and `REPONAME` is the repository name.

To serve a particular repository, the server downloads
`gs://vcs-test/VCS/REPONAME.zip` from Google Cloud Storage and unzips it
into an empty directory.
The result should be a valid repository directory for the given version control system.
If the needed format of the zip file is unclear, download and inspect `gs://vcs-test/VCS/hello.zip`
from `https://vcs-test.storage.googleapis.com/VCS/hello.zip`.

Stale data may be served for up to five minutes after a zip file is updated in the
Google Cloud Storage bucket. To force a rescan of Google Cloud Storage,
fetch `http://vcs-test.golang.org/VCS/REPONAME?vcweb-force-reload=1`.

## Static files

The URL space `http://vcs-test.golang.org/go/NAME` is served by static files,
fetched from `gs://vcs-test/go/NAME.zip`.
The main use for static files is to write redirect HTML.
See `gs://vcs-test/go/hello.zip` for examples.
Note that because the server uses `http.DetectContentType` to deduce
the content type from file data, it is not necessary to
name HTML files with a `.html` suffix.

## HTTPS

The server fetches an HTTPS certificate on demand from Let's Encrypt,
using `golang.org/x/crypto/acme/autocert`.
It caches the certificates in `gs://vcs-test-autocert` using
`golang.org/x/build/autocertcache`.

