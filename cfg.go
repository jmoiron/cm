package main

// Configuration management part of cm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	CONFIG_ROOT      = "/"
	CONFIG_PATH      = "/opt/cm"
	UNSAFE_SYNC      = false
	SYNC_SYMLINK     = false
	CONFIG_PATH_MODE = os.FileMode(0755)
)

// return the overlay version of a path
func C(path string) string { return filepath.Join(CONFIG_PATH, path) }

// return the non-overlay version of a path
func X(path string) string {
	if strings.HasPrefix(path, CONFIG_PATH) {
		return CONFIG_ROOT + path[len(CONFIG_PATH):]
	}
	return path
}

// Strip the root from path.  If root does not exist, return an error
func StripRoot(path string) (string, error) {
	if !strings.HasPrefix(path, CONFIG_ROOT) {
		return "", fmt.Errorf("Path %s not in CONFIG_ROOT %s", path, CONFIG_ROOT)
	}
	return path[len(CONFIG_ROOT):], nil

}

// copy the file `dst` to the file `src`, creating any direcsrcries necessary
// try to preserve atime/mtime
func Copy(dst, src string) error {
	if !isFile(src) {
		return fmt.Errorf("Source path %s must be a file.", src)
	}
	ss, _ := os.Stat(src)
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()
	dir := filepath.Dir(dst)
	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	d, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, ss.Mode())
	if err != nil {
		return err
	}
	defer d.Close()
	_, err = io.Copy(d, s)

	if err != nil {
		return err
	}
	// copy modification time;  use the mtime as an atime
	err = os.Chtimes(dst, ss.ModTime(), ss.ModTime())
	return err
}

// Quick diff.  If the modtime and the size is the same, we assume that the
// files are the same.  Otherwise, we assume that they are different or at
// least require further checking.
func QuickDiff(path1, path2 string) bool {
	fi1, err1 := os.Stat(path1)
	fi2, err2 := os.Stat(path2)
	if err1 != err2 {
		return false
	}
	if fi1.Size() != fi2.Size() {
		return false
	}
	if fi1.ModTime() != fi2.ModTime() {
		return false
	}
	return true
}

// return true if path exists and is a file, false otherwise
func isFile(path string) bool {
	fi, err := os.Stat(path)
	if err == nil && !fi.IsDir() {
		return true
	}
	if err != nil && !os.IsNotExist(err) {
		fmt.Printf("%s\n", err.Error())
	}
	return false
}

// initialize config path if not present, return an error if either the dir
// exists and is not writable or the dir does not exist and cannot be created
func configPathInit() error {
	fi, err := os.Stat(CONFIG_PATH)
	// if it doesn't exist
	if err != nil && os.IsNotExist(err) {
		err = os.MkdirAll(CONFIG_PATH, CONFIG_PATH_MODE)
		return err
		// if it isn't a dir or symlink
	} else if !fi.IsDir() && (fi.Mode()&os.ModeSymlink) == 0 {
		return fmt.Errorf("Path %s must be directory or symlink.", CONFIG_PATH)
	}
	// if it exists, check for writability
	for i := 0; ; i++ {
		path := C(fmt.Sprintf("__testfile%d", i))
		if isFile(path) {
			continue
		}
		_, err := os.Create(path)
		if err == nil {
			os.Remove(path)
			return nil
		}
		if os.IsPermission(err) {
			return fmt.Errorf("Path %s must be writable.", CONFIG_PATH)
		}
		return err
	}
	return errors.New("Unknown error setting up config path.")
}

func init() {
	conf := os.Getenv("CM_CONFIG_PATH")
	if len(conf) > 0 {
		CONFIG_PATH = conf
	}

	conf = os.Getenv("CM_UNSAFE_SYNC")
	if len(conf) > 0 && conf != "0" {
		UNSAFE_SYNC = true
	}

	conf = os.Getenv("CM_SYNC_SYMLINK")
	if len(conf) > 0 && conf != "0" {
		SYNC_SYMLINK = true
	}

	conf = os.Getenv("CM_CONFIG_ROOT")
	if len(conf) > 0 {
		CONFIG_ROOT = conf
	}
}
