// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package owners

import (
	"golang.org/x/build/internal/gophers"
)

func gh(githubUsername string) Owner {
	p := gophers.GetPerson("@" + githubUsername)
	if p == nil {
		panic(githubUsername + " not found")
	}
	if len(p.Emails) == 0 {
		panic("person with GitHub username " + githubUsername + " must have at least one email")
	}
	return Owner{GitHubUsername: githubUsername, GerritEmail: p.Emails[0]}
}

var (
	adonovan     = gh("alandonovan")
	agl          = gh("agl")
	agnivade     = gh("agnivade")
	alexbrainman = gh("alexbrainman")
	andybons     = gh("andybons")
	austin       = gh("aclements")
	bcmills      = gh("bcmills")
	bradfitz     = gh("bradfitz")
	cbro         = gh("broady")
	cherryyz     = gh("cherrymui")
	dmitshur     = gh("dmitshur")
	danderson    = gh("danderson")
	drchase      = gh("dr2chase")
	dvyukov      = gh("dvyukov")
	eliasnaur    = gh("eliasnaur")
	filippo      = gh("FiloSottile")
	gri          = gh("griesemer")
	hanwen       = gh("hanwen")
	hyangah      = gh("hyangah")
	iant         = gh("ianlancetaylor")
	jbd          = gh("rakyll")
	joetsai      = gh("dsnet")
	josharian    = gh("josharian")
	kardianos    = gh("kardianos")
	kevinburke   = gh("kevinburke")
	khr          = gh("randall77")
	martisch     = gh("martisch")
	matloob      = gh("matloob")
	mdempsky     = gh("mdempsky")
	mdlayher     = gh("mdlayher")
	mikioh       = gh("mikioh")
	minux        = gh("minux")
	mpvl         = gh("mpvl")
	mvdan        = gh("mvdan")
	mwhudson     = gh("mwhudson")
	r            = gh("robpike")
	rlh          = gh("RLH")
	rsc          = gh("rsc")
	sameer       = gh("Sajmani")
	thanm        = gh("thanm")
	tklauser     = gh("tklauser")
	tombergan    = gh("tombergan")
	ysmolsky     = gh("ysmolsky")
)

// entries is a map of <repo name>/<path> to Owner entries.
// It should not be modified at runtime.
var entries = map[string]*Entry{
	"arch/": {
		Primary: []Owner{cherryyz},
	},

	"crypto/": {
		Primary:   []Owner{filippo},
		Secondary: []Owner{agl},
	},
	"crypto/ssh/": {
		Primary:   []Owner{hanwen},
		Secondary: []Owner{filippo},
	},

	"go/": {
		Primary: []Owner{rsc, iant, bradfitz},
	},
	"go/src/archive/tar": {
		Primary: []Owner{joetsai},
	},
	"go/src/archive/zip": {
		Primary:   []Owner{joetsai},
		Secondary: []Owner{bradfitz},
	},
	"go/src/bufio": {
		Primary:   []Owner{},
		Secondary: []Owner{gri, bradfitz, iant},
	},
	"go/src/bytes": {
		Primary:   []Owner{},
		Secondary: []Owner{bradfitz, iant},
	},
	"go/src/cmd/compile/": {
		Primary:   []Owner{khr, gri},
		Secondary: []Owner{josharian, mdempsky, martisch},
	},
	"go/src/cmd/compile/amd64": {
		Primary:   []Owner{khr},
		Secondary: []Owner{josharian, rsc, drchase, cherryyz, martisch},
	},
	"go/src/cmd/compile/arm": {
		Primary:   []Owner{khr},
		Secondary: []Owner{josharian, rsc, drchase, cherryyz},
	},
	"go/src/cmd/compile/arm64": {
		Primary:   []Owner{khr},
		Secondary: []Owner{josharian, rsc, drchase, cherryyz},
	},
	"go/src/cmd/compile/mips": {
		Primary:   []Owner{khr},
		Secondary: []Owner{rsc, drchase, cherryyz},
	},
	"go/src/cmd/compile/mips64": {
		Primary:   []Owner{khr},
		Secondary: []Owner{rsc, drchase, cherryyz},
	},
	"go/src/cmd/compile/ppc64": {
		Primary:   []Owner{khr},
		Secondary: []Owner{rsc, drchase, cherryyz},
	},
	"go/src/cmd/compile/s390x": {
		Primary:   []Owner{khr},
		Secondary: []Owner{rsc, drchase, cherryyz},
	},
	"go/src/cmd/compile/x86": {
		Primary:   []Owner{khr},
		Secondary: []Owner{josharian, rsc, drchase, cherryyz, martisch},
	},
	"go/src/cmd/compile/internal/syntax": {
		Primary:   []Owner{gri},
		Secondary: []Owner{rsc, mdempsky},
	},
	"go/src/cmd/compile/internal/types": {
		Primary:   []Owner{gri},
		Secondary: []Owner{josharian, mdempsky, rsc},
	},
	"go/src/cmd/compile/internal/ssa": {
		Primary:   []Owner{khr},
		Secondary: []Owner{josharian, martisch},
	},
	"go/src/cmd/doc": {
		Primary:   []Owner{r},
		Secondary: []Owner{mvdan},
	},
	"go/src/cmd/go": {
		Primary: []Owner{bcmills, rsc, iant},
	},
	"go/src/cmd/link": {
		Primary:   []Owner{cherryyz, rsc, mdempsky, iant},
		Secondary: []Owner{mwhudson, thanm},
	},
	"go/src/cmd/pprof": {
		Primary: []Owner{hyangah},
	},
	"go/src/cmd/trace": {
		Primary: []Owner{hyangah},
	},
	"go/src/cmd/vet": {
		Primary:   []Owner{adonovan},
		Secondary: []Owner{josharian, mvdan},
	},
	"go/src/compress/bzip2": {
		Primary:   []Owner{joetsai},
		Secondary: []Owner{mdempsky},
	},
	"go/src/compress/flate": {
		Primary:   []Owner{joetsai},
		Secondary: []Owner{mdempsky},
	},
	"go/src/compress/gzip": {
		Primary: []Owner{joetsai},
	},
	"go/src/compress/lzw": {
		Primary:   []Owner{joetsai},
		Secondary: []Owner{mdempsky},
	},
	"go/src/compress/zlib": {
		Primary:   []Owner{joetsai},
		Secondary: []Owner{mdempsky},
	},
	"go/src/container/heap": {
		Primary: []Owner{gri},
	},
	"go/src/container/list": {
		Primary: []Owner{gri},
	},
	"go/src/container/ring": {
		Primary: []Owner{gri},
	},
	"go/src/context": {
		Primary: []Owner{sameer, bradfitz},
	},
	"go/src/crypto": {
		Primary:   []Owner{filippo},
		Secondary: []Owner{rsc},
	},
	"go/src/crypto/tls": {
		Primary:   []Owner{filippo},
		Secondary: []Owner{agl, rsc, kevinburke},
	},
	"go/src/crypto/x509": {
		Primary:   []Owner{filippo},
		Secondary: []Owner{agl, rsc},
	},
	"go/src/database/sql": {
		Primary:   []Owner{bradfitz, kardianos},
		Secondary: []Owner{kevinburke},
	},
	"go/src/debug/dwarf": {
		Primary:   []Owner{austin},
		Secondary: []Owner{thanm},
	},
	"go/src/debug/elf": {
		Primary: []Owner{iant},
	},
	"go/src/debug/pe": {
		Primary: []Owner{alexbrainman},
	},
	"go/src/encoding": {
		Primary: []Owner{rsc},
	},
	"go/src/encoding/asn1": {
		Primary:   []Owner{filippo},
		Secondary: []Owner{agl},
	},
	"go/src/encoding/binary": {
		Primary: []Owner{gri},
	},
	"go/src/encoding/csv": {
		Primary:   []Owner{},
		Secondary: []Owner{joetsai, bradfitz, rsc},
	},
	"go/src/encoding/gob": {
		Primary: []Owner{r},
	},
	"go/src/encoding/json": {
		Primary:   []Owner{rsc},
		Secondary: []Owner{joetsai, bradfitz, mvdan},
	},
	"go/src/encoding/xml": {
		Primary: []Owner{rsc},
	},
	"go/src/expvar": {
		Primary:   []Owner{},
		Secondary: []Owner{bradfitz},
	},
	"go/src/flag": {
		Primary: []Owner{r},
	},
	"go/src/fmt": {
		Primary:   []Owner{r},
		Secondary: []Owner{martisch},
	},
	"go/src/go/ast": {
		Primary:   []Owner{gri},
		Secondary: []Owner{josharian},
	},
	"go/src/go/build": {
		Primary: []Owner{rsc},
	},
	"go/src/go/constant": {
		Primary: []Owner{gri},
	},
	"go/src/go/doc": {
		Primary: []Owner{gri},
	},
	"go/src/go/format": {
		Primary: []Owner{gri},
	},
	"go/src/go/importer": {
		Primary: []Owner{gri, adonovan},
	},
	"go/src/go/internal/gccgoimporter": {
		Primary: []Owner{gri, iant},
	},
	"go/src/go/internal/gcimporter": {
		Primary: []Owner{gri},
	},
	"go/src/go/parser": {
		Primary: []Owner{gri},
	},
	"go/src/go/printer": {
		Primary: []Owner{gri},
	},
	"go/src/go/scanner": {
		Primary: []Owner{gri},
	},
	"go/src/go/token": {
		Primary: []Owner{gri},
	},
	"go/src/go/types": {
		Primary:   []Owner{gri},
		Secondary: []Owner{adonovan},
	},
	"go/src/image": {
		Primary:   []Owner{},
		Secondary: []Owner{r},
	},
	"go/src/index/suffixarray": {
		Primary: []Owner{gri},
	},
	"go/src/internal/bytealg": {
		Primary:   []Owner{khr},
		Secondary: []Owner{josharian},
	},
	"go/src/internal/cpu": {
		Primary: []Owner{khr, martisch},
	},
	"go/src/internal/singleflight": {
		Primary: []Owner{bradfitz, iant},
	},
	"go/src/internal/syscall/unix": {
		Primary:   []Owner{iant, bradfitz},
		Secondary: []Owner{tklauser},
	},
	"go/src/internal/syscall/windows": {
		Primary:   []Owner{alexbrainman},
		Secondary: []Owner{bradfitz},
	},
	"go/src/internal/syscall/windows/registry": {
		Primary:   []Owner{alexbrainman},
		Secondary: []Owner{bradfitz},
	},
	"go/src/internal/syscall/windows/sysdll": {
		Primary:   []Owner{alexbrainman},
		Secondary: []Owner{bradfitz},
	},
	"go/src/internal/testenv": {
		Primary:   []Owner{bradfitz, iant},
		Secondary: []Owner{josharian},
	},
	"go/src/log": {
		Primary: []Owner{r},
	},
	"go/src/math": {
		Primary: []Owner{gri, rsc},
	},
	"go/src/math/big": {
		Primary: []Owner{gri},
	},
	"go/src/math/bits": {
		Primary:   []Owner{gri},
		Secondary: []Owner{khr, josharian},
	},
	"go/src/math/rand": {
		Primary:   []Owner{gri, rsc},
		Secondary: []Owner{josharian},
	},
	"go/src/mime": {
		Primary: []Owner{bradfitz},
	},
	"go/src/mime/multipart": {
		Primary: []Owner{bradfitz, minux},
	},
	"go/src/mime/quotedprintable": {
		Primary: []Owner{bradfitz, minux},
	},
	"go/src/net": {
		Primary:   []Owner{mikioh},
		Secondary: []Owner{bradfitz, iant},
	},
	"go/src/net/http": {
		Primary:   []Owner{bradfitz},
		Secondary: []Owner{rsc},
	},
	"go/src/net/http/cgi": {
		Primary: []Owner{bradfitz},
	},
	"go/src/net/http/cookiejar": {
		Primary: []Owner{},
	},
	"go/src/net/http/httptest": {
		Primary: []Owner{bradfitz},
	},
	"go/src/net/http/httptrace": {
		Primary: []Owner{bradfitz},
	},
	"go/src/net/http/httputil": {
		Primary: []Owner{bradfitz},
	},
	"go/src/net/http/internal": {
		Primary: []Owner{bradfitz},
	},
	"go/src/net/http/pprof": {
		Primary: []Owner{rsc},
	},
	"go/src/net/internal/socktest": {
		Primary: []Owner{mikioh},
	},
	"go/src/net/mail": {
		Primary:   []Owner{},
		Secondary: []Owner{bradfitz},
	},
	"go/src/net/rpc": {
		Primary: []Owner{r},
	},
	"go/src/net/rpc/jsonrpc": {
		Primary: []Owner{r},
	},
	"go/src/net/smtp": {
		Primary:   []Owner{},
		Secondary: []Owner{bradfitz},
	},
	"go/src/net/textproto": {
		Primary: []Owner{bradfitz, rsc},
	},
	"go/src/net/url": {
		Primary: []Owner{rsc, bradfitz},
	},
	"go/src/os": {
		Primary: []Owner{rsc, r, iant, bradfitz, gri},
	},
	"go/src/os/exec": {
		Primary: []Owner{bradfitz, iant},
	},
	"go/src/os/signal": {
		Primary: []Owner{iant},
	},
	"go/src/os/user": {
		Primary:   []Owner{bradfitz},
		Secondary: []Owner{kevinburke},
	},
	"go/src/path": {
		Primary: []Owner{r, rsc},
	},
	"go/src/path/filepath": {
		Primary: []Owner{r, rsc},
	},
	"go/src/plugin": {
		Primary:   []Owner{iant},
		Secondary: []Owner{cherryyz},
	},
	"go/src/reflect": {
		Primary: []Owner{rsc, iant},
	},
	"go/src/regexp": {
		Primary:   []Owner{rsc},
		Secondary: []Owner{matloob},
	},
	"go/src/regexp/syntax": {
		Primary: []Owner{rsc},
	},
	"go/src/runtime": {
		Primary:   []Owner{austin, rsc, rlh, khr},
		Secondary: []Owner{iant, dvyukov, josharian, martisch},
	},
	"go/src/runtime/cgo": {
		Primary: []Owner{iant},
	},
	"go/src/runtime/internal/atomic": {
		Primary: []Owner{austin, khr},
	},
	"go/src/runtime/internal/sys": {
		Primary: []Owner{austin, khr},
	},
	"go/src/runtime/pprof": {
		Primary: []Owner{hyangah},
	},
	"go/src/runtime/pprof/internal/protopprof": {
		Primary:   []Owner{},
		Secondary: []Owner{matloob},
	},
	"go/src/runtime/race": {
		Primary: []Owner{dvyukov},
	},
	"go/src/runtime/trace": {
		Primary: []Owner{hyangah, dvyukov},
	},
	"go/src/sort": {
		Primary: []Owner{rsc, gri, iant, bradfitz},
	},
	"go/src/strconv": {
		Primary: []Owner{rsc, gri, iant, bradfitz},
	},
	"go/src/sync": {
		Primary: []Owner{rsc, iant, dvyukov, austin, rlh},
	},
	"go/src/sync/atomic": {
		Primary: []Owner{rsc, iant, dvyukov, austin, rlh},
	},
	"go/src/syscall": {
		Primary:   []Owner{iant, bradfitz},
		Secondary: []Owner{tklauser},
	},
	"go/src/testing": {
		Primary:   []Owner{},
		Secondary: []Owner{mpvl, josharian},
	},
	"go/src/testing/quick": {
		Primary:   []Owner{},
		Secondary: []Owner{agl},
	},
	"go/src/text/scanner": {
		Primary: []Owner{gri},
	},
	"go/src/text/tabwriter": {
		Primary: []Owner{gri},
	},
	"go/src/text/template": {
		Primary:   []Owner{r},
		Secondary: []Owner{mvdan},
	},
	"go/src/text/template/parse": {
		Primary:   []Owner{r},
		Secondary: []Owner{mvdan},
	},
	"go/src/time": {
		Primary: []Owner{rsc},
	},
	"go/src/unicode": {
		Primary:   []Owner{r},
		Secondary: []Owner{mpvl},
	},
	"go/src/unicode/utf16": {
		Primary: []Owner{r},
	},
	"go/src/unicode/utf8": {
		Primary: []Owner{r},
	},
	"go/src/unsafe": {
		Primary: []Owner{gri},
	},

	"build/maintner/cmd/maintserve": {
		Primary:   []Owner{dmitshur},
		Secondary: []Owner{andybons},
	},

	"gofrontend/": {
		Primary:   []Owner{iant},
		Secondary: []Owner{thanm},
	},

	"gollvm/": {
		Primary:   []Owner{thanm},
		Secondary: []Owner{cherryyz},
	},

	"mobile/": {
		Primary:   []Owner{eliasnaur},
		Secondary: []Owner{hyangah},
	},

	"net/": {
		Primary:   []Owner{mikioh},
		Secondary: []Owner{bradfitz, iant},
	},
	"net/bpf": {
		Primary: []Owner{danderson, mdlayher},
	},
	"net/http": {
		Primary: []Owner{bradfitz},
	},
	"net/http2": {
		Primary: []Owner{bradfitz, tombergan},
	},
	"net/icmp": {
		Primary: []Owner{mikioh},
	},
	"net/ipv4": {
		Primary: []Owner{mikioh, iant},
	},
	"net/ipv6": {
		Primary: []Owner{mikioh, iant},
	},

	"review/": {
		Secondary: []Owner{josharian, kevinburke},
	},

	"sync/": {
		Primary: []Owner{bcmills},
	},

	"sys/unix": {
		Primary: []Owner{iant, bradfitz, tklauser},
	},
	"sys/windows": {
		Primary: []Owner{alexbrainman, bradfitz},
	},

	"text/": {
		Primary: []Owner{mpvl},
	},

	"tools/cmd/compilebench": {
		Secondary: []Owner{josharian},
	},
	"tools/cmd/bundle": {
		Primary: []Owner{adonovan},
	},
	"tools/cmd/godoc": {
		Primary:   []Owner{andybons},
		Secondary: []Owner{agnivade, bradfitz, gri, ysmolsky, kevinburke},
	},
	"tools/cmd/goimports": {
		Primary:   []Owner{bradfitz},
		Secondary: []Owner{josharian},
	},
	"tools/cmd/stringer": {
		Secondary: []Owner{mvdan},
	},
	"tools/cmd/toolstash": {
		Secondary: []Owner{josharian},
	},
	"playground": {
		Secondary: []Owner{ysmolsky},
	},
}
