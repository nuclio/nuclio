usage: FORCE
	# See Makefile targets.
	exit 1

FORCE:

upload: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --gzip --file=run-builder-darwin-10_10.sh -cacheable=false -public=true go-builder-data/run-builder-darwin-10_10.gz

