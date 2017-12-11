package archiver

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nwaples/rardecode"
)

// Rar is for RAR archive format
var Rar rarFormat

func init() {
	RegisterFormat("Rar", Rar)
}

type rarFormat struct{}

func (rarFormat) Match(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".rar") || isRar(filename)
}

// isRar checks the file has the RAR 1.5 or 5.0 format signature by reading its
// beginning bytes and matching it
func isRar(rarPath string) bool {
	f, err := os.Open(rarPath)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, 8)
	if n, err := f.Read(buf); err != nil || n < 8 {
		return false
	}

	return bytes.Equal(buf[:7], []byte("Rar!\x1a\x07\x00")) || // ver 1.5
		bytes.Equal(buf, []byte("Rar!\x1a\x07\x01\x00")) // ver 5.0
}

// Write outputs a .rar archive, but this is not implemented because
// RAR is a proprietary format. It is here only for symmetry with
// the other archive formats in this package.
func (rarFormat) Write(output io.Writer, filePaths []string) error {
	return fmt.Errorf("write: RAR not implemented (proprietary format)")
}

// Make makes a .rar archive, but this is not implemented because
// RAR is a proprietary format. It is here only for symmetry with
// the other archive formats in this package.
func (rarFormat) Make(rarPath string, filePaths []string) error {
	return fmt.Errorf("make %s: RAR not implemented (proprietary format)", rarPath)
}

// Read extracts the RAR file read from input and puts the contents
// into destination.
func (rarFormat) Read(input io.Reader, destination string) error {
	rr, err := rardecode.NewReader(input, "")
	if err != nil {
		return fmt.Errorf("read: failed to create reader: %v", err)
	}

	for {
		header, err := rr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if header.IsDir {
			err = mkdir(filepath.Join(destination, header.Name))
			if err != nil {
				return err
			}
			continue
		}

		// if files come before their containing folders, then we must
		// create their folders before writing the file
		err = mkdir(filepath.Dir(filepath.Join(destination, header.Name)))
		if err != nil {
			return err
		}

		err = writeNewFile(filepath.Join(destination, header.Name), rr, header.Mode())
		if err != nil {
			return err
		}
	}

	return nil
}

// Open extracts the RAR file at source and puts the contents
// into destination.
func (rarFormat) Open(source, destination string) error {
	rf, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open file: %v", source, err)
	}
	defer rf.Close()

	return Rar.Read(rf, destination)
}
