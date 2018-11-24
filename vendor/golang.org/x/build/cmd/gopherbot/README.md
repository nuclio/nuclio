[![GoDoc](https://godoc.org/golang.org/x/build/cmd/gopherbot?status.svg)](https://godoc.org/golang.org/x/build/cmd/gopherbot)

# golang.org/x/build/cmd/gopherbot

The gopherbot command runs Go's gopherbot role account on GitHub and Gerrit.

## Development with Docker

```
make docker-image
docker volume create golang-maintner
docker run -v golang-maintner:/.cache/golang-maintner \
    -it --rm gcr.io/go-dashboard-dev/gopherbot --dry-run
```
