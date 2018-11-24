// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package dashboard contains shared configuration and logic used by various
// pieces of the Go continuous build system.
package dashboard

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/build/buildenv"
)

// Builders are the different build configurations.
// The keys are like "darwin-amd64" or "linux-386-387".
// This map should not be modified by other packages.
// Initialization happens below, via calls to addBuilder.
var Builders = map[string]*BuildConfig{}

// Hosts contains the names and configs of all the types of
// buildlets. They can be VMs, containers, or dedicated machines.
var Hosts = map[string]*HostConfig{
	"host-linux-jessie": &HostConfig{
		Notes:           "Debian Jessie, our standard Linux container image.",
		ContainerImage:  "linux-x86-jessie:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
		SSHUsername:     "root",
	},
	"host-linux-stretch": &HostConfig{
		Notes:           "Debian Stretch",
		ContainerImage:  "linux-x86-stretch:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
		SSHUsername:     "root",
	},
	"host-linux-armhf-cross": &HostConfig{
		Notes:           "Debian Jessie with armhf cross-compiler, built from env/crosscompile/linux-armhf-jessie",
		ContainerImage:  "linux-armhf-jessie:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
	},
	"host-linux-armel-cross": &HostConfig{
		Notes:           "Debian Jessie with armel cross-compiler, from env/crosscompile/linux-armel-stretch",
		ContainerImage:  "linux-armel-stretch:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
	},
	"host-linux-amd64-localdev": &HostConfig{
		IsReverse:   true,
		ExpectNum:   0,
		Notes:       "for localhost development of buildlets/gomote/coordinator only",
		SSHUsername: os.Getenv("USER"),
	},
	"host-nacl-arm-davecheney": &HostConfig{
		IsReverse:   true,
		ExpectNum:   1,
		Notes:       "Raspberry Pi 3",
		OwnerGithub: "davecheney",
	},
	"host-nacl-kube": &HostConfig{
		Notes:           "Container with Native Client binaries.",
		ContainerImage:  "linux-x86-nacl:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
	},
	"host-js-wasm": &HostConfig{
		Notes:           "Container with node.js for testing js/wasm.",
		ContainerImage:  "js-wasm:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
	},
	"host-s390x-cross-kube": &HostConfig{
		Notes:           "Container with s390x cross-compiler.",
		ContainerImage:  "linux-s390x-stretch:latest",
		buildletURLTmpl: "https://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
	},
	"host-linux-x86-alpine": &HostConfig{
		Notes:           "Alpine container",
		ContainerImage:  "linux-x86-alpine:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64-static",
		env:             []string{"GOROOT_BOOTSTRAP=/usr/lib/go"},
	},
	"host-linux-clang": &HostConfig{
		Notes:           "Container with clang.",
		ContainerImage:  "linux-x86-clang:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
	},
	"host-linux-sid": &HostConfig{
		Notes:           "Debian sid, updated occasionally.",
		ContainerImage:  "linux-x86-sid:latest",
		buildletURLTmpl: "http://storage.googleapis.com/$BUCKET/buildlet.linux-amd64",
		env:             []string{"GOROOT_BOOTSTRAP=/go1.4"},
	},
	"host-linux-arm-scaleway": &HostConfig{
		IsReverse:       true,
		HermeticReverse: true,
		ExpectNum:       50,
		env:             []string{"GOROOT_BOOTSTRAP=/usr/local/go"},
		ReverseAliases:  []string{"linux-arm", "linux-arm-arm5"},
		SSHUsername:     "root",
	},
	"host-linux-arm5spacemonkey": &HostConfig{
		IsReverse:      true,
		ExpectNum:      3,
		env:            []string{"GOROOT_BOOTSTRAP=/usr/local/go"},
		ReverseAliases: []string{"linux-arm-arm5spacemonkey"},
		OwnerGithub:    "zeebo",
	},
	"host-openbsd-amd64-60": &HostConfig{
		VMImage:            "openbsd-amd64-60",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.openbsd-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-openbsd-amd64-60.tar.gz",
		Notes:              "OpenBSD 6.0; GCE VM is built from script in build/env/openbsd-amd64",
		SSHUsername:        "gopher",
	},
	"host-openbsd-386-60": &HostConfig{
		VMImage:            "openbsd-386-60",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.openbsd-386",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-openbsd-386-60.tar.gz",
		Notes:              "OpenBSD 6.0; GCE VM is built from script in build/env/openbsd-386",
		SSHUsername:        "gopher",
	},
	"host-openbsd-amd64-62": &HostConfig{
		VMImage:            "openbsd-amd64-62",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.openbsd-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-openbsd-amd64-60.tar.gz",
		Notes:              "OpenBSD 6.2; GCE VM is built from script in build/env/openbsd-amd64",
		SSHUsername:        "gopher",
	},
	"host-openbsd-386-62": &HostConfig{
		VMImage:            "openbsd-386-62-a",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.openbsd-386",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-openbsd-386-60.tar.gz",
		Notes:              "OpenBSD 6.2; GCE VM is built from script in build/env/openbsd-386",
		SSHUsername:        "gopher",
	},
	"host-openbsd-amd64-64": &HostConfig{
		VMImage:            "openbsd-amd64-64-v2",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.openbsd-amd64-64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-openbsd-amd64-64.tar.gz",
		Notes:              "OpenBSD 6.4 with hw.smt=1; GCE VM is built from script in build/env/openbsd-amd64",
		SSHUsername:        "gopher",
	},
	"host-openbsd-386-64": &HostConfig{
		VMImage:            "openbsd-386-64",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.openbsd-386-64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-openbsd-386-64.tar.gz",
		Notes:              "OpenBSD 6.4; GCE VM is built from script in build/env/openbsd-386",
		SSHUsername:        "gopher",
	},
	"host-freebsd-93-gce": &HostConfig{
		VMImage:            "freebsd-amd64-gce93",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.freebsd-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-freebsd-amd64.tar.gz",
		SSHUsername:        "gopher",
	},
	"host-freebsd-10_3": &HostConfig{
		VMImage:            "freebsd-amd64-103-b",
		Notes:              "FreeBSD 10.3; GCE VM is built from script in build/env/freebsd-amd64",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.freebsd-amd64", // TODO(bradfitz): why was this http instead of https?
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-freebsd-amd64.tar.gz",
		env:                []string{"CC=clang"},
		SSHUsername:        "gopher",
	},
	"host-freebsd-10_4": &HostConfig{
		VMImage:            "freebsd-amd64-104",
		Notes:              "FreeBSD 10.4; GCE VM is built from script in build/env/freebsd-amd64",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.freebsd-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-freebsd-amd64.tar.gz",
		SSHUsername:        "gopher",
	},
	"host-freebsd-11_1": &HostConfig{
		VMImage:            "freebsd-amd64-111-b",
		Notes:              "FreeBSD 11.1; GCE VM is built from script in build/env/freebsd-amd64",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.freebsd-amd64", // TODO(bradfitz): why was this http instead of https?
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-freebsd-amd64.tar.gz",
		env:                []string{"CC=clang"},
		SSHUsername:        "gopher",
	},
	"host-freebsd-11_2": &HostConfig{
		VMImage:            "freebsd-amd64-112",
		Notes:              "FreeBSD 11.2; GCE VM is built from script in build/env/freebsd-amd64",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.freebsd-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-freebsd-amd64.tar.gz",
		SSHUsername:        "gopher",
	},
	"host-freebsd-12_0": &HostConfig{
		VMImage:            "freebsd-amd64-120a8",
		Notes:              "FreeBSD 12.0alpha8; GCE VM is built from script in build/env/freebsd-amd64",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "https://storage.googleapis.com/$BUCKET/buildlet.freebsd-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-freebsd-amd64.tar.gz",
		SSHUsername:        "gopher",
	},
	"host-netbsd-amd64-8_0": &HostConfig{
		VMImage:            "netbsd-amd64-8-0-2018q1",
		Notes:              "NetBSD 8.0RC1; GCE VM is built from script in build/env/netbsd-amd64",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.netbsd-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-netbsd-amd64-2da6b33.tar.gz",
		SSHUsername:        "root",
	},
	// Note: the netbsd-386 host hangs during the ../test phase of all.bash,
	// so we don't use this for now. (See the netbsd-386-8 BuildConfig below.)
	"host-netbsd-386-8_0": &HostConfig{
		VMImage:            "netbsd-386-8-0-2018q1",
		Notes:              "NetBSD 8.0RC1; GCE VM is built from script in build/env/netbsd-386",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.netbsd-386",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-netbsd-386-0b3b511.tar.gz",
		SSHUsername:        "root",
	},
	"host-dragonfly-amd64-tdfbsd": &HostConfig{
		IsReverse:      true,
		ExpectNum:      1,
		env:            []string{"GOROOT_BOOTSTRAP=/usr/local/go"},
		ReverseAliases: []string{"dragonfly-amd64"},
		OwnerGithub:    "tdfbsd",
	},
	"host-freebsd-arm-paulzhol": &HostConfig{
		IsReverse:      true,
		ExpectNum:      1,
		Notes:          "Cubiboard2 1Gb RAM dual-core Cortex-A7 (Allwinner A20), FreeBSD 11.1-RELEASE",
		env:            []string{"GOROOT_BOOTSTRAP=/usr/home/paulzhol/go1.4"},
		ReverseAliases: []string{"freebsd-arm-paulzhol"},
		OwnerGithub:    "paulzhol",
	},
	"host-plan9-arm-0intro": &HostConfig{
		IsReverse:   true,
		ExpectNum:   1,
		Notes:       "Raspberry Pi 3 Model B, Plan 9 from Bell Labs",
		OwnerGithub: "0intro",
	},
	"host-plan9-amd64-0intro": &HostConfig{
		IsReverse:   true,
		ExpectNum:   1,
		OwnerGithub: "0intro",
	},
	"host-plan9-386-gce": &HostConfig{
		VMImage:            "plan9-386-v6",
		Notes:              "Plan 9 from 0intro; GCE VM is built from script in build/env/plan9-386",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.plan9-386",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/gobootstrap-plan9-386.tar.gz",

		// We *were* using n1-standard-1 because Plan 9 can only
		// reliably use a single CPU. Using 2 or 4 and we see
		// test failures. See:
		//    https://golang.org/issue/8393
		//    https://golang.org/issue/9491
		// n1-standard-1 has 3.6 GB of memory which WAS (see below)
		// overkill (userspace probably only sees 2GB anyway),
		// but it's the cheapest option. And plenty to keep
		// our ~250 MB of inputs+outputs in its ramfs.
		//
		// But the docs says "For the n1 series of machine
		// types, a virtual CPU is implemented as a single
		// hyperthread on a 2.6GHz Intel Sandy Bridge Xeon or
		// Intel Ivy Bridge Xeon (or newer) processor. This
		// means that the n1-standard-2 machine type will see
		// a whole physical core."
		//
		// ... so we used n1-highcpu-2 (1.80 RAM, still
		// plenty), just so we can get 1 whole core for the
		// single-core Plan 9. It will see 2 virtual cores and
		// only use 1, but we hope that 1 will be more powerful
		// and we'll stop timing out on tests.
		machineType: "n1-highcpu-4",
		env:         []string{"GO_TEST_TIMEOUT_SCALE=2"},
	},
	"host-windows-amd64-2008": &HostConfig{
		VMImage:            "windows-amd64-server-2008r2-v7",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.windows-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-windows-amd64.tar.gz",
		SSHUsername:        "gopher",
	},
	"host-windows-amd64-2012": &HostConfig{
		VMImage:            "windows-amd64-server-2012r2-v7",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.windows-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-windows-amd64.tar.gz",
		SSHUsername:        "gopher",
	},
	"host-windows-amd64-2016": &HostConfig{
		VMImage:            "windows-amd64-server-2016-v7",
		machineType:        "n1-highcpu-4",
		buildletURLTmpl:    "http://storage.googleapis.com/$BUCKET/buildlet.windows-amd64",
		goBootstrapURLTmpl: "https://storage.googleapis.com/$BUCKET/go1.4-windows-amd64.tar.gz",
		SSHUsername:        "gopher",
	},
	"host-windows-arm-iotcore": &HostConfig{
		IsReverse:   true,
		ExpectNum:   1,
		OwnerGithub: "jordanrh1",
		env:         []string{"GOROOT_BOOTSTRAP=C:\\Data\\Go"},
	},
	"host-darwin-10_8": &HostConfig{
		IsReverse: true,
		ExpectNum: 0,
		Notes:     "MacStadium OS X 10.8 VM under VMWare ESXi",
		env: []string{
			"GOROOT_BOOTSTRAP=/Users/gopher/go1.4",
		},
		ReverseAliases:  []string{"darwin-amd64-10_8"},
		SSHUsername:     "gopher",
		HermeticReverse: false, // TODO: make it so, like 10.12
	},
	"host-darwin-10_10": &HostConfig{
		IsReverse: true,
		ExpectNum: 1,
		Notes:     "MacStadium OS X 10.10 VM under VMWare ESXi",
		env: []string{
			"GOROOT_BOOTSTRAP=/Users/gopher/go1.4",
		},
		ReverseAliases:  []string{"darwin-amd64-10_10"},
		SSHUsername:     "gopher",
		HermeticReverse: false, // TODO: make it so, like 10.12
	},
	"host-darwin-10_11": &HostConfig{
		IsReverse: true,
		ExpectNum: 17,
		Notes:     "MacStadium OS X 10.11 VM under VMWare ESXi",
		env: []string{
			"GOROOT_BOOTSTRAP=/Users/gopher/go1.4",
		},
		ReverseAliases:  []string{"darwin-amd64-10_11"},
		SSHUsername:     "gopher",
		HermeticReverse: false, // TODO: make it so, like 10.12
	},
	"host-darwin-10_12": &HostConfig{
		IsReverse: true,
		ExpectNum: 2,
		Notes:     "MacStadium OS X 10.12 VM under VMWare ESXi",
		env: []string{
			"GOROOT_BOOTSTRAP=/Users/gopher/go1.4",
		},
		ReverseAliases:  []string{"darwin-amd64-10_12"},
		SSHUsername:     "gopher",
		HermeticReverse: true, // we destroy the VM when done & let cmd/makemac recreate
	},
	"host-linux-s390x": &HostConfig{
		Notes:          "run by IBM",
		OwnerGithub:    "mundaym",
		IsReverse:      true,
		env:            []string{"GOROOT_BOOTSTRAP=/var/buildlet/go-linux-s390x-bootstrap"},
		ReverseAliases: []string{"linux-s390x-ibm"},
	},
	"host-linux-ppc64-osu": &HostConfig{
		Notes:           "Debian jessie; run by Go team on osuosl.org",
		IsReverse:       true,
		ExpectNum:       5,
		env:             []string{"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap"},
		ReverseAliases:  []string{"linux-ppc64-buildlet"},
		SSHUsername:     "debian",
		HermeticReverse: false, // TODO: use rundockerbuildlet like arm64
	},
	"host-linux-ppc64le-osu": &HostConfig{
		Notes:           "Debian jessie; run by Go team on osuosl.org",
		IsReverse:       true,
		ExpectNum:       5,
		env:             []string{"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap"},
		ReverseAliases:  []string{"linux-ppc64le-buildlet"},
		SSHUsername:     "debian",
		HermeticReverse: false, // TODO: use rundockerbuildlet like arm64
	},
	"host-linux-arm64-linaro": &HostConfig{
		Notes:           "Ubuntu xenial; run by Go team, from linaro",
		IsReverse:       true,
		HermeticReverse: true,
		ExpectNum:       5,
		env:             []string{"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap"},
		ReverseAliases:  []string{"linux-arm64-buildlet"},
		SSHUsername:     "root",
	},
	"host-linux-arm64-packet": &HostConfig{
		Notes:           "On 96 core packet.net host (Xenial) in Docker containers (Jessie); run by Go team. See x/build/env/linux-arm64/packet",
		IsReverse:       true,
		HermeticReverse: true,
		ExpectNum:       20,
		env:             []string{"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap"},
		SSHUsername:     "root",
	},
	"host-solaris-amd64": &HostConfig{
		Notes:          "run by Go team on Joyent, on a SmartOS 'infrastructure container'",
		IsReverse:      true,
		ExpectNum:      5,
		env:            []string{"GOROOT_BOOTSTRAP=/root/go-solaris-amd64-bootstrap"},
		ReverseAliases: []string{"solaris-amd64-smartosbuildlet"},
	},
	"host-solaris-oracle-amd64-oraclerel": &HostConfig{
		Notes:       "Oracle Solaris amd64 Release System",
		Owner:       "shawn.walker@oracle.com",
		OwnerGithub: "binarycrusader",
		IsReverse:   true,
		ExpectNum:   1,
		env:         []string{"GOROOT_BOOTSTRAP=/opt/golang/go-solaris-amd64-bootstrap"},
	},
	"host-solaris-oracle-shawn": &HostConfig{
		Notes:       "Oracle Solaris amd64 Development System",
		Owner:       "shawn.walker@oracle.com",
		OwnerGithub: "binarycrusader",
		IsReverse:   true,
		ExpectNum:   1,
		env:         []string{"GOROOT_BOOTSTRAP=/opt/golang/go-solaris-amd64-bootstrap"},
	},
	"host-linux-mips": &HostConfig{
		Notes:       "Run by Brendan Kirby, imgtec.com",
		OwnerGithub: "MIPSbkirby",
		IsReverse:   true,
		ExpectNum:   1,
		env: []string{
			"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap-mips",
			"GOARCH=mips",
			"GOHOSTARCH=mips",
			"GO_TEST_TIMEOUT_SCALE=4",
		},
		ReverseAliases: []string{"linux-mips"},
	},
	"host-linux-mipsle": &HostConfig{
		Notes:       "Run by Brendan Kirby, imgtec.com",
		OwnerGithub: "MIPSbkirby",
		IsReverse:   true,
		ExpectNum:   1,
		env: []string{
			"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap-mipsle",
			"GOARCH=mipsle",
			"GOHOSTARCH=mipsle",
		},
		ReverseAliases: []string{"linux-mipsle"},
	},
	"host-linux-mips64": &HostConfig{
		Notes:       "Run by Brendan Kirby, imgtec.com",
		OwnerGithub: "MIPSbkirby",
		IsReverse:   true,
		ExpectNum:   1,
		env: []string{
			"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap-mips64",
			"GOARCH=mips64",
			"GOHOSTARCH=mips64",
			"GO_TEST_TIMEOUT_SCALE=4",
		},
		ReverseAliases: []string{"linux-mips64"},
	},
	"host-linux-mips64le": &HostConfig{
		Notes:       "Run by Brendan Kirby, imgtec.com",
		OwnerGithub: "MIPSbkirby",
		IsReverse:   true,
		ExpectNum:   1,
		env: []string{
			"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap-mips64le",
			"GOARCH=mips64le",
			"GOHOSTARCH=mips64le",
		},
		ReverseAliases: []string{"linux-mips64le"},
	},
	"host-darwin-amd64-eliasnaur-android": &HostConfig{
		Notes:       "Mac Mini hosted by Elias Naur, running the android reverse buildlet",
		OwnerGithub: "eliasnaur",
		IsReverse:   true,
		ExpectNum:   1,
		env: []string{
			"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap",
			"GOHOSTARCH=amd64",
			"GOOS=android",
		},
	},
	"host-darwin-amd64-eliasnaur-ios": &HostConfig{
		Notes:       "Mac Mini hosted by Elias Naur, running the ios reverse buildlet",
		OwnerGithub: "eliasnaur",
		IsReverse:   true,
		ExpectNum:   1,
		env: []string{
			"GOROOT_BOOTSTRAP=/usr/local/go-bootstrap",
			"GOHOSTARCH=amd64",
		},
	},
	"host-darwin-amd64-zenly-ios": &HostConfig{
		Notes:       "MacBook Pro hosted by Zenly, running the ios reverse buildlet",
		OwnerGithub: "znly",
		IsReverse:   true,
		ExpectNum:   1,
		env: []string{
			"GOROOT_BOOTSTRAP=/usr/local/Cellar/1.10.3/libexec",
			"GOHOSTARCH=amd64",
		},
	},
	"host-aix-ppc64-osuosl": &HostConfig{
		Notes:       "AIX 7.2 VM on OSU; run by Tony Reix",
		OwnerGithub: "trex58",
		IsReverse:   true,
		ExpectNum:   1,
		env:         []string{"GOROOT_BOOTSTRAP=/opt/freeware/lib/golang"},
	},
}

func init() {
	for key, c := range Hosts {
		if key == "" {
			panic("empty string key in Hosts")
		}
		if c.HostType == "" {
			c.HostType = key
		}
		if c.HostType != key {
			panic(fmt.Sprintf("HostType %q != key %q", c.HostType, key))
		}
		nSet := 0
		if c.VMImage != "" {
			nSet++
		}
		if c.ContainerImage != "" {
			nSet++
		}
		if c.IsReverse {
			nSet++
		}
		if nSet != 1 {
			panic(fmt.Sprintf("exactly one of VMImage, ContainerImage, IsReverse must be set for host %q; got %v", key, nSet))
		}
		if c.buildletURLTmpl == "" && (c.VMImage != "" || c.ContainerImage != "") {
			panic(fmt.Sprintf("missing buildletURLTmpl for host type %q", key))
		}
	}
}

// A HostConfig describes the available ways to obtain buildlets of
// different types. Some host configs can server multiple
// builders. For example, a host config of "host-linux-jessie" can
// serve linux-amd64, linux-amd64-race, linux-386, linux-386-387, etc.
type HostConfig struct {
	// HostType is the unique name of this host config. It is also
	// the key in the Hosts map.
	HostType string

	// buildletURLTmpl is the URL "template" ($BUCKET is auto-expanded)
	// for the URL to the buildlet binary.
	// This field is required for VM and Container builders. It's not
	// needed for reverse buildlets because in that case, the buildlets
	// are already running and their stage0 should know how to update it
	// it automatically.
	buildletURLTmpl string

	// Exactly 1 of these must be set:
	VMImage        string // e.g. "openbsd-amd64-60"
	ContainerImage string // e.g. "linux-buildlet-std:latest" (suffix after "gcr.io/<PROJ>/")
	IsReverse      bool   // if true, only use the reverse buildlet pool

	// GCE options, if VMImage != ""
	machineType string // optional GCE instance type
	RegularDisk bool   // if true, use spinning disk instead of SSD

	// ReverseOptions:
	ExpectNum       int  // expected number of reverse buildlets of this type
	HermeticReverse bool // whether reverse buildlet has fresh env per conn

	// Optional base env. GOROOT_BOOTSTRAP should go here if the buildlet
	// has Go 1.4+ baked in somewhere.
	env []string

	// These template URLs may contain $BUCKET which is expanded to the
	// relevant Cloud Storage bucket as specified by the build environment.
	goBootstrapURLTmpl string // optional URL to a built Go 1.4+ tar.gz

	Owner       string // optional email of owner; "bradfitz@golang.org", empty means golang-dev
	OwnerGithub string // optional GitHub username of owner
	Notes       string // notes for humans

	SSHUsername string // username to ssh as, empty means not supported

	// ReverseAliases lists alternate names for this buildlet
	// config, for older clients doing a reverse dial into the
	// coordinator from outside. This prevents us from updating
	// 75+ dedicated machines/VMs atomically, switching them to
	// the new "host-*" names.
	// This is only applicable if IsReverse.
	ReverseAliases []string
}

// A BuildConfig describes how to run a builder.
type BuildConfig struct {
	// Name is the unique name of the builder, in the form of
	// "GOOS-GOARCH" or "GOOS-GOARCH-suffix". For example,
	// "darwin-386", "linux-386-387", "linux-amd64-race". Some
	// suffixes are well-known and carry special meaning, such as
	// "-race".
	Name string

	// HostType is the required key into the Hosts map, describing
	// the type of host this build will run on.
	// For example, "host-linux-jessie".
	HostType string

	Notes string // notes for humans

	tryBot      func(proj string) bool // if non-nil, policy func for whether trybots enabled. nil means off.
	TryOnly     bool                   // only used for trybots, and not regular builds
	CompileOnly bool                   // if true, compile tests, but don't run them
	FlakyNet    bool                   // network tests are flaky (try anyway, but ignore some failures)

	// MaxAtOnce optionally specifies a cap of how many builds of
	// this type can run at once. Zero means unlimited. This is a
	// temporary measure until the build scheduler
	// (golang.org/issue/19178) is done, at which point this field
	// should be deleted.
	MaxAtOnce int

	// SkipSnapshot, if true, means to not fetch a tarball
	// snapshot of the world post-make.bash from the buildlet (and
	// thus to not write it to Google Cloud Storage). This is
	// incompatible with sharded tests, and should only be used
	// for very slow builders or networks, unable to transfer
	// the tarball in under ~5 minutes.
	SkipSnapshot bool

	// RunBench causes the coordinator to run benchmarks on this buildlet type.
	RunBench bool

	// StopAfterMake causes the build to stop after the make
	// script completes, returning its result as the result of the
	// whole build. It does not run or compile any of the tests,
	// nor does it write a snapshot of the world to cloud
	// storage. This option is only supported for builders whose
	// BuildConfig.SplitMakeRun returns true.
	StopAfterMake bool

	// InstallRacePackages controls which packages to "go install
	// -race <pkgs>" after running make.bash (or equivalent).  If
	// the builder ends in "-race", the default if non-nil is just
	// "std".
	InstallRacePackages []string

	// GoDeps is a list of of git sha1 commits that must be in the
	// commit to be tested's history. If absent, this builder is
	// not run for that commit.
	GoDeps []string

	// ShouldRunDistTest optionally specifies a function which
	// controls whether a test (a name from "go tool dist test
	// -list") is run. The isTry value is true for trybot runs.
	// A few general special cases are handled in
	// cmd/coordinator's in buildStatus.shouldSkipTest.
	ShouldRunDistTest func(distTestName string, isTry bool) bool

	// numTestHelpers is the number of _additional_ buildlets
	// past the first one to help out with sharded tests.
	// For trybots, the numTryHelpers value is used, unless it's
	// zero, in which case numTestHelpers is used.
	numTestHelpers    int
	numTryTestHelpers int // for trybots. if 0, numTesthelpers is used

	env           []string // extra environment ("key=value") pairs
	allScriptArgs []string
}

func (c *BuildConfig) Env() []string {
	env := []string{"GO_BUILDER_NAME=" + c.Name}
	if c.FlakyNet {
		env = append(env, "GO_BUILDER_FLAKY_NET=1")
	}
	env = append(env, c.hostConf().env...)
	return append(env, c.env...)
}

func (c *BuildConfig) IsReverse() bool { return c.hostConf().IsReverse }

func (c *BuildConfig) IsContainer() bool { return c.hostConf().IsContainer() }
func (c *HostConfig) IsContainer() bool  { return c.ContainerImage != "" }

func (c *BuildConfig) IsVM() bool { return c.hostConf().IsVM() }
func (c *HostConfig) IsVM() bool  { return c.VMImage != "" }

func (c *BuildConfig) GOOS() string { return c.Name[:strings.Index(c.Name, "-")] }

func (c *BuildConfig) GOARCH() string {
	arch := c.Name[strings.Index(c.Name, "-")+1:]
	i := strings.Index(arch, "-")
	if i == -1 {
		return arch
	}
	return arch[:i]
}

// FilePathJoin is mostly like filepath.Join (without the cleaning) except
// it uses the path separator of c.GOOS instead of the host system's.
func (c *BuildConfig) FilePathJoin(x ...string) string {
	if c.GOOS() == "windows" {
		return strings.Join(x, "\\")
	}
	return strings.Join(x, "/")
}

func (c *BuildConfig) hostConf() *HostConfig {
	if c, ok := Hosts[c.HostType]; ok {
		return c
	}
	panic(fmt.Sprintf("missing buildlet config for buildlet %q", c.Name))
}

// BuildletBinaryURL returns the public URL of this builder's buildlet.
func (c *BuildConfig) GoBootstrapURL(e *buildenv.Environment) string {
	return strings.Replace(c.hostConf().goBootstrapURLTmpl, "$BUCKET", e.BuildletBucket, 1)
}

// BuildletBinaryURL returns the public URL of this builder's buildlet.
func (c *HostConfig) BuildletBinaryURL(e *buildenv.Environment) string {
	tmpl := c.buildletURLTmpl
	return strings.Replace(tmpl, "$BUCKET", e.BuildletBucket, 1)
}

func (c *BuildConfig) IsRace() bool {
	return strings.HasSuffix(c.Name, "-race")
}

func (c *BuildConfig) GoInstallRacePackages() []string {
	if c.InstallRacePackages != nil {
		return append([]string(nil), c.InstallRacePackages...)
	}
	if c.IsRace() {
		return []string{"std"}
	}
	return nil
}

// AllScript returns the relative path to the operating system's script to
// do the build and run its standard set of tests.
// Example values are "src/all.bash", "src/all.bat", "src/all.rc".
func (c *BuildConfig) AllScript() string {
	if c.Name == "" {
		panic("bogus BuildConfig")
	}
	if c.IsRace() {
		if strings.HasPrefix(c.Name, "windows-") {
			return "src/race.bat"
		}
		return "src/race.bash"
	}
	if strings.HasPrefix(c.Name, "windows-") {
		return "src/all.bat"
	}
	if strings.HasPrefix(c.Name, "plan9-") {
		return "src/all.rc"
	}
	if strings.HasPrefix(c.Name, "nacl-") {
		return "src/nacltest.bash"
	}
	if strings.HasPrefix(c.Name, "android-") {
		return "src/androidtest.bash"
	}
	if strings.HasPrefix(c.Name, "darwin-arm") {
		return "src/iostest.bash"
	}
	if strings.HasPrefix(c.Name, "misc-compile") {
		return "src/buildall.bash"
	}
	return "src/all.bash"
}

// SplitMakeRun reports whether the coordinator should first compile
// (using c.MakeScript), then snapshot, then run the tests (ideally
// sharded) using c.RunScript.
// Eventually this function should always return true (and then be deleted)
// but for now we've only set up the scripts and verified that the main
// configurations work.
func (c *BuildConfig) SplitMakeRun() bool {
	switch c.AllScript() {
	case "src/all.bash", "src/all.bat",
		"src/race.bash", "src/race.bat",
		"src/all.rc",
		"src/nacltest.bash":
		// These we've verified to work.
		return true
	}
	// TODO(bradfitz): make androidtest.bash and iotest.bash work
	// too. And buildall.bash should really just be N small
	// container jobs instead of a "buildall.bash". Then we can
	// delete this whole method.
	return false
}

func (c *BuildConfig) buildSubrepos() bool {
	if !c.SplitMakeRun() {
		return false
	}
	// TODO(bradfitz,dmitshur): move this into BuildConfig bools, rather than this Name switch.
	switch c.Name {
	case "darwin-amd64-10_11",
		"darwin-386-10_11",
		// TODO: add darwin-amd64-10_12 when we have a build scheduler
		"freebsd-amd64-93",
		"freebsd-386-10_3", "freebsd-amd64-10_3",
		"freebsd-386-11_1", "freebsd-amd64-11_1",
		"freebsd-386-10_4", "freebsd-amd64-10_4",
		"freebsd-386-11_2", "freebsd-amd64-11_2",
		"linux-386", "linux-amd64", "linux-amd64-nocgo",
		"linux-s390x-ibm",
		"openbsd-386-60", "openbsd-amd64-60",
		"openbsd-386-62", "openbsd-amd64-62",
		"openbsd-386-64", "openbsd-amd64-64",
		"netbsd-amd64-8_0",
		"netbsd-386-8_0",
		"plan9-386",
		"freebsd-arm-paulzhol",
		"windows-amd64-2016", "windows-386-2008":
		return true
	default:
		return false
	}
}

// BuildRepo reports whether we should do post-submit builds of the provided
// repo ("go", "sys", "net", etc).
func (c *BuildConfig) BuildRepo(repo string) bool {
	if repo == "go" {
		return true
	}
	return c.buildSubrepos()
}

// BuildBranch reports whether we should do post-submit builds of the provided
// branch.
// repo is "go", "sys", "net", etc.
// branch is the branch of the repo (usually "master").
// goBranch is non-empty for a non-"go" repo, and is the branch of Go the subrepo is being tested at.
func (c *BuildConfig) BuildBranch(repo, branch, goBranch string) bool {
	if strings.HasPrefix(c.Name, "darwin-") {
		switch c.Name {
		case "darwin-amd64-10_8", "darwin-amd64-10_10", "darwin-amd64-10_11",
			"darwin-386-10_8", "darwin-386-10_10", "darwin-386-10_11":
			// OS X before Sierra can build any branch.
			// (We've never had a 10.9 builder.)
		default:
			// Sierra or after, however, requires the 1.7 branch:
			switch branch {
			case "release-branch.go1.6",
				"release-branch.go1.5",
				"release-branch.go1.4",
				"release-branch.go1.3",
				"release-branch.go1.2",
				"release-branch.go1.1",
				"release-branch.go1":
				return false
			}
		}
	}
	// NetBSD support was resurrected during the Go 1.10 dev cycle.
	// Skip subrepo builds against Go 1.8 and Go 1.9. Failures there aren't interesting.
	if strings.HasPrefix(c.Name, "netbsd-") {
		switch goBranch {
		case "release-branch.go1.8", "release-branch.go1.9":
			return false
		}
	}
	// Build dev.boringcrypto branches only on linux/amd64 and windows/386 (see golang.org/issue/26791).
	if repo == "go" && (branch == "dev.boringcrypto" || strings.HasPrefix(branch, "dev.boringcrypto.")) {
		if c.Name != "linux-amd64" && c.Name != "windows-386-2008" {
			return false
		}
	}
	return true
}

// AllScriptArgs returns the set of arguments that should be passed to the
// all.bash-equivalent script. Usually empty.
func (c *BuildConfig) AllScriptArgs() []string {
	if strings.HasPrefix(c.Name, "darwin-arm") {
		return []string{"-restart"}
	}
	return append([]string(nil), c.allScriptArgs...)
}

// MakeScript returns the relative path to the operating system's script to
// do the build.
// Example values are "src/make.bash", "src/make.bat", "src/make.rc".
func (c *BuildConfig) MakeScript() string {
	if strings.HasPrefix(c.Name, "windows-") {
		return "src/make.bat"
	}
	if strings.HasPrefix(c.Name, "plan9-") {
		return "src/make.rc"
	}
	if strings.HasPrefix(c.Name, "nacl-") {
		return "src/naclmake.bash"
	}
	return "src/make.bash"
}

// MakeScriptArgs returns the set of arguments that should be passed to the
// make.bash-equivalent script. Usually empty.
func (c *BuildConfig) MakeScriptArgs() []string {
	return c.AllScriptArgs()
}

// RunScript returns the relative path to the operating system's script to
// run the test suite.
// Example values are "src/run.bash", "src/run.bat", "src/run.rc".
func (c *BuildConfig) RunScript() string {
	if strings.HasPrefix(c.Name, "windows-") {
		return "src/run.bat"
	}
	if strings.HasPrefix(c.Name, "plan9-") {
		return "src/run.rc"
	}
	return "src/run.bash"
}

// RunScriptArgs returns the set of arguments that should be passed to the
// run.bash-equivalent script.
func (c *BuildConfig) RunScriptArgs() []string {
	return []string{"--no-rebuild"}
}

// GorootFinal returns the default install location for
// releases for this platform.
func (c *BuildConfig) GorootFinal() string {
	if strings.HasPrefix(c.Name, "windows-") {
		return "c:\\go"
	}
	return "/usr/local/go"
}

// MachineType returns the GCE machine type to use for this builder.
func (c *HostConfig) MachineType() string {
	if v := c.machineType; v != "" {
		return v
	}
	if c.IsContainer() {
		// Set a higher default machine size for containers,
		// so their /workdir tmpfs can be larger. The COS
		// image has no swap, so we want to make sure the
		// /workdir fits completely in memory.
		return "n1-standard-4" // 4 CPUs, 15GB RAM
	}
	return "n1-highcpu-2"
}

// ShortOwner returns a short human-readable owner.
func (c BuildConfig) ShortOwner() string {
	owner := c.hostConf().Owner
	if owner == "" {
		return "go-dev"
	}
	return strings.TrimSuffix(owner, "@golang.org")
}

// OwnerGithub returns the Github handle of the owner.
func (c BuildConfig) OwnerGithub() string {
	return c.hostConf().OwnerGithub
}

// PoolName returns a short summary of the builder's host type for the
// https://farmer.golang.org/builders page.
func (c *HostConfig) PoolName() string {
	switch {
	case c.IsReverse:
		return "Reverse (dedicated machine/VM)"
	case c.IsVM():
		return "GCE VM"
	case c.IsContainer():
		return "Container"
	}
	panic("unknown builder type")
}

// IsHermetic reports whether this host config gets a fresh
// environment (including /usr, /var, etc) for each execution. This is
// true for VMs, GKE, and reverse buildlets running their containers
// running in Docker, but false on some reverse buildlets.
func (c *HostConfig) IsHermetic() bool {
	switch {
	case c.IsReverse:
		return c.HermeticReverse
	case c.IsVM():
		return true
	case c.IsContainer():
		return true
	}
	panic("unknown builder type")
}

// GCENumCPU reports the number of GCE CPUs this buildlet requires.
func (c *HostConfig) GCENumCPU() int {
	t := c.MachineType()
	n, _ := strconv.Atoi(t[strings.LastIndex(t, "-")+1:])
	return n
}

func (c *BuildConfig) NumTestHelpers(isTry bool) int {
	if isTry && c.numTryTestHelpers != 0 {
		return c.numTryTestHelpers
	}
	return c.numTestHelpers
}

// defaultTrySet returns a trybot policy function that reports whether
// a project should use trybots. All the default projects are included,
// plus any given in extraProj.
func defaultTrySet(extraProj ...string) func(proj string) bool {
	return func(proj string) bool {
		if proj == "go" {
			return true
		}
		for _, p := range extraProj {
			if proj == p {
				return true
			}
		}
		switch proj {
		case "grpc-review", "build", "exp", "mobile", "term", "oauth2":
			return false
		}
		return true
	}
}

// explicitTrySet returns a trybot policy function that reports
// whether a project should use trybots. Only the provided projects in
// projs are enabled.
func explicitTrySet(projs ...string) func(proj string) bool {
	return func(proj string) bool {
		for _, p := range projs {
			if proj == p {
				return true
			}
		}
		return false
	}
}

func init() {
	addBuilder(BuildConfig{
		Name:      "freebsd-amd64-gce93",
		HostType:  "host-freebsd-93-gce",
		TryOnly:   true, // don't run regular build...
		MaxAtOnce: 2,
	})
	addBuilder(BuildConfig{
		Name:      "freebsd-amd64-10_3",
		HostType:  "host-freebsd-10_3",
		MaxAtOnce: 2,
	})
	addBuilder(BuildConfig{
		Name:      "freebsd-amd64-10_4",
		HostType:  "host-freebsd-10_4",
		tryBot:    explicitTrySet("sys"),
		MaxAtOnce: 2,
	})
	addBuilder(BuildConfig{
		Name:              "freebsd-amd64-11_1",
		HostType:          "host-freebsd-11_1",
		tryBot:            nil,
		ShouldRunDistTest: fasterTrybots,
		numTryTestHelpers: 4,
		MaxAtOnce:         2,
	})
	addBuilder(BuildConfig{
		Name:              "freebsd-amd64-11_2",
		HostType:          "host-freebsd-11_2",
		tryBot:            explicitTrySet("sys"),
		ShouldRunDistTest: fasterTrybots,
		numTryTestHelpers: 4,
		MaxAtOnce:         2,
	})
	addBuilder(BuildConfig{
		Name:              "freebsd-amd64-12_0",
		HostType:          "host-freebsd-12_0",
		tryBot:            defaultTrySet(),
		ShouldRunDistTest: fasterTrybots,
		numTryTestHelpers: 4,
		MaxAtOnce:         2,
	})
	addBuilder(BuildConfig{
		Name:      "freebsd-amd64-race",
		HostType:  "host-freebsd-11_1",
		MaxAtOnce: 2,
	})
	addBuilder(BuildConfig{
		Name:      "freebsd-386-10_3",
		HostType:  "host-freebsd-10_3",
		env:       []string{"GOARCH=386", "GOHOSTARCH=386"},
		MaxAtOnce: 2,
	})
	addBuilder(BuildConfig{
		Name:      "freebsd-386-10_4",
		HostType:  "host-freebsd-10_4",
		tryBot:    explicitTrySet("sys"),
		env:       []string{"GOARCH=386", "GOHOSTARCH=386"},
		MaxAtOnce: 2,
	})
	addBuilder(BuildConfig{
		Name:              "freebsd-386-11_1",
		HostType:          "host-freebsd-11_1",
		ShouldRunDistTest: noTestDir,
		env:               []string{"GOARCH=386", "GOHOSTARCH=386"},
		MaxAtOnce:         2,
	})
	addBuilder(BuildConfig{
		Name:              "freebsd-386-11_2",
		HostType:          "host-freebsd-11_2",
		ShouldRunDistTest: noTestDir,
		env:               []string{"GOARCH=386", "GOHOSTARCH=386"},
		MaxAtOnce:         2,
	})
	addBuilder(BuildConfig{
		Name:              "linux-386",
		HostType:          "host-linux-jessie",
		ShouldRunDistTest: fasterTrybots,
		tryBot:            defaultTrySet(),
		env:               []string{"GOARCH=386", "GOHOSTARCH=386"},
		numTestHelpers:    1,
		numTryTestHelpers: 3,
	})
	addBuilder(BuildConfig{
		Name:     "linux-386-387",
		Notes:    "GO386=387",
		HostType: "host-linux-jessie",
		env:      []string{"GOARCH=386", "GOHOSTARCH=386", "GO386=387"},
	})
	addBuilder(BuildConfig{
		Name:              "linux-amd64",
		HostType:          "host-linux-jessie",
		tryBot:            defaultTrySet(),
		MaxAtOnce:         3,
		numTestHelpers:    1,
		numTryTestHelpers: 4,
		RunBench:          true,
	})

	const testAlpine = false // Issue 22689 (hide all red builders), Issue 19938 (get Alpine passing)
	if testAlpine {
		addBuilder(BuildConfig{
			Name:     "linux-amd64-alpine",
			HostType: "host-linux-x86-alpine",
		})
	}

	// Add the -vetall builder. The builder name suffix "-vetall" is recognized by cmd/dist/test.go
	// to only run the "go vet std cmd" test and no others.
	addBuilder(BuildConfig{
		Name:           "misc-vet-vetall",
		HostType:       "host-linux-jessie",
		Notes:          "Runs vet over the standard library.",
		tryBot:         defaultTrySet(),
		numTestHelpers: 5,
	})

	addMiscCompile := func(suffix, rx string) {
		addBuilder(BuildConfig{
			Name:        "misc-compile" + suffix,
			HostType:    "host-linux-jessie",
			tryBot:      defaultTrySet(),
			TryOnly:     true,
			CompileOnly: true,
			Notes:       "Runs buildall.sh to cross-compile std packages for " + rx + ", but doesn't run any tests.",
			allScriptArgs: []string{
				// Filtering pattern to buildall.bash:
				rx,
			},
		})
	}
	addMiscCompile("", "^(linux-arm64|linux-s390x|solaris-amd64|darwin-386)$") // 4 ports
	addMiscCompile("-nacl", "^nacl")                                           // 3
	addMiscCompile("-mips", "^linux-mips")                                     // 4
	addMiscCompile("-ppc", "^linux-ppc64")                                     // 2
	addMiscCompile("-plan9", "^plan9-")                                        // 3
	addMiscCompile("-freebsd", "^freebsd-")                                    // 3
	addMiscCompile("-netbsd", "^netbsd-")                                      // 3
	addMiscCompile("-openbsd", "^openbsd-")                                    // 3
	// TODO: Issue 25963, get the misc-compile trybots for mobile working, and then:
	// addMiscCompile("-mobile", "(^android|darwin-arm64)")                       // 5 ports

	addBuilder(BuildConfig{
		Name:      "linux-amd64-nocgo",
		HostType:  "host-linux-jessie",
		MaxAtOnce: 1,
		Notes:     "cgo disabled",
		env: []string{
			"CGO_ENABLED=0",
			// This USER=root was required for Docker-based builds but probably isn't required
			// in the VM anymore, since the buildlet probably already has this in its environment.
			// (It was required because without cgo, it couldn't find the username)
			"USER=root",
		},
	})
	addBuilder(BuildConfig{
		Name:      "linux-amd64-noopt",
		Notes:     "optimizations and inlining disabled",
		HostType:  "host-linux-jessie",
		env:       []string{"GO_GCFLAGS=-N -l"},
		MaxAtOnce: 1,
	})
	addBuilder(BuildConfig{
		Name:        "linux-amd64-ssacheck",
		HostType:    "host-linux-jessie",
		MaxAtOnce:   1,
		tryBot:      nil, // TODO: add a func to conditionally run this trybot if compiler dirs are touched
		CompileOnly: true,
		Notes:       "SSA internal checks enabled",
		env:         []string{"GO_GCFLAGS=-d=ssa/check/on,dclstack"},
		GoDeps: []string{
			"f65abf6ddc8d1f3d403a9195fd74eaffa022b07f", // adds dclstack
		},
	})
	addBuilder(BuildConfig{
		Name:                "linux-amd64-racecompile",
		HostType:            "host-linux-jessie",
		tryBot:              nil, // TODO: add a func to conditionally run this trybot if compiler dirs are touched
		MaxAtOnce:           1,
		CompileOnly:         true,
		SkipSnapshot:        true,
		StopAfterMake:       true,
		InstallRacePackages: []string{"cmd/compile"},
		Notes:               "race-enabled cmd/compile",
		GoDeps: []string{
			"22f1b56dab29d397d2bdbdd603d85e60fb678089", // adds cmd/compile -c; Issue 20222
		},
	})
	addBuilder(BuildConfig{
		Name:              "linux-amd64-race",
		HostType:          "host-linux-jessie",
		tryBot:            defaultTrySet(),
		MaxAtOnce:         1,
		ShouldRunDistTest: fasterTrybots,
		numTestHelpers:    1,
		numTryTestHelpers: 5,
	})
	addBuilder(BuildConfig{
		Name:      "linux-386-clang",
		HostType:  "host-linux-clang",
		MaxAtOnce: 1,
		Notes:     "Debian jessie + clang 3.9 instead of gcc",
		env:       []string{"CC=/usr/bin/clang", "GOHOSTARCH=386"},
	})
	addBuilder(BuildConfig{
		Name:      "linux-amd64-clang",
		HostType:  "host-linux-clang",
		MaxAtOnce: 1,
		Notes:     "Debian jessie + clang 3.9 instead of gcc",
		env:       []string{"CC=/usr/bin/clang"},
	})
	addBuilder(BuildConfig{
		Name:      "linux-386-sid",
		HostType:  "host-linux-sid",
		Notes:     "Debian sid (unstable)",
		MaxAtOnce: 1,
		env:       []string{"GOHOSTARCH=386"},
	})
	addBuilder(BuildConfig{
		Name:      "linux-amd64-sid",
		HostType:  "host-linux-sid",
		MaxAtOnce: 1,
		Notes:     "Debian sid (unstable)",
	})
	addBuilder(BuildConfig{
		Name:      "linux-amd64-stretch",
		HostType:  "host-linux-stretch",
		MaxAtOnce: 1,
		Notes:     "Debian Stretch",
	})
	addBuilder(BuildConfig{
		Name:      "linux-amd64-longtest",
		HostType:  "host-linux-stretch",
		MaxAtOnce: 1,
		Notes:     "Debian Stretch with go test -short=false",
		env: []string{
			"GO_TEST_SHORT=0",
			// runtime takes ~190 seconds in long mode, which is over
			// the 180 seconds default timeout.
			"GO_TEST_TIMEOUT_SCALE=2",
		},
	})
	addBuilder(BuildConfig{
		Name:              "linux-arm",
		HostType:          "host-linux-arm-scaleway",
		tryBot:            nil, // Issue 22748, Issue 22749
		FlakyNet:          true,
		numTestHelpers:    2,
		numTryTestHelpers: 7,
	})
	addBuilder(BuildConfig{
		Name:          "linux-arm-nativemake",
		Notes:         "runs make.bash on real ARM hardware, but does not run tests",
		HostType:      "host-linux-arm-scaleway",
		StopAfterMake: true,
	})
	addBuilder(BuildConfig{
		Name:     "linux-arm-arm5spacemonkey",
		HostType: "host-linux-arm5spacemonkey",
		env: []string{
			"GOARM=5",
			"GO_TEST_TIMEOUT_SCALE=4", // arm is normally 2; double that.
		},
		ShouldRunDistTest: func(distTest string, isTry bool) bool {
			if strings.Contains(distTest, "vendor/github.com/google/pprof") {
				// Not worth it. And broken.
				return false
			}
			if distTest == "api" {
				// Broken on this build config (Issue
				// 24754), and not worth it on slow
				// builder. It's covered by other
				// builders anyway.
				return false
			}
			if strings.HasPrefix(distTest, "test:") {
				// Slow, and not worth it on slow builder.
				return false
			}
			return true
		},
	})
	addBuilder(BuildConfig{
		Name:              "nacl-386",
		HostType:          "host-nacl-kube",
		tryBot:            explicitTrySet("go"),
		MaxAtOnce:         2,
		numTryTestHelpers: 3,
		env:               []string{"GOOS=nacl", "GOARCH=386", "GOHOSTOS=linux", "GOHOSTARCH=amd64"},
	})
	addBuilder(BuildConfig{
		Name:              "nacl-amd64p32",
		HostType:          "host-nacl-kube",
		tryBot:            explicitTrySet("go"),
		MaxAtOnce:         2,
		numTryTestHelpers: 3,
		env:               []string{"GOOS=nacl", "GOARCH=amd64p32", "GOHOSTOS=linux", "GOHOSTARCH=amd64"},
	})
	addBuilder(BuildConfig{
		Name:     "js-wasm",
		HostType: "host-js-wasm",
		tryBot:   explicitTrySet("go"),
		ShouldRunDistTest: func(distTest string, isTry bool) bool {
			if isTry {
				if strings.HasPrefix(distTest, "test:") {
					return false
				}
				if strings.Contains(distTest, "/internal/") ||
					strings.Contains(distTest, "vendor/golang.org/x/arch") {
					return false
				}
				switch distTest {
				case "cmd/go", "nolibgcc:crypto/x509":
					return false
				}
				return true
			}
			return true
		},
		numTryTestHelpers: 4,
		GoDeps: []string{
			"3dced519cbabc213df369d9112206986e62687fa", // first passing commit
		},
		env: []string{
			"GOOS=js", "GOARCH=wasm", "GOHOSTOS=linux", "GOHOSTARCH=amd64",
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:/workdir/go/misc/wasm",
		},
	})
	addBuilder(BuildConfig{
		Name:              "openbsd-amd64-60",
		HostType:          "host-openbsd-amd64-60",
		ShouldRunDistTest: noTestDir,
		TryOnly:           true, // disabled by default; Go 1.11+ don't support it anymore
		tryBot:            nil,
		MaxAtOnce:         1,
		numTestHelpers:    2,
		numTryTestHelpers: 5,
	})
	addBuilder(BuildConfig{
		Name:              "openbsd-386-60",
		HostType:          "host-openbsd-386-60",
		ShouldRunDistTest: noTestDir,
		TryOnly:           true, // disabled by default; Go 1.11+ don't support it anymore
		tryBot:            nil,
		MaxAtOnce:         1,
		env: []string{
			// cmd/go takes ~192 seconds on openbsd-386
			// now, which is over the 180 second default
			// dist test timeout. So, bump this builder
			// up:
			"GO_TEST_TIMEOUT_SCALE=2",
		},
	})
	addBuilder(BuildConfig{
		Name:              "openbsd-386-62",
		HostType:          "host-openbsd-386-62",
		ShouldRunDistTest: noTestDir,
		MaxAtOnce:         1,
		env: []string{
			// cmd/go takes ~192 seconds on openbsd-386
			// now, which is over the 180 second default
			// dist test timeout. So, bump this builder
			// up:
			"GO_TEST_TIMEOUT_SCALE=2",
		},
	})
	addBuilder(BuildConfig{
		Name:              "openbsd-amd64-62",
		HostType:          "host-openbsd-amd64-62",
		ShouldRunDistTest: noTestDir,
		tryBot:            nil,
		numTestHelpers:    0,
		numTryTestHelpers: 5,
		MaxAtOnce:         1,
	})
	addBuilder(BuildConfig{
		Name:              "openbsd-amd64-64",
		HostType:          "host-openbsd-amd64-64",
		ShouldRunDistTest: noTestDir,
		tryBot:            defaultTrySet(),
		numTestHelpers:    0,
		numTryTestHelpers: 5,
		MaxAtOnce:         1,
	})
	addBuilder(BuildConfig{
		Name:              "openbsd-386-64",
		HostType:          "host-openbsd-386-64",
		tryBot:            explicitTrySet("sys"),
		ShouldRunDistTest: noTestDir,
		MaxAtOnce:         1,
	})
	addBuilder(BuildConfig{
		Name:              "netbsd-amd64-8_0",
		HostType:          "host-netbsd-amd64-8_0",
		ShouldRunDistTest: noTestDir,
		MaxAtOnce:         1,
		tryBot:            explicitTrySet("sys"),
	})
	addBuilder(BuildConfig{
		Name:              "netbsd-386-8_0",
		HostType:          "host-netbsd-386-8_0",
		ShouldRunDistTest: noTestDir,
		MaxAtOnce:         1,
		// This builder currently hangs in the “../test” phase of all.bash.
		// (https://golang.org/issue/25206)
		TryOnly: true, // Disable regular builds.
		tryBot:  nil,  // Disable trybots.
	})
	addBuilder(BuildConfig{
		Name:           "plan9-386",
		HostType:       "host-plan9-386-gce",
		numTestHelpers: 1,
		MaxAtOnce:      2,
	})
	addBuilder(BuildConfig{
		Name:              "windows-amd64-2008",
		HostType:          "host-windows-amd64-2008",
		ShouldRunDistTest: noTestDir,
		env: []string{
			"GOARCH=amd64",
			"GOHOSTARCH=amd64",
			// cmd/go takes ~188 seconds on windows-amd64
			// now, which is over the 180 second default
			// dist test timeout. So, bump this builder
			// up:
			"GO_TEST_TIMEOUT_SCALE=2",
		},
	})
	addBuilder(BuildConfig{
		Name:              "windows-386-2008",
		HostType:          "host-windows-amd64-2008",
		ShouldRunDistTest: fasterTrybots,
		env:               []string{"GOARCH=386", "GOHOSTARCH=386"},
		MaxAtOnce:         2,
		tryBot:            defaultTrySet(),
		numTryTestHelpers: 4,
	})
	addBuilder(BuildConfig{
		Name:              "windows-amd64-2012",
		HostType:          "host-windows-amd64-2012",
		ShouldRunDistTest: noTestDir,
		env: []string{
			"GOARCH=amd64",
			"GOHOSTARCH=amd64",
			// cmd/go takes ~188 seconds on windows-amd64
			// now, which is over the 180 second default
			// dist test timeout. So, bump this builder
			// up:
			"GO_TEST_TIMEOUT_SCALE=2",
		},
		MaxAtOnce: 2,
	})
	addBuilder(BuildConfig{
		Name:              "windows-amd64-2016",
		HostType:          "host-windows-amd64-2016",
		ShouldRunDistTest: fasterTrybots,
		env: []string{
			"GOARCH=amd64",
			"GOHOSTARCH=amd64",
			// cmd/go takes ~188 seconds on windows-amd64
			// now, which is over the 180 second default
			// dist test timeout. So, bump this builder
			// up:
			"GO_TEST_TIMEOUT_SCALE=2",
		},
		tryBot:            defaultTrySet(),
		numTryTestHelpers: 5,
	})
	addBuilder(BuildConfig{
		Name:     "windows-amd64-race",
		HostType: "host-windows-amd64-2008",
		Notes:    "Only runs -race tests (./race.bat)",
		env: []string{
			"GOARCH=amd64",
			"GOHOSTARCH=amd64",
			// cmd/go takes ~188 seconds on windows-amd64
			// now, which is over the 180 second default
			// dist test timeout. So, bump this builder
			// up:
			"GO_TEST_TIMEOUT_SCALE=2"},
	})
	addBuilder(BuildConfig{
		Name:     "windows-arm",
		HostType: "host-windows-arm-iotcore",
		env: []string{
			"GOARM=7",
			"GO_TEST_TIMEOUT_SCALE=2",
		},
	})
	addBuilder(BuildConfig{
		Name:              "darwin-amd64-10_8",
		HostType:          "host-darwin-10_8",
		ShouldRunDistTest: noTestDir,
		TryOnly:           true, // but not in trybot set, so effectively disabled
		tryBot:            nil,
	})
	addBuilder(BuildConfig{
		Name:              "darwin-amd64-10_10",
		HostType:          "host-darwin-10_10",
		ShouldRunDistTest: noTestDir,
	})
	addBuilder(BuildConfig{
		Name:              "darwin-amd64-10_11",
		HostType:          "host-darwin-10_11",
		tryBot:            nil, // disabled until Macs fixed; https://golang.org/issue/23859
		ShouldRunDistTest: noTestDir,
		numTryTestHelpers: 3,
	})
	addBuilder(BuildConfig{
		Name:              "darwin-386-10_11",
		HostType:          "host-darwin-10_11",
		ShouldRunDistTest: noTestDir,
		MaxAtOnce:         1,
		env:               []string{"GOARCH=386", "GOHOSTARCH=386"},
	})
	addBuilder(BuildConfig{
		Name:              "darwin-amd64-10_12",
		HostType:          "host-darwin-10_12",
		ShouldRunDistTest: noTestDir,
	})
	addBuilder(BuildConfig{
		Name:              "darwin-amd64-race",
		HostType:          "host-darwin-10_11",
		ShouldRunDistTest: noTestDir,
	})
	addBuilder(BuildConfig{
		Name:     "darwin-arm-a1428ios",
		HostType: "host-darwin-amd64-eliasnaur-ios",
		Notes:    "iPhone 6 (model A1586), via a Mac Mini; owned by elias.naur",
		env:      []string{"GOARCH=arm"},
	})
	addBuilder(BuildConfig{
		Name:     "darwin-arm64-a1549ios",
		HostType: "host-darwin-amd64-eliasnaur-ios",
		Notes:    "iPhone 6 (model A1586), via a Mac Mini; owned by elias.naur",
		env:      []string{"GOARCH=arm64"},
	})
	addBuilder(BuildConfig{
		Name:     "darwin-arm-mg912baios",
		HostType: "host-darwin-amd64-zenly-ios",
		Notes:    "iPhone 5C (model MG912B/A), via a MacBook Pro; owned by zenly",
		env: []string{
			"GOARCH=arm",
			"GOIOS_DEVICE_ID=8e5c23a5d0843d1ffe164ea0b2f2500599c3ebff",
		},
	})
	addBuilder(BuildConfig{
		Name:     "darwin-arm64-mn4m2zdaios",
		HostType: "host-darwin-amd64-zenly-ios",
		Notes:    "iPhone 7+ (model MN4M2ZD/A), via a MacBook Pro; owned by zenly",
		env: []string{
			"GOARCH=arm64",
			"GOIOS_DEVICE_ID=5ec20fafe317e1c8ff51efc6d508cf19808474a2",
		},
	})
	addBuilder(BuildConfig{
		Name:     "android-arm-wiko-fever",
		HostType: "host-darwin-amd64-eliasnaur-android",
		Notes:    "Android Wiko Fever phone running Android 6.0, via a Mac Mini",
		env: []string{
			"GOARCH=arm",
			"GOARM=7",
			"GOANDROID_ADB_FLAGS=-d", // Run on device
			"CC_FOR_TARGET=/Users/elias/android-ndk-standalone-arm/bin/clang",
		},
	})
	addBuilder(BuildConfig{
		Name:     "android-arm64-wiko-fever",
		HostType: "host-darwin-amd64-eliasnaur-android",
		Notes:    "Android Wiko Fever phone running Android 6.0, via a Mac Mini",
		env: []string{
			"GOARCH=arm64",
			"GOANDROID_ADB_FLAGS=-d", // Run on device
			"CC_FOR_TARGET=/Users/elias/android-ndk-standalone-arm64/bin/clang",
		},
	})
	addBuilder(BuildConfig{
		Name:     "android-386-emulator",
		HostType: "host-darwin-amd64-eliasnaur-android",
		Notes:    "Android emulator, via a Mac Mini",
		env: []string{
			"GOARCH=386",
			"GOANDROID_ADB_FLAGS=-e", // Run on emulator
			"CC_FOR_TARGET=/Users/elias/android-ndk-standalone-386/bin/clang",
		},
	})
	addBuilder(BuildConfig{
		Name:     "android-amd64-emulator",
		HostType: "host-darwin-amd64-eliasnaur-android",
		Notes:    "Android emulator, via a Mac Mini",
		env: []string{
			"GOARCH=amd64",
			"GOANDROID_ADB_FLAGS=-e", // Run on emulator
			"CC_FOR_TARGET=/Users/elias/android-ndk-standalone-amd64/bin/clang",
		},
	})
	addBuilder(BuildConfig{
		Name:     "solaris-amd64-oracledev",
		HostType: "host-solaris-oracle-shawn",
		Notes:    "Oracle Solaris development version",
	})
	addBuilder(BuildConfig{
		Name:     "solaris-amd64-oraclerel",
		HostType: "host-solaris-oracle-amd64-oraclerel",
		Notes:    "Oracle Solaris release version",
	})
	addBuilder(BuildConfig{
		Name:     "solaris-amd64-smartosbuildlet",
		HostType: "host-solaris-amd64",
	})
	addBuilder(BuildConfig{
		Name:     "linux-ppc64-buildlet",
		HostType: "host-linux-ppc64-osu",
		FlakyNet: true,
	})
	addBuilder(BuildConfig{
		Name:     "linux-ppc64le-buildlet",
		HostType: "host-linux-ppc64le-osu",
		FlakyNet: true,
	})
	addBuilder(BuildConfig{
		Name:     "linux-arm64-buildlet",
		HostType: "host-linux-arm64-linaro",
		FlakyNet: true,
	})
	addBuilder(BuildConfig{
		Name:     "linux-arm64-packet",
		HostType: "host-linux-arm64-packet",
		FlakyNet: true, // unknown; just copied from the linaro one
	})
	addBuilder(BuildConfig{
		Name:         "linux-mips",
		HostType:     "host-linux-mips",
		SkipSnapshot: true,
	})
	addBuilder(BuildConfig{
		Name:         "linux-mipsle",
		HostType:     "host-linux-mipsle",
		SkipSnapshot: true,
	})
	addBuilder(BuildConfig{
		Name:         "linux-mips64",
		HostType:     "host-linux-mips64",
		SkipSnapshot: true,
	})
	addBuilder(BuildConfig{
		Name:         "linux-mips64le",
		HostType:     "host-linux-mips64le",
		SkipSnapshot: true,
	})
	addBuilder(BuildConfig{
		Name:           "linux-s390x-ibm",
		HostType:       "host-linux-s390x",
		numTestHelpers: 0,
	})
	addBuilder(BuildConfig{
		Name:        "linux-s390x-crosscompile",
		HostType:    "host-s390x-cross-kube",
		Notes:       "s390x cross-compile builder for releases; doesn't run tests",
		CompileOnly: true,
		TryOnly:     true, // but not in trybot set for now
		env: []string{
			"CGO_ENABLED=1",
			"GOARCH=s390x",
			"GOHOSTARCH=amd64",
			"CC_FOR_TARGET=s390x-linux-gnu-gcc",
		},
	})
	addBuilder(BuildConfig{
		Name:     "linux-amd64-localdev",
		HostType: "host-linux-amd64-localdev",
		Notes:    "for localhost development only",
		TryOnly:  true,
	})
	addBuilder(BuildConfig{
		Name:              "dragonfly-amd64",
		HostType:          "host-dragonfly-amd64-tdfbsd",
		ShouldRunDistTest: noTestDir,
		SkipSnapshot:      true,
	})
	addBuilder(BuildConfig{
		Name:         "freebsd-arm-paulzhol",
		HostType:     "host-freebsd-arm-paulzhol",
		SkipSnapshot: true,
		env: []string{
			"GOARM=7",
			"CGO_ENABLED=1",
		},
	})
	addBuilder(BuildConfig{
		Name:         "plan9-arm",
		HostType:     "host-plan9-arm-0intro",
		SkipSnapshot: true,
	})
	addBuilder(BuildConfig{
		Name:         "nacl-arm",
		HostType:     "host-nacl-arm-davecheney",
		SkipSnapshot: true,
	})
	addBuilder(BuildConfig{
		Name:         "plan9-amd64-9front",
		HostType:     "host-plan9-amd64-0intro",
		SkipSnapshot: true,
	})
	addBuilder(BuildConfig{
		Name:     "aix-ppc64",
		HostType: "host-aix-ppc64-osuosl",
	})

}

// addBuilder adds c to the Builders map after doing some sanity
// checks.
func addBuilder(c BuildConfig) {
	if c.Name == "" {
		panic("empty name")
	}
	if c.HostType == "" {
		panic(fmt.Sprintf("missing HostType for builder %q", c.Name))
	}
	if _, dup := Builders[c.Name]; dup {
		panic("dup name " + c.Name)
	}
	if _, ok := Hosts[c.HostType]; !ok {
		panic(fmt.Sprintf("undefined HostType %q for builder %q", c.HostType, c.Name))
	}
	if c.SkipSnapshot && (c.numTestHelpers > 0 || c.numTryTestHelpers > 0) {
		panic(fmt.Sprintf("config %q's SkipSnapshot is not compatible with sharded test helpers", c.Name))
	}

	types := 0
	for _, fn := range []func() bool{c.IsReverse, c.IsContainer, c.IsVM} {
		if fn() {
			types++
		}
	}
	if types != 1 {
		panic(fmt.Sprintf("build config %q host type inconsistent (must be Reverse, Image, or VM)", c.Name))
	}

	Builders[c.Name] = &c
}

// fasterTrybots is a ShouldRunDistTest policy function.
// It skips (returns false) the test/ directory tests for trybots.
func fasterTrybots(distTest string, isTry bool) bool {
	if isTry && strings.HasPrefix(distTest, "test:") {
		return false // skip test
	}
	return true
}

// noTestDir is a ShouldRunDistTest policy function.
// It skips (returns false) the test/ directory tests for all builds.
func noTestDir(distTest string, isTry bool) bool {
	if strings.HasPrefix(distTest, "test:") {
		return false // skip test
	}
	return true
}

// TryBuildersForProject returns the builders that should run as part of
// a TryBot set for the given project.
// The project argument is of the form "go", "net", "sys", etc.
func TryBuildersForProject(proj string) []*BuildConfig {
	var confs []*BuildConfig
	for _, conf := range Builders {
		if conf.tryBot != nil && conf.tryBot(proj) {
			confs = append(confs, conf)
		}
	}
	sort.Slice(confs, func(i, j int) bool {
		return confs[i].Name < confs[j].Name
	})
	return confs
}
