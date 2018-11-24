// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
The builder binary is the old continuous build client for the Go
project. It is still used by a few builders, but most builders have
been migrated to our newer build & test infrastructure.

This command is intended to run continuously as a background process.
It periodically pulls updates from the Go git repository and requests
work to do from the Go Dashboard AppEngine application running at
https://build.golang.org/.

When a new commit to test is found, the Go Builder creates a clone of
the repository, runs all.bash, and reports build success or failure to
the Go Dashboard.

Usage:

  $ builder goos-goarch...

  Several goos-goarch combinations can be provided, and the builder will
  build them in serial.

Optional flags:

  -dashboard="https://build.golang.org": Go Dashboard Host
    The location of the Go Dashboard application to which Go Builder will
    report its results.

  -rev=N: Build revision N and exit

  -cmd="./all.bash": Build command (specify absolute or relative to go/src)
    This flag is overridden in the following conditions:

    - If the build key ends in -race, then race.bash or race.bat will be chosen.
    - If the build key begins with nacl, then nacltest.bash will be chosen.

  -v: Verbose logging

The key file should be located at $HOME/.gobuildkey or, for a builder-specific
key, $HOME/.gobuildkey-$BUILDER (eg, $HOME/.gobuildkey-linux-amd64).

The build key file is a text file of the format:

  godashboard-key

*/
package main // import "golang.org/x/build/cmd/builder"
