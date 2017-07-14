//////////////////////////////////////////
// archivex.go
// Jhonathan Paulo Banczek - 2014
// jpbanczek@gmail.com - jhoonb.com
//////////////////////////////////////////

package archivex

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// interface
type Archivex interface {
	Create(name string) error
	Add(name string, file []byte) error
	AddFile(name string) error
	AddAll(dir string, includeCurrentFolder bool) error
	Close() error
}

// ArchiveWriteFunc is the closure used by an archive's AddAll method to actually put a file into an archive
// Note that for directory entries, this func will be called with a nil 'file' param
type ArchiveWriteFunc func(info os.FileInfo, file io.Reader, entryName string) (err error)

// ZipFile implement *zip.Writer
type ZipFile struct {
	Writer *zip.Writer
	Name   string
	file   *os.File
}

// TarFile implement *tar.Writer
type TarFile struct {
	Writer     *tar.Writer
	Name       string
	GzWriter   *gzip.Writer
	Compressed bool
}

func (z *ZipFile) createWriter(name string) (io.Writer, error) {
	header := &zip.FileHeader{
		Name:   name,
		Flags:  1 << 11, // use utf8 encoding the file Name
		Method: zip.Deflate,
	}

	return z.Writer.CreateHeader(header)
}

// Create new file zip
func (z *ZipFile) Create(name string) error {
	// check extension .zip
	if strings.HasSuffix(name, ".zip") != true {
		if strings.HasSuffix(name, ".tar.gz") == true {
			name = strings.Replace(name, ".tar.gz", ".zip", -1)
		} else {
			name = name + ".zip"
		}
	}
	z.Name = name
	file, err := os.Create(z.Name)
	if err != nil {
		return err
	}
	z.Writer = zip.NewWriter(file)
	z.file = file
	return nil
}

// Add add byte in archive zip
func (z *ZipFile) Add(name string, file []byte) error {
	iow, err := z.createWriter(name)
	if err != nil {
		return err
	}

	_, err = iow.Write(file)

	return err
}

// AddFile add file from dir in archive
func (z *ZipFile) AddFile(name string) error {
	zippedFile, err := z.createWriter(name)
	if err != nil {
		return err
	}

	file, _ := os.Open(filepath.Join(name))
	fileReader := bufio.NewReader(file)

	blockSize := 512 * 1024 // 512kb
	bytes := make([]byte, blockSize)

	for {
		readedBytes, err := fileReader.Read(bytes)

		if err != nil {
			if err.Error() == "EOF" {
				break
			}

			if err.Error() != "EOF" {
				return err
			}
		}

		if readedBytes >= blockSize {
			zippedFile.Write(bytes)
			continue
		}

		zippedFile.Write(bytes[:readedBytes])
	}

	return nil
}

//AddFileWithName add a file to zip with a name
func (z *ZipFile) AddFileWithName(name string, filepath string) error {
	zippedFile, err := z.createWriter(name)
	if err != nil {
		return err
	}

	file, e := os.Open(filepath)
	defer file.Close()
	if e != nil {
		return e
	}
	fileReader := bufio.NewReader(file)

	blockSize := 512 * 1024 // 512kb
	bytes := make([]byte, blockSize)

	for {
		readedBytes, err := fileReader.Read(bytes)

		if err != nil {
			if err.Error() == "EOF" {
				break
			}

			if err.Error() != "EOF" {
				return err
			}
		}

		if readedBytes >= blockSize {
			zippedFile.Write(bytes)
			continue
		}

		zippedFile.Write(bytes[:readedBytes])
	}

	return nil
}

// AddAll adds all files from dir in archive, recursively.
// Directories receive a zero-size entry in the archive, with a trailing slash in the header name, and no compression
func (z *ZipFile) AddAll(dir string, includeCurrentFolder bool) error {
	dir = path.Clean(dir)
	return addAll(dir, dir, includeCurrentFolder, func(info os.FileInfo, file io.Reader, entryName string) (err error) {

		// Create a header based off of the fileinfo
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// If it's a file, set the compression method to deflate (leave directories uncompressed)
		if !info.IsDir() {
			header.Method = zip.Deflate
		}

		// Set the header's name to what we want--it may not include the top folder
		header.Name = entryName

		// Add a trailing slash if the entry is a directory
		if info.IsDir() {
			header.Name += "/"
		}

		// Get a writer in the archive based on our header
		writer, err := z.Writer.CreateHeader(header)
		if err != nil {
			return err
		}

		// If we have a file to write (i.e., not a directory) then pipe the file into the archive writer
		if file != nil {
			if _, err := io.Copy(writer, file); err != nil {
				return err
			}
		}

		return nil
	})
}

//Close close the zip file
func (z *ZipFile) Close() error {
	err := z.Writer.Close()
	z.file.Close()
	return err
}

// Create new Tar file
func (t *TarFile) Create(name string) error {
	// check the filename extension

	// if it has a .gz, we'll compress it.
	if strings.HasSuffix(name, ".tar.gz") {
		t.Compressed = true
	} else {
		t.Compressed = false
	}

	// check to see if they have the wrong extension
	if strings.HasSuffix(name, ".tar.gz") != true && strings.HasSuffix(name, ".tar") != true {
		// is it .zip? replace it
		if strings.HasSuffix(name, ".zip") == true {
			name = strings.Replace(name, ".zip", ".tar.gz", -1)
			t.Compressed = true
		} else {
			// if it's not, add .tar
			// since we'll assume it's not compressed
			name = name + ".tar"
		}
	}

	t.Name = name
	file, err := os.Create(t.Name)
	if err != nil {
		return err
	}

	if t.Compressed {
		t.GzWriter = gzip.NewWriter(file)
		t.Writer = tar.NewWriter(t.GzWriter)
	} else {
		t.Writer = tar.NewWriter(file)
	}

	return nil
}

// Add add byte in archive tar
func (t *TarFile) Add(name string, file []byte) error {

	hdr := &tar.Header{
		Name:    name,
		Size:    int64(len(file)),
		Mode:    0666,
		ModTime: time.Now(),
	}
	if err := t.Writer.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := t.Writer.Write(file)
	return err
}

// Add add byte in archive tar
func (t *TarFile) AddWithHeader(name string, file []byte, hdr *tar.Header) error {

	if err := t.Writer.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := t.Writer.Write(file)
	return err
}

// AddFile add file from dir in archive tar
func (t *TarFile) AddFile(name string) error {
	bytearq, err := ioutil.ReadFile(name)
	if err != nil {
		return err
	}

	info, err := os.Stat(name)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	err = t.Writer.WriteHeader(header)
	if err != nil {
		return err
	}
	_, err = t.Writer.Write(bytearq)
	if err != nil {
		return err
	}
	return nil
}

// AddFile add file from dir in archive tar
func (t *TarFile) AddFileWithName(name string, filename string) error {
	bytearq, err := ioutil.ReadFile(name)
	if err != nil {
		return err
	}

	info, err := os.Stat(name)
	if err != nil {
		return err
	}

	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filename

	err = t.Writer.WriteHeader(header)
	if err != nil {
		return err
	}
	_, err = t.Writer.Write(bytearq)
	if err != nil {
		return err
	}
	return nil
}

// AddAll adds all files from dir in archive
// Tar does not support directories
func (t *TarFile) AddAll(dir string, includeCurrentFolder bool) error {
	dir = path.Clean(dir)
	return addAll(dir, dir, includeCurrentFolder, func(info os.FileInfo, file io.Reader, entryName string) (err error) {

		// Create a header based off of the fileinfo
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Set the header's name to what we want--it may not include the top folder
		header.Name = entryName

		// Write the header into the tar file
		if err := t.Writer.WriteHeader(header); err != nil {
			return err
		}

		// The directory don't need copy file
		if file == nil {
			return nil
		}

		// Pipe the file into the tar
		if _, err := io.Copy(t.Writer, file); err != nil {
			return err
		}

		return nil
	})
}

// Close the file Tar
func (t *TarFile) Close() error {
	err := t.Writer.Close()
	if err != nil {
		return err
	}

	if t.Compressed {
		err = t.GzWriter.Close()
		if err != nil {
			return err
		}
	}

	return err
}

func getSubDir(dir string, rootDir string, includeCurrentFolder bool) (subDir string) {

	subDir = strings.Replace(dir, rootDir, "", 1)
	// Remove leading slashes, since this is intentionally a subdirectory.
	if len(subDir) > 0 && subDir[0] == os.PathSeparator {
		subDir = subDir[1:]
	}
	subDir = path.Join(strings.Split(subDir, string(os.PathSeparator))...)

	if includeCurrentFolder {
		parts := strings.Split(rootDir, string(os.PathSeparator))
		subDir = path.Join(parts[len(parts)-1], subDir)
	}

	return
}

// addAll is used to recursively go down through directories and add each file and directory to an archive, based on an ArchiveWriteFunc given to it
func addAll(dir string, rootDir string, includeCurrentFolder bool, writerFunc ArchiveWriteFunc) error {

	// Get a list of all entries in the directory, as []os.FileInfo
	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	// Loop through all entries
	for _, info := range fileInfos {

		full := filepath.Join(dir, info.Name())

		// If the entry is a file, get an io.Reader for it
		var file *os.File
		var reader io.Reader
		if !info.IsDir() {
			file, err = os.Open(full)
			if err != nil {
				return err
			}
			reader = file
		}

		// Write the entry into the archive
		subDir := getSubDir(dir, rootDir, includeCurrentFolder)
		entryName := path.Join(subDir, info.Name())
		if err := writerFunc(info, reader, entryName); err != nil {
			if file != nil {
				file.Close()
			}
			return err
		}

		if file != nil {
			if err := file.Close(); err != nil {
				return err
			}

		}

		// If the entry is a directory, recurse into it
		if info.IsDir() {
			addAll(full, rootDir, includeCurrentFolder, writerFunc)
		}
	}

	return nil
}
