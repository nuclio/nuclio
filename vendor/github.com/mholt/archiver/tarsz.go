package archiver

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/golang/snappy"
)

// TarSz is for TarSz format
var TarSz tarSzFormat

func init() {
	RegisterFormat("TarSz", TarSz)
}

type tarSzFormat struct{}

func (tarSzFormat) Match(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".tar.sz") || strings.HasSuffix(strings.ToLower(filename), ".tsz") || isTarSz(filename)
}

// isTarSz checks the file has the sz compressed Tar format header by
// reading its beginning block.
func isTarSz(tarszPath string) bool {
	f, err := os.Open(tarszPath)
	if err != nil {
		return false
	}
	defer f.Close()

	szr := snappy.NewReader(f)
	buf := make([]byte, tarBlockSize)
	n, err := szr.Read(buf)
	if err != nil || n < tarBlockSize {
		return false
	}

	return hasTarHeader(buf)
}

// Write outputs a .tar.sz file to a Writer containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarSzFormat) Write(output io.Writer, filePaths []string) error {
	return writeTarSz(filePaths, output, "")
}

// Make creates a .tar.sz file at tarszPath containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarSzFormat) Make(tarszPath string, filePaths []string) error {
	out, err := os.Create(tarszPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", tarszPath, err)
	}
	defer out.Close()

	return writeTarSz(filePaths, out, tarszPath)
}

func writeTarSz(filePaths []string, output io.Writer, dest string) error {
	szw := snappy.NewBufferedWriter(output)
	defer szw.Close()

	return writeTar(filePaths, szw, dest)
}

// Read untars a .tar.sz file read from a Reader and decompresses
// the contents into destination.
func (tarSzFormat) Read(input io.Reader, destination string) error {
	szr := snappy.NewReader(input)

	return Tar.Read(szr, destination)
}

// Open untars source and decompresses the contents into destination.
func (tarSzFormat) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	return TarSz.Read(f, destination)
}
