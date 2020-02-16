set -e

cd $GOPATH/src/github.com/v3io/v3io-go

echo Installing impi
go get -u github.com/pavius/impi/cmd/impi

echo Linting imports with impi
$GOPATH/bin/impi \
    --local github.com/v3io/v3io-go \
    --scheme stdLocalThirdParty \
    --skip=pkg/dataplane/schemas/node/common \
    ./pkg/...

echo Getting all packages
go get ./...

echo Installing gometalinter
go get -u gopkg.in/alecthomas/gometalinter.v2
$GOPATH/bin/gometalinter.v2 --install

echo Linting with gometalinter
$GOPATH/bin/gometalinter.v2 \
    --deadline=300s \
    --disable-all \
    --enable-gc \
    --enable=deadcode \
    --enable=goconst \
    --enable=gofmt \
    --enable=golint \
    --enable=gosimple \
    --enable=ineffassign \
    --enable=interfacer \
    --enable=misspell \
    --enable=staticcheck \
    --enable=unconvert \
    --enable=varcheck \
    --enable=vet \
    --enable=vetshadow \
    --enable=errcheck \
    --exclude="_test.go" \
    --exclude="comment on" \
    --exclude="error should be the last" \
    --exclude="should have comment" \
    --skip=pkg/platform/kube/apis \
    --skip=pkg/platform/kube/client \
    --skip=pkg/dataplane/schemas/node/common \
    ./pkg/...
