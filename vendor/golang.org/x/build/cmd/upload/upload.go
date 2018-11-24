// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The upload command writes a file to Google Cloud Storage. It's used
// exclusively by the Makefiles in the Go project repos. Think of it
// as a very light version of gsutil or gcloud, but with some
// Go-specific configuration knowledge baked in.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/build/envutil"
)

var (
	public        = flag.Bool("public", false, "object should be world-readable")
	cacheable     = flag.Bool("cacheable", true, "object should be cacheable")
	file          = flag.String("file", "-", "Filename to read object from, or '-' for stdin. If it begins with 'go:' then the rest is considered to be a Go target to install first, and then upload.")
	verbose       = flag.Bool("verbose", false, "verbose logging")
	osarch        = flag.String("osarch", "", "Optional 'GOOS-GOARCH' value to cross-compile; used only if --file begins with 'go:'. As a special case, if the value contains a '.' byte, anything up to and including that period is discarded.")
	project       = flag.String("project", "", "GCE Project. If blank, it's automatically inferred from the bucket name for the common Go buckets.")
	tags          = flag.String("tags", "", "tags to pass to go list, go install, etc. Only applicable if the --file value begins with 'go:'")
	doGzip        = flag.Bool("gzip", false, "gzip the stored contents (not the upload's Content-Encoding); this forces the Content-Type to be application/octet-stream. To prevent misuse, the object name must also end in '.gz'")
	extraEnv      = flag.String("extraenv", "", "comma-separated list of addition KEY=val environment pairs to include in build environment when building a target to upload")
	installSuffix = flag.String("installsuffix", "", "installsuffix for the go command")
	static        = flag.Bool("static", false, "compile the binary statically, adds necessary ldflags")
)

// to match uploads to e.g. https://storage.googleapis.com/golang/go1.4-bootstrap-20170531.tar.gz.
var go14BootstrapRx = regexp.MustCompile(`^go1\.4-bootstrap-20\d{6}\.tar\.gz$`)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: upload [--public] [--file=...] <bucket/object>

If <bucket/object> is of the form "golang/go1.4-bootstrap-20yymmdd.tar.gz",
then the current release-branch.go1.4 is uploaded from Gerrit, with each
tar entry filename beginning with the prefix "go/".

`)
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	args := strings.SplitN(flag.Arg(0), "/", 2)
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	if strings.HasPrefix(*file, "go:") {
		buildGoTarget()
	}
	bucket, object := args[0], args[1]

	// Special support for auto-tarring up Go 1.4 tarballs from the 1.4 release branch.
	is14Src := bucket == "golang" && go14BootstrapRx.MatchString(object)
	if is14Src {
		if *file != "-" {
			log.Fatalf("invalid use of --file with Go 1.4 tarball %v", object)
		}
		*doGzip = true
		*public = true
		*cacheable = true
	}

	if *doGzip && !strings.HasSuffix(object, ".gz") {
		log.Fatalf("--gzip flag requires object ending in .gz")
	}

	proj := *project
	if proj == "" {
		proj, _ = bucketProject[bucket]
		if proj == "" {
			log.Fatalf("bucket %q doesn't have an associated project in upload.go", bucket)
		}
	}

	ctx := context.Background()
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		log.Fatalf("storage.NewClient: %v", err)
	}

	if is14Src {
		_, err := storageClient.Bucket(bucket).Object(object).Attrs(context.Background())
		if err != storage.ErrObjectNotExist {
			if err == nil {
				log.Fatalf("object %v already exists; refusing to overwrite.", object)
			}
			log.Fatalf("error checking for %v: %v", object, err)
		}
	} else if alreadyUploaded(storageClient, bucket, object) {
		if *verbose {
			log.Printf("Already uploaded.")
		}
		return
	}

	w := storageClient.Bucket(bucket).Object(object).NewWriter(ctx)
	// If you don't give the owners access, the web UI seems to
	// have a bug and doesn't have access to see that it's public, so
	// won't render the "Shared Publicly" link. So we do that, even
	// though it's dumb and unnecessary otherwise:
	w.ACL = append(w.ACL, storage.ACLRule{Entity: storage.ACLEntity("project-owners-" + proj), Role: storage.RoleOwner})
	if *public {
		w.ACL = append(w.ACL, storage.ACLRule{Entity: storage.AllUsers, Role: storage.RoleReader})
		if !*cacheable {
			w.CacheControl = "no-cache"
		}
	}
	var content io.Reader
	switch {
	case is14Src:
		content = generate14Tarfile()
	case *file == "-":
		content = os.Stdin
	default:
		content, err = os.Open(*file)
		if err != nil {
			log.Fatal(err)
		}
	}
	if *doGzip {
		var zbuf bytes.Buffer
		zw := gzip.NewWriter(&zbuf)
		if _, err := io.Copy(zw, content); err != nil {
			log.Fatalf("compressing content: %v", err)
		}
		if err := zw.Close(); err != nil {
			log.Fatalf("gzip.Close: %v", err)
		}
		content = &zbuf
	}

	const maxSlurp = 1 << 20
	var buf bytes.Buffer
	n, err := io.CopyN(&buf, content, maxSlurp)
	if err != nil && err != io.EOF {
		log.Fatalf("Error reading from stdin: %v, %v", n, err)
	}
	if *doGzip {
		w.ContentType = "application/octet-stream"
	} else {
		w.ContentType = http.DetectContentType(buf.Bytes())
	}

	_, err = io.Copy(w, io.MultiReader(&buf, content))
	if cerr := w.Close(); cerr != nil && err == nil {
		err = cerr
	}
	if err != nil {
		log.Fatalf("Write error: %v", err)
	}
	if *verbose {
		log.Printf("Wrote %v", object)
	}
	os.Exit(0)
}

var bucketProject = map[string]string{
	"dev-gccgo-builder-data": "gccgo-dashboard-dev",
	"dev-go-builder-data":    "go-dashboard-dev",
	"gccgo-builder-data":     "gccgo-dashboard-builders",
	"go-builder-data":        "symbolic-datum-552",
	"go-build-log":           "symbolic-datum-552",
	"http2-demo-server-tls":  "symbolic-datum-552",
	"winstrap":               "999119582588",
	"gobuilder":              "999119582588", // deprecated
	"golang":                 "999119582588",
}

func buildGoTarget() {
	target := strings.TrimPrefix(*file, "go:")
	var goos, goarch string
	if *osarch != "" {
		*osarch = strings.TrimSuffix(*osarch, ".gz")
		*osarch = (*osarch)[strings.LastIndex(*osarch, ".")+1:]
		v := strings.Split(*osarch, "-")
		if len(v) == 3 {
			v = v[:2] // support e.g. "linux-arm-scaleway" as GOOS=linux, GOARCH=arm
		}
		if len(v) != 2 || v[0] == "" || v[1] == "" {
			log.Fatalf("invalid -osarch value %q", *osarch)
		}
		goos, goarch = v[0], v[1]
	}

	env := append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch)
	if *extraEnv != "" {
		env = append(env, strings.Split(*extraEnv, ",")...)
	}
	env = envutil.Dedup(runtime.GOOS == "windows", env)
	cmd := exec.Command("go",
		"list",
		"--tags="+*tags,
		"--installsuffix="+*installSuffix,
		"-f", "{{.Target}}",
		target)
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		log.Fatalf("go list: %v", err)
	}
	outFile := string(bytes.TrimSpace(out))
	fi0, err := os.Stat(outFile)
	if os.IsNotExist(err) {
		if *verbose {
			log.Printf("File %s doesn't exist; building...", outFile)
		}
	}

	version := os.Getenv("USER") + "-" + time.Now().Format(time.RFC3339)
	ldflags := "-X main.Version=" + version
	if *static {
		ldflags = "-linkmode=external -extldflags '-static -pthread' " + ldflags
	}
	cmd = exec.Command("go",
		"install",
		"--tags="+*tags,
		"--installsuffix="+*installSuffix,
		"-x",
		"--ldflags="+ldflags,
		target)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if *verbose {
		cmd.Stdout = os.Stdout
	}
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		log.Fatalf("go install %s: %v, %s", target, err, stderr.Bytes())
	}

	fi1, err := os.Stat(outFile)
	if err != nil {
		log.Fatalf("Expected output file %s stat failure after go install %v: %v", outFile, target, err)
	}
	if !os.SameFile(fi0, fi1) {
		if *verbose {
			log.Printf("File %s rebuilt.", outFile)
		}
	}
	*file = outFile
}

// alreadyUploaded reports whether *file has already been uploaded and the correct contents
// are on cloud storage already.
func alreadyUploaded(storageClient *storage.Client, bucket, object string) bool {
	if *file == "-" {
		return false // don't know.
	}
	o, err := storageClient.Bucket(bucket).Object(object).Attrs(context.Background())
	if err == storage.ErrObjectNotExist {
		return false
	}
	if err != nil {
		log.Printf("Warning: stat failure: %v", err)
		return false
	}
	m5 := md5.New()
	fi, err := os.Stat(*file)
	if err != nil {
		log.Fatal(err)
	}
	if fi.Size() != o.Size {
		return false
	}
	f, err := os.Open(*file)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	n, err := io.Copy(m5, f)
	if err != nil {
		log.Fatal(err)
	}
	if n != fi.Size() {
		log.Printf("Warning: file size of %v changed", *file)
	}
	return bytes.Equal(m5.Sum(nil), o.MD5)
}

// generate14Tarfile downloads the release-branch.go1.4 release branch
// tarball and returns it uncompressed, with the "go/" prefix before
// each tar header's filename.
func generate14Tarfile() io.Reader {
	const tarURL = "https://go.googlesource.com/go/+archive/release-branch.go1.4.tar.gz"
	res, err := http.Get(tarURL)
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode != 200 {
		log.Fatalf("%v: %v", tarURL, res.Status)
	}
	if got, want := res.Header.Get("Content-Type"), "application/x-gzip"; got != want {
		log.Fatalf("%v: response Content-Type = %q; expected %q", tarURL, got, want)
	}

	var out bytes.Buffer // output tar (not gzipped)

	tw := tar.NewWriter(&out)

	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	tr := tar.NewReader(zr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		switch hdr.Typeflag {
		case tar.TypeReg, tar.TypeRegA, tar.TypeSymlink, tar.TypeDir:
			// Accept these.
		default:
			continue
		}
		hdr.Name = "go/" + hdr.Name
		if err := tw.WriteHeader(hdr); err != nil {
			log.Fatalf("WriteHeader: %v", err)
		}
		if _, err := io.Copy(tw, tr); err != nil {
			log.Fatalf("tar copying %v: %v", hdr.Name, err)
		}
	}
	if err := tw.Close(); err != nil {
		log.Fatal(err)
	}
	return &out
}
