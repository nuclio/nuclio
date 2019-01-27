package common

import (
	"archive/zip"
	"io/ioutil"
)

func GetZipFileContents(zf *zip.File) (string, error) {
	unzippedFileBytes, err := readZipFile(zf)
	if err != nil {
		return "", err
	}

	return string(unzippedFileBytes), nil
}

func readZipFile(zf *zip.File) ([]byte, error) {
	f, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint: errcheck
	return ioutil.ReadAll(f)
}
