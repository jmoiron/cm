package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	DIFF     = ""
	CM_DIFF  = ""
	DIFFSTAT = ""
)

// Diff two files.  If forceNormal is True, custom diffs configured in the
// the environ are ignored for the regular `diff` command.
func diff(src, dest string, forceNormal ...bool) (*exec.Cmd, error) {
	var err error

	if len(DIFF) == 0 {
		CM_DIFF = os.Getenv("CM_DIFF")
		DIFF, _ = exec.LookPath("diff")
		if len(CM_DIFF) == 0 {
			CM_DIFF, err = exec.LookPath("colordiff")
			if err != nil {
				CM_DIFF = DIFF
			}
		}
	}

	if len(CM_DIFF) == 0 {
		return nil, errors.New("Could not find suitable diff executable in your PATH.")
	}

	if len(forceNormal) > 0 && forceNormal[0] {
		return exec.Command(DIFF, "-Nu", src, dest), nil
	}
	return exec.Command(CM_DIFF, "-Nu", src, dest), nil
}

// Runs diffstat on some diff output.  Returns diffstat command.
func diffstat(output string) (*exec.Cmd, error) {
	var err error
	var cmd *exec.Cmd

	if len(DIFFSTAT) == 0 {
		DIFFSTAT, _ = exec.LookPath("diffstat")
		if len(DIFFSTAT) == 0 {
			return nil, errors.New("Could not find suitable diffstat executable in your PATH.")
		}
	}

	if runtime.GOOS == "darwin" {
		cmd = exec.Command(DIFFSTAT)
	} else {
		cmd = exec.Command(DIFFSTAT, "-C")
	}
	if err != nil {
		return nil, err
	}

	cmd.Stdin = strings.NewReader(output)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

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
	var cfg, dest string
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
		// strip out the CONFIG ROOT for the file;  if it isn't inside the CONFIG_ROOT,
		// then it isn't valid.
		dest, err = StripRoot(file)
		if err != nil {
			return err
		}
		cfg = C(dest)
		err = Copy(cfg, file)
		if err != nil {
			return err
		}

		fmt.Printf("%s -> %s\n", file, cfg)
	}
	return nil
}

// implement the `rm` command, handling one path at a time.  the `rm` command
// will remove files from the overlay not present in the actual working
// filesystem.
func Rm(path string) error {
	var err error
	var abs, dest string

	abs, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	dest, err = StripRoot(abs)
	if err != nil {
		return err
	}

	path = C(dest)
	err = os.RemoveAll(path)
	if err == nil {
		fmt.Printf("Removed %s\n", abs)
	} else {
		fmt.Fprintf(os.Stderr, "%s could not be removed: %s\n", abs, err.Error())
		return err
	}
	return nil
}

// `list` command, handling one path at a time, shows what files exist under
// the overlay's version of the path
func List(path string) error {
	var err error
	var abs, dest string

	abs, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	dest, err = StripRoot(abs)
	if err != nil {
		return err
	}
	path = C(dest)
	cfgs, err := pathToFiles(path)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Could not show %s: %s\n", path, err.Error())
	} else if err != nil {
		return nil
	}

	for _, cfg := range cfgs {
		fmt.Println(X(cfg))
	}
	return nil
}

// `pull` command, copies file or directory from the overlay to the actual
// filesystem.  will never delete files on filesystem, but always overwrites
// existing ones.
func Pull(path string) error {
	var err error
	var abs, dest, src string

	abs, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	dest, err = StripRoot(abs)
	if err != nil {
		return err
	}

	src = C(dest)
	cfgs, err := pathToFiles(src)
	if err != nil && os.IsNotExist(err) {
		return nil
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Could not load files from %s: %s\n", src, err.Error())
		return err
	}

	for _, cfg := range cfgs {
		if !QuickDiff(cfg, X(cfg)) {
			file := X(cfg)
			err = Copy(file, cfg)
			if err == nil {
				fmt.Printf("%s -> %s\n", cfg, file)
			} else {
				fmt.Fprintf(os.Stderr, "%s could not be pulled: %s\n", file, err.Error())
			}
		}
	}
	return nil
}

// `push` command, copies a file or all files from the actual filesystem that
// exist in the overlay to the overlay.
func Push(path string) error {
	var err error
	var abs, dest, src string

	abs, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	dest, err = StripRoot(abs)
	if err != nil {
		return err
	}

	src = C(dest)
	cfgs, err := pathToFiles(src)
	if err != nil && os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	for _, cfg := range cfgs {
		file := X(cfg)
		if !QuickDiff(cfg, file) {
			err = Copy(cfg, file)
			if err != nil {
				return err
			}
			fmt.Printf("%s -> %s\n", file, cfg)
		}
	}
	return nil

}

// `diff` command, diffs two files or two directories of files using the system
// diff program with '-u'.  this will only diff files which are present in the
// overlay, and the diff shows what would happen with a `pull`
func Diff(path string, reverse bool) error {
	var cmd *exec.Cmd
	var err error
	var abs, dest, src string

	abs, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	dest, err = StripRoot(abs)
	if err != nil {
		return err
	}

	src = C(dest)
	cfgs, err := pathToFiles(src)

	if err != nil && os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Requested file not found: %s: %s\n", src, err.Error())
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Could not diff files from %s: %s\n", src, err.Error())
		return err
	}

	for _, cfg := range cfgs {
		if !QuickDiff(cfg, X(cfg)) {
			if reverse {
				cmd, err = diff(X(cfg), cfg)
			} else {
				cmd, err = diff(cfg, X(cfg))
			}
			if err != nil {
				return err
			}
			output, _ := cmd.Output()
			fmt.Print(string(output))
		}
	}

	return nil
}

// Show a diffstat style status for the path
func Status(path string, reverse bool) error {
	var err error
	var abs, dest, src string
	var cmd *exec.Cmd
	var diffout, output []byte

	abs, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	dest, err = StripRoot(abs)
	if err != nil {
		return err
	}

	src = C(dest)
	cfgs, err := pathToFiles(src)

	if err != nil && os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Requested file not found: %s: %s\n", src, err.Error())
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "Could not diff files from %s: %s\n", src, err.Error())
		return err
	}

	diffout = make([]byte, 0, 4096)
	for _, cfg := range cfgs {
		if !QuickDiff(cfg, X(cfg)) {
			if reverse {
				cmd, err = diff(X(cfg), cfg, true)
			} else {
				cmd, err = diff(cfg, X(cfg), true)
			}
			if err != nil {
				return err
			}
			output, _ = cmd.Output()
			diffout = append(diffout, output...)
		}
	}

	cmd, err = diffstat(string(diffout))
	if err != nil {
		return err
	}
	out, _ := cmd.Output()
	fmt.Printf(string(out))

	return nil
}
