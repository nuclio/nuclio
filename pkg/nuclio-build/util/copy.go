package util

import (
    "fmt"
    "io"
    "io/ioutil"
    "os"
    "path/filepath"
)

// Copies file source to destination dest.
func CopyFile(source string, dest string) (err error) {
    sf, err := os.Open(source)
    if err != nil {
        return err
    }
    defer sf.Close()
    df, err := os.Create(dest)
    if err != nil {
        return err
    }
    defer df.Close()
    _, err = io.Copy(df, sf)
    if err == nil {
        si, err := sf.Stat()
        if err == nil {
            err = os.Chmod(dest, si.Mode())
        }
    }

    return
}

// Recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must *not* exist.
func CopyDir(source string, dest string) (err error) {

    // get properties of source dir
    fi, err := os.Stat(source)
    if err != nil {
        return
    }

    if !fi.IsDir() {
        return &CustomError{"Source is not a directory"}
    }

    // ensure dest dir does not already exist

    _, err = os.Open(dest)
    if !os.IsNotExist(err) {
        return &CustomError{fmt.Sprintf("Destination already exists: %s", dest)}
    }

    // create dest dir

    err = os.MkdirAll(dest, fi.Mode())
    if err != nil {
        return
    }

    entries, err := ioutil.ReadDir(source)
    if err != nil {
        return
    }
    for _, entry := range entries {

        sfp := filepath.Join(source, entry.Name())
        dfp := filepath.Join(dest, entry.Name())
        if entry.IsDir() {
            err = CopyDir(sfp, dfp)
            if err != nil {
                return
            }
        } else {
            // perform copy
            err = CopyFile(sfp, dfp)
            if err != nil {
                return
            }
        }

    }
    return
}

// A struct for returning custom error messages
type CustomError struct {
    What string
}

// Returns the error message defined in What as a string
func (e *CustomError) Error() string {
    return e.What
}
