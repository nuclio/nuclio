package archiver

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/pierrec/lz4"
)

// TarLz4 is for TarLz4 format
var TarLz4 tarLz4Format

func init() {
	RegisterFormat("TarLz4", TarLz4)
}

type tarLz4Format struct{}

func (tarLz4Format) Match(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".tar.lz4") || strings.HasSuffix(strings.ToLower(filename), ".tlz4") || isTarLz4(filename)
}

// isTarLz4 checks the file has the lz4 compressed Tar format header by
// reading its beginning block.
func isTarLz4(tarlz4Path string) bool {
	f, err := os.Open(tarlz4Path)
	if err != nil {
		return false
	}
	defer f.Close()

	lz4r := lz4.NewReader(f)
	buf := make([]byte, tarBlockSize)
	n, err := lz4r.Read(buf)
	if err != nil || n < tarBlockSize {
		return false
	}

	return hasTarHeader(buf)
}

// Write outputs a .tar.lz4 file to a Writer containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarLz4Format) Write(output io.Writer, filePaths []string) error {
	return writeTarLz4(filePaths, output, "")
}

// Make creates a .tar.lz4 file at tarlz4Path containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarLz4Format) Make(tarlz4Path string, filePaths []string) error {
	out, err := os.Create(tarlz4Path)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", tarlz4Path, err)
	}
	defer out.Close()

	return writeTarLz4(filePaths, out, tarlz4Path)
}

func writeTarLz4(filePaths []string, output io.Writer, dest string) error {
	lz4w := lz4.NewWriter(output)
	defer lz4w.Close()

	return writeTar(filePaths, lz4w, dest)
}

// Read untars a .tar.xz file read from a Reader and decompresses
// the contents into destination.
func (tarLz4Format) Read(input io.Reader, destination string) error {
	lz4r := lz4.NewReader(input)

	return Tar.Read(lz4r, destination)
}

// Open untars source and decompresses the contents into destination.
func (tarLz4Format) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	return TarLz4.Read(f, destination)
}
