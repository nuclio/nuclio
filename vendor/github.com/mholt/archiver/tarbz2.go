package archiver

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dsnet/compress/bzip2"
)

// TarBz2 is for TarBz2 format
var TarBz2 tarBz2Format

func init() {
	RegisterFormat("TarBz2", TarBz2)
}

type tarBz2Format struct{}

func (tarBz2Format) Match(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".tar.bz2") ||
		strings.HasSuffix(strings.ToLower(filename), ".tbz2") ||
		isTarBz2(filename)
}

// isTarBz2 checks the file has the bzip2 compressed Tar format header by
// reading its beginning block.
func isTarBz2(tarbz2Path string) bool {
	f, err := os.Open(tarbz2Path)
	if err != nil {
		return false
	}
	defer f.Close()

	bz2r, err := bzip2.NewReader(f, nil)
	if err != nil {
		return false
	}
	defer bz2r.Close()

	buf := make([]byte, tarBlockSize)
	n, err := bz2r.Read(buf)
	if err != nil || n < tarBlockSize {
		return false
	}

	return hasTarHeader(buf)
}

// Write outputs a .tar.bz2 file to a Writer containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarBz2Format) Write(output io.Writer, filePaths []string) error {
	return writeTarBz2(filePaths, output, "")
}

// Make creates a .tar.bz2 file at tarbz2Path containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarBz2Format) Make(tarbz2Path string, filePaths []string) error {
	out, err := os.Create(tarbz2Path)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", tarbz2Path, err)
	}
	defer out.Close()

	return writeTarBz2(filePaths, out, tarbz2Path)
}

func writeTarBz2(filePaths []string, output io.Writer, dest string) error {
	bz2w, err := bzip2.NewWriter(output, nil)
	if err != nil {
		return fmt.Errorf("error compressing bzip2: %v", err)
	}
	defer bz2w.Close()

	return writeTar(filePaths, bz2w, dest)
}

// Read untars a .tar.bz2 file read from a Reader and decompresses
// the contents into destination.
func (tarBz2Format) Read(input io.Reader, destination string) error {
	bz2r, err := bzip2.NewReader(input, nil)
	if err != nil {
		return fmt.Errorf("error decompressing bzip2: %v", err)
	}
	defer bz2r.Close()

	return Tar.Read(bz2r, destination)
}

// Open untars source and decompresses the contents into destination.
func (tarBz2Format) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	return TarBz2.Read(f, destination)
}
