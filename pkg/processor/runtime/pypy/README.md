# pypy Runtime

## Developing

* You'll need [pypy][pypy] (version 2) installed (doh!).
* We're using [cgo][cgo] to embed the pypy interpreter, so process can't be
  built with `CGO_ENABLED=0`
* You'll also need to setup `pkg-config` and the pypy home directory (see below)
* pypy is not built during normal processor build. You need to set build tag
  `pypy` to make it work
    go build -tags pypy ./cmd/processor


### pkg-config

[pkg-config][pkg] is used find out there right linker flags. Sadly pypy doesn't
come with support for pkg-config so you'll have to add you own. Assuming you're
pypy is installed at `/opt/pypy` the write a file called `pypy.pc`

```
Name: pypy
Description: A fast, compliant alternative implementation of the Python language
Version: 2-5.9
Libs: -lpypy-c
Cflags: -I/opt/pypy/include
```

And either place it in `/usr/share/pkgconfig/` or at any directory under
`PKG_CONFIG_PATH`. In my cases I've copied it to `~/.local/share/pkgconfig` and
have the following in my `~/.zshrc`

```bash
export PKG_CONFIG_PATH=${HOME}/.local/share/pkgconfig
```

### pypy Home
On the pypy docker the pypy home is at `/usr/local` if you've placed pypy
somewhere else set `NUCLIO_PYPY_HOME` environment variable. (e.g. `export
NUCLIO_PYPY_HOME=/opt/pypy`)

[pypy]: https://pypy.org/
[cgo]: https://golang.org/cmd/cgo/
[pkg]: https://www.freedesktop.org/wiki/Software/pkg-config/

## Running

The processor passes Go allocated structures to the C layer. To enable this you
must set `GODEBUG="cgocheck=0"` before running the processor
