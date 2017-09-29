# go-kinesis
[![Build Status](https://travis-ci.org/sendgridlabs/go-kinesis.png?branch=master)](https://travis-ci.org/sendgridlabs/go-kinesis)

GO-lang library for AWS Kinesis API.

## Documentation

* [Core API](http://godoc.org/github.com/sendgridlabs/go-kinesis)
* [Batch Producer API](http://godoc.org/github.com/sendgridlabs/go-kinesis/batchproducer)

## Example

Example you can find in folder `examples`.

## Command line interface

You can find a tool for interacting with kinesis from the command line in folder `kinesis-cli`.

## Testing

### Local Kinesis Server

The tests require a local Kinesis server such as [Kinesalite](https://github.com/mhart/kinesalite)
to be running and reachable at `http://127.0.0.1:4567`.

To make the tests complete faster, you might want to have Kinesalite perform stream creation and
deletion faster than the default of 500ms, like so:

    kinesalite --createStreamMs 5 --deleteStreamMs 5 &

The `&` runs Kinesalite in the background, which is probably what you want.

### go test

Some of the tests are marked as safe to be run in parallel, so to speed up test execution you might
want to run `go test` with [the `-parallel n` flag](https://golang.org/cmd/go/#hdr-Description_of_testing_flags).
