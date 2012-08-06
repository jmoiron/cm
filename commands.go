package main

import (
	_ "errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// turn a path into a list of files.  if the path denotes a file, the list
// contains only that file.  if it contains a directory, a list of that
// directory is returned.  all paths are absolute.
func pathToFiles(path string) ([]string, error) {
	files := []string{}
	if isFile(path) {
		files = append(files, path)
	} else {
		stat, err := os.Stat(path)
		if err != nil {
			return files, err
		}
		if stat.IsDir() {
			fileinfos, err := ioutil.ReadDir(path)
			if err != nil {
				return files, err
			}
			for _, fi := range fileinfos {
				if !fi.IsDir() {
					files = append(files, filepath.Join(path, fi.Name()))
				} else {
					more, err := pathToFiles(filepath.Join(path, fi.Name()))
					if err == nil {
						newf := make([]string, len(files)+len(more))
						copy(newf, files)
						copy(newf[len(files):], more)
						files = newf
					}
				}
			}
		}
	}
	return files, nil
}

// implement the Add command, handling one path at a time.  if the path is
// a directory, then copy all of the files over.
func Add(path string) error {
	var cfg string
	files := []string{}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	files, err = pathToFiles(abs)
	if err != nil {
		return err
	}

	for _, file := range files {
		cfg = C(file)
		err = Copy(cfg, file)
		if err == nil {
			fmt.Printf("%s -> %s\n", file, cfg)
		} else {
			fmt.Fprintf(os.Stderr, "%s could not be added: %s\n", file, err.Error())
		}
	}
	return nil
}

// implement the `rm` command, handling one path at a time.  the `rm` command 
// will remove files from the overlay not present in the actual working 
// filesystem.
func Rm(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	dst := C(abs)
	err = os.RemoveAll(dst)
	if err == nil {
		fmt.Printf("Removed %s\n", abs)
	} else {
		fmt.Fprintf(os.Stderr, "%s could not be removed: %s\n", abs, err.Error())
		return err
	}
	return nil
}

// `show` command, handling one path at a time, shows what files exist under
// the overlay's version of the path
func Show(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	dst := C(abs)
	cfgs, err := pathToFiles(dst)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Could not show %s: %s\n", dst, err.Error())
	} else if err != nil {
		return nil
	}

	for _, cfg := range cfgs {
		fmt.Println(X(cfg))
	}
	return nil
}

// `sync` command, copies file or directory from the overlay to the actual
// filesystem.  will never delete files on filesystem, but always overwrites
// existing ones.
func Sync(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	src := C(abs)
	cfgs, err := pathToFiles(src)
	if err != nil && os.IsNotExist(err) {
		return nil
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load files from %s: %s\n", src, err.Error())
		return err
	}

	for _, cfg := range cfgs {
		file := X(cfg)
		err = Copy(file, cfg)
		if err == nil {
			fmt.Printf("%s -> %s\n", cfg, file)
		} else {
			fmt.Fprintf(os.Stderr, "%s could not be synced: %s\n", file, err.Error())
		}
	}
	return nil
}

// `diff` command, diffs two files or two directories of files using the system
// diff program with '-u'.  this will only diff files which are present in the
// overlay, and the diff shows what would happen with a `sync`
func Diff(path string) error {
	var cmd *exec.Cmd
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	diff := os.Getenv("CM_DIFF")
	if len(diff) == 0 {
		// user colordiff if available, else diff
		diff, err = exec.LookPath("colordiff")
		if err != nil {
			diff, err = exec.LookPath("diff")
			if err != nil {
				return fmt.Errorf("Could not find suitable diff executable in your PATH.")
			}
		}
	}

	src := C(abs)
	cfgs, err := pathToFiles(src)

	if err != nil && os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Requested file not found: %s: %s\n", src, err.Error())
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Could not diff files from %s: %s\n", src, err.Error())
		return err
	}

	for _, cfg := range cfgs {
		cmd = exec.Command(diff, "-u", cfg, X(cfg))
		output, _ := cmd.Output()
		fmt.Print(string(output))
	}

	return nil
}
