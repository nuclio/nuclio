package archiver

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ulikunitz/xz"
)

// TarXZ is for TarXZ format
var TarXZ xzFormat

func init() {
	RegisterFormat("TarXZ", TarXZ)
}

type xzFormat struct{}

// Match returns whether filename matches this format.
func (xzFormat) Match(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".tar.xz") ||
		strings.HasSuffix(strings.ToLower(filename), ".txz") ||
		isTarXz(filename)
}

// isTarXz checks the file has the xz compressed Tar format header by reading
// its beginning block.
func isTarXz(tarxzPath string) bool {
	f, err := os.Open(tarxzPath)
	if err != nil {
		return false
	}
	defer f.Close()

	xzr, err := xz.NewReader(f)
	if err != nil {
		return false
	}

	buf := make([]byte, tarBlockSize)
	n, err := xzr.Read(buf)
	if err != nil || n < tarBlockSize {
		return false
	}

	return hasTarHeader(buf)
}

// Write outputs a .tar.xz file to a Writer containing
// the contents of files listed in filePaths. File paths
// can be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (xzFormat) Write(output io.Writer, filePaths []string) error {
	return writeTarXZ(filePaths, output, "")
}

// Make creates a .tar.xz file at xzPath containing
// the contents of files listed in filePaths. File
// paths can be those of regular files or directories.
// Regular files are stored at the 'root' of the
// archive, and directories are recursively added.
func (xzFormat) Make(xzPath string, filePaths []string) error {
	out, err := os.Create(xzPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", xzPath, err)
	}
	defer out.Close()

	return writeTarXZ(filePaths, out, xzPath)
}

func writeTarXZ(filePaths []string, output io.Writer, dest string) error {
	xzw, err := xz.NewWriter(output)
	if err != nil {
		return fmt.Errorf("error compressing xz: %v", err)
	}
	defer xzw.Close()

	return writeTar(filePaths, xzw, dest)
}

// Read untars a .tar.xz file read from a Reader and decompresses
// the contents into destination.
func (xzFormat) Read(input io.Reader, destination string) error {
	xzr, err := xz.NewReader(input)
	if err != nil {
		return fmt.Errorf("error decompressing xz: %v", err)
	}

	return Tar.Read(xzr, destination)
}

// Open untars source and decompresses the contents into destination.
func (xzFormat) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	return TarXZ.Read(f, destination)
}
