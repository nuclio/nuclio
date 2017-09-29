package common

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func DownloadFile(URL, destFile string) error {
	out, err := os.OpenFile(destFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	response, err := http.Get(URL)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	written, err := io.Copy(out, response.Body)
	if err != nil {
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if response.ContentLength != -1 && written != response.ContentLength {
		return fmt.Errorf(
			"Downloaded file length (%d) is different than URL content length (%d)",
			written,
			response.ContentLength)
	}
	return nil
}

func IsURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
