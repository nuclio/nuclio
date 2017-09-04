package common

import (
	"io"
	"net/http"
	"os"
)

func DownloadFile(url, dest string) error {
	out, err := os.OpenFile(dest, os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, err = io.Copy(out, response.Body)
	if err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return nil
}
