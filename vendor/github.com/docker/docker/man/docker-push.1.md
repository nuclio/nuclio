% DOCKER(1) Docker User Manuals
% Docker Community
% JUNE 2014
# NAME
docker-push - Push an image or a repository to a registry

# SYNOPSIS
**docker push**
[**--help**]
NAME[:TAG] | [REGISTRY_HOST[:REGISTRY_PORT]/]NAME[:TAG]

# DESCRIPTION

Use `docker push` to share your images to the [Docker Hub](https://hub.docker.com)
registry or to a self-hosted one.

Refer to **docker-tag(1)** for more information about valid image and tag names.

Killing the **docker push** process, for example by pressing **CTRL-c** while it
is running in a terminal, terminates the push operation.

Registry credentials are managed by **docker-login(1)**.


# OPTIONS

**--disable-content-trust**
  Skip image verification (default true)

**--help**
  Print usage statement

# EXAMPLES

## Pushing a new image to a registry

First save the new image by finding the container ID (using **docker ps**)
and then committing it to a new image name.  Note that only a-z0-9-_. are
allowed when naming images:

    # docker commit c16378f943fe rhel-httpd

Now, push the image to the registry using the image ID. In this example the
registry is on host named `registry-host` and listening on port `5000`. To do
this, tag the image with the host name or IP address, and the port of the
registry:

    # docker tag rhel-httpd registry-host:5000/myadmin/rhel-httpd
    # docker push registry-host:5000/myadmin/rhel-httpd

Check that this worked by running:

    # docker images

You should see both `rhel-httpd` and `registry-host:5000/myadmin/rhel-httpd`
listed.

# HISTORY
April 2014, Originally compiled by William Henry (whenry at redhat dot com)
based on docker.com source material and internal work.
June 2014, updated by Sven Dowideit <SvenDowideit@home.org.au>
April 2015, updated by Mary Anthony for v2 <mary@docker.com>
June 2015, updated by Sally O'Malley <somalley@redhat.com>
