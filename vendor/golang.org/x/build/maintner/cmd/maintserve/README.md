[![GoDoc](https://godoc.org/golang.org/x/build/maintner/cmd/maintserve?status.svg)](https://godoc.org/golang.org/x/build/maintner/cmd/maintserve)

# golang.org/x/build/maintner/cmd/maintserve

maintserve is a program that serves Go issues over HTTP, so they can be
viewed in a browser. It uses x/build/maintner/godata as its backing
source of data.

It statically embeds all the resources it uses, so it's possible to use
it when offline. During that time, the corpus will not be able to update,
and GitHub user profile pictures won't load.
