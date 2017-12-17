package archiver

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Tar is for Tar format
var Tar tarFormat

func init() {
	RegisterFormat("Tar", Tar)
}

type tarFormat struct{}

func (tarFormat) Match(filename string) bool {
	return strings.HasSuffix(strings.ToLower(filename), ".tar") || isTar(filename)
}

const tarBlockSize int = 512

// isTar checks the file has the Tar format header by reading its beginning
// block.
func isTar(tarPath string) bool {
	f, err := os.Open(tarPath)
	if err != nil {
		return false
	}
	defer f.Close()

	buf := make([]byte, tarBlockSize)
	if _, err = io.ReadFull(f, buf); err != nil {
		return false
	}

	return hasTarHeader(buf)
}

// hasTarHeader checks passed bytes has a valid tar header or not. buf must
// contain at least 512 bytes and if not, it always returns false.
func hasTarHeader(buf []byte) bool {
	if len(buf) < tarBlockSize {
		return false
	}

	b := buf[148:156]
	b = bytes.Trim(b, " \x00") // clean up all spaces and null bytes
	if len(b) == 0 {
		return false // unknown format
	}
	hdrSum, err := strconv.ParseUint(string(b), 8, 64)
	if err != nil {
		return false
	}

	// According to the go official archive/tar, Sun tar uses signed byte
	// values so this calcs both signed and unsigned
	var usum uint64
	var sum int64
	for i, c := range buf {
		if 148 <= i && i < 156 {
			c = ' ' // checksum field itself is counted as branks
		}
		usum += uint64(uint8(c))
		sum += int64(int8(c))
	}

	if hdrSum != usum && int64(hdrSum) != sum {
		return false // invalid checksum
	}

	return true
}

// Write outputs a .tar file to a Writer containing the
// contents of files listed in filePaths. File paths can
// be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarFormat) Write(output io.Writer, filePaths []string) error {
	return writeTar(filePaths, output, "")
}

// Make creates a .tar file at tarPath containing the
// contents of files listed in filePaths. File paths can
// be those of regular files or directories. Regular
// files are stored at the 'root' of the archive, and
// directories are recursively added.
func (tarFormat) Make(tarPath string, filePaths []string) error {
	out, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("error creating %s: %v", tarPath, err)
	}
	defer out.Close()

	return writeTar(filePaths, out, tarPath)
}

func writeTar(filePaths []string, output io.Writer, dest string) error {
	tarWriter := tar.NewWriter(output)
	defer tarWriter.Close()

	return tarball(filePaths, tarWriter, dest)
}

// tarball writes all files listed in filePaths into tarWriter, which is
// writing into a file located at dest.
func tarball(filePaths []string, tarWriter *tar.Writer, dest string) error {
	for _, fpath := range filePaths {
		err := tarFile(tarWriter, fpath, dest)
		if err != nil {
			return err
		}
	}
	return nil
}

// tarFile writes the file at source into tarWriter. It does so
// recursively for directories.
func tarFile(tarWriter *tar.Writer, source, dest string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("%s: stat: %v", source, err)
	}

	var baseDir string
	if sourceInfo.IsDir() {
		baseDir = filepath.Base(source)
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking to %s: %v", path, err)
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("%s: making header: %v", path, err)
		}

		if baseDir != "" {
			header.Name = filepath.Join(baseDir, strings.TrimPrefix(path, source))
		}

		if header.Name == dest {
			// our new tar file is inside the directory being archived; skip it
			return nil
		}

		if info.IsDir() {
			header.Name += "/"
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("%s: writing header: %v", path, err)
		}

		if info.IsDir() {
			return nil
		}

		if header.Typeflag == tar.TypeReg {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("%s: open: %v", path, err)
			}
			defer file.Close()

			_, err = io.CopyN(tarWriter, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("%s: copying contents: %v", path, err)
			}
		}
		return nil
	})
}

// Read untars a .tar file read from a Reader and puts
// the contents into destination.
func (tarFormat) Read(input io.Reader, destination string) error {
	return untar(tar.NewReader(input), destination)
}

// Open untars source and puts the contents into destination.
func (tarFormat) Open(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("%s: failed to open archive: %v", source, err)
	}
	defer f.Close()

	return Tar.Read(f, destination)
}

// untar un-tarballs the contents of tr into destination.
func untar(tr *tar.Reader, destination string) error {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		if err := untarFile(tr, header, destination); err != nil {
			return err
		}
	}
	return nil
}

// untarFile untars a single file from tr with header header into destination.
func untarFile(tr *tar.Reader, header *tar.Header, destination string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return mkdir(filepath.Join(destination, header.Name))
	case tar.TypeReg, tar.TypeRegA, tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
		return writeNewFile(filepath.Join(destination, header.Name), tr, header.FileInfo().Mode())
	case tar.TypeSymlink:
		return writeNewSymbolicLink(filepath.Join(destination, header.Name), header.Linkname)
	case tar.TypeLink:
		return writeNewHardLink(filepath.Join(destination, header.Name), filepath.Join(destination, header.Linkname))
	default:
		return fmt.Errorf("%s: unknown type flag: %c", header.Name, header.Typeflag)
	}
}
