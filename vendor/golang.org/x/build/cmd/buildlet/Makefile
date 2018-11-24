usage: FORCE
	# See targets in Makefile (e.g. "buildlet.darwin-amd64")
	exit 1

FORCE:

# The 'compile' target is a sanity check target to make sure changes
# compile everywhere, without uploading them to GCS.
# Note that the goal here isn't to build every GOOS/GOARCH combination;
# we only build the versions used by buildlet hosts.
compile:
	GOOS=linux GOARCH=amd64 go install golang.org/x/build/cmd/buildlet
	GOOS=linux GOARCH=arm go install golang.org/x/build/cmd/buildlet
	GOOS=linux GOARCH=arm64 go install golang.org/x/build/cmd/buildlet
	GOOS=linux GOARCH=mips64 go install golang.org/x/build/cmd/buildlet
	GOOS=linux GOARCH=mips64le go install golang.org/x/build/cmd/buildlet
	GOOS=linux GOARCH=ppc64 go install golang.org/x/build/cmd/buildlet
	GOOS=linux GOARCH=ppc64le go install golang.org/x/build/cmd/buildlet
	GOOS=linux GOARCH=s390x go install golang.org/x/build/cmd/buildlet
	GOOS=darwin GOARCH=amd64 go install golang.org/x/build/cmd/buildlet
	GOOS=windows GOARCH=amd64 go install golang.org/x/build/cmd/buildlet
	GOOS=windows GOARCH=386 go install golang.org/x/build/cmd/buildlet
	GOOS=freebsd GOARCH=amd64 go install golang.org/x/build/cmd/buildlet
	GOOS=netbsd GOARCH=amd64 go install golang.org/x/build/cmd/buildlet
	GOOS=openbsd GOARCH=amd64 go install golang.org/x/build/cmd/buildlet
	GOOS=openbsd GOARCH=386 go install golang.org/x/build/cmd/buildlet
	GOOS=plan9 GOARCH=386 go install golang.org/x/build/cmd/buildlet
	GOOS=solaris GOARCH=amd64 go install golang.org/x/build/cmd/buildlet

# buildlet.all is compiles & uploads all targets.
buildlet.all: FORCE buildlet.darwin-amd64 buildlet.darwin-amd64.gz buildlet.freebsd-amd64 buildlet.linux-amd64 buildlet.netbsd-amd64 buildlet.openbsd-amd64 buildlet.openbsd-386 buildlet.plan9-386 buildlet.windows-amd64 buildlet.linux-arm buildlet.linux-arm-arm5 buildlet.linux-arm64 buildlet.linux-mips buildlet.linux-mipsle buildlet.linux-mips64 buildlet.linux-mips64le buildlet.linux-ppc64 buildlet.linux-ppc64le buildlet.linux-s390x buildlet.solaris-amd64
	echo "done"

buildlet.darwin-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.freebsd-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-amd64-static: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false --static go-builder-data/$@

buildlet.netbsd-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.netbsd-386: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.openbsd-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.openbsd-386: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.plan9-386: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.windows-amd64: FORCE buildlet_windows.go
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.windows-386: FORCE buildlet_windows.go
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-arm: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-arm-arm5: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=linux-arm --installsuffix=arm5 --extraenv=GOARM=5 --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-arm64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-mips: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-mipsle: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-mips64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-mips64le: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-ppc64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-ppc64le: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.linux-s390x: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

buildlet.solaris-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false go-builder-data/$@

dev-buildlet.linux-arm: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false dev-go-builder-data/buildlet.linux-arm

dev-buildlet.linux-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false dev-go-builder-data/buildlet.linux-amd64

dev-buildlet.windows-amd64: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public --cacheable=false dev-go-builder-data/buildlet.windows-amd64

dev-buildlet.plan9-386: FORCE
	go install golang.org/x/build/cmd/upload
	upload --verbose --osarch=$@ --file=go:golang.org/x/build/cmd/buildlet --public dev-go-builder-data/buildlet.plan9-386
