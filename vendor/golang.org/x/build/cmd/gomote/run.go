// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/build/buildlet"
	"golang.org/x/build/dashboard"
	"golang.org/x/build/envutil"
)

func run(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "run usage: gomote run [run-opts] <instance> <cmd> [args...]")
		fs.PrintDefaults()
		os.Exit(1)
	}
	var sys bool
	fs.BoolVar(&sys, "system", false, "run inside the system, and not inside the workdir; this is implicit if cmd starts with '/'")
	var debug bool
	fs.BoolVar(&debug, "debug", false, "write debug info about the command's execution before it begins")
	var env stringSlice
	fs.Var(&env, "e", "Environment variable KEY=value. The -e flag may be repeated multiple times to add multiple things to the environment.")
	var path string
	fs.StringVar(&path, "path", "", "Comma-separated list of ExecOpts.Path elements. The special string 'EMPTY' means to run without any $PATH. The empty string (default) does not modify the $PATH. Otherwise, the following expansions apply: the string '$PATH' expands to the current PATH element(s), the substring '$WORKDIR' expands to the buildlet's temp workdir.")

	var dir string
	fs.StringVar(&dir, "dir", "", "Directory to run from. Defaults to the directory of the command, or the work directory if -system is true.")
	var builderEnv string
	fs.StringVar(&builderEnv, "builderenv", "", "Optional alternate builder to act like. Must share the same underlying buildlet host type, or it's an error. For instance, linux-amd64-race or linux-386-387 are compatible with linux-amd64, but openbsd-amd64 and openbsd-386 are different hosts.")

	fs.Parse(args)
	if fs.NArg() < 2 {
		fs.Usage()
	}
	name, cmd := fs.Arg(0), fs.Arg(1)

	var conf *dashboard.BuildConfig

	bc, conf, err := clientAndConf(name)
	if err != nil {
		return err
	}

	if builderEnv != "" {
		altConf, ok := dashboard.Builders[builderEnv]
		if !ok {
			return fmt.Errorf("unknown --builderenv=%q builder value", builderEnv)
		}
		if altConf.HostType != conf.HostType {
			return fmt.Errorf("--builderEnv=%q has host type %q, which is not compatible with the named buildlet's host type %q",
				builderEnv, altConf.HostType, conf.HostType)
		}
		conf = altConf
	}

	var pathOpt []string
	if path == "EMPTY" {
		pathOpt = []string{} // non-nil
	} else if path != "" {
		pathOpt = strings.Split(path, ",")
	}

	remoteErr, execErr := bc.Exec(cmd, buildlet.ExecOpts{
		Dir:         dir,
		SystemLevel: sys || strings.HasPrefix(cmd, "/"),
		Output:      os.Stdout,
		Args:        fs.Args()[2:],
		ExtraEnv:    envutil.Dedup(conf.GOOS() == "windows", append(conf.Env(), []string(env)...)),
		Debug:       debug,
		Path:        pathOpt,
	})
	if execErr != nil {
		return fmt.Errorf("Error trying to execute %s: %v", cmd, execErr)
	}
	return remoteErr
}

// stringSlice implements flag.Value, specifically for storing environment
// variable key=value pairs.
type stringSlice []string

func (*stringSlice) String() string { return "" } // default value

func (ss *stringSlice) Set(v string) error {
	if v != "" {
		if !strings.Contains(v, "=") {
			return fmt.Errorf("-e argument %q doesn't contains an '=' sign.", v)
		}
		*ss = append(*ss, v)
	}
	return nil
}
