[![GoDoc](https://godoc.org/golang.org/x/build/cmd/makemac?status.svg)](https://godoc.org/golang.org/x/build/cmd/makemac)

# golang.org/x/build/cmd/makemac

The makemac command starts OS X VMs for the builders.

## Deploying `makemac`

```
* On Linux,
  $ go install golang.org/x/build/cmd/makemac
  $ scp -i ~/.ssh/id_ed25519_golang1 $GOPATH/bin/makemac gopher@macstadiumd.golang.org:makemac.new
  $ ssh -i ~/.ssh/id_ed25519_golang1 gopher@macstadiumd.golang.org 'cp makemac makemac.old; install makemac.new makemac'
  $ ssh -i ~/.ssh/id_ed25519_golang1 gopher@macstadiumd.golang.org

On that host,
  * sudo systemctl restart makemac
  $ sudo journalctl -f -u makemac     # watch it
```

## Updating `makemac.service`

```
* On Linux,
  $ scp -i ~/.ssh/id_ed25519_golang1 cmd/makemac/makemac.service gopher@macstadiumd.golang.org:makemac.service

On that host,
  $ sudo mv makemac.service /etc/systemd/system/makemac.service
  $ sudo systemctl daemon-reload
  $ sudo systemctl restart makemac
  $ sudo journalctl -f -u makemac     # watch it
```
