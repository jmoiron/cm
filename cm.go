package main

import (
	"fmt"
	"github.com/jmoiron/go-pkg-optarg"
	"os"
)

const (
	VERSION = "0.1"
	ABOUT   = `
cm is a (very) simple configuration manager designed to help keep and maintain
system configs in an alternate overlay directory, which could then be separately
backed up, distributed, or under version control.  The optional file arguments
given to cm can generally also be directories, in which case cm always behaves
recursively.  If no <file> arguments are provided, the current working directory
is used.  Usage follows typical version control interface, with 'cm' followed
by a command and then arguments related to that command:

	add <file>      - add a file to management directory
	rm <file>       - remove file from management directory
	show            - show what files, if any, are added under cm
	diff [file]     - show diff between files under cm
	sync [all]      - sync cm overlay to cwd, or / if "all"
	pkg	[subcmd...] - package management subcommand

cm can also keep a log of installed packages and install missing packages.  This
is done by using the command "pkg" (or "package") and using one of the following
subcommands:

	pkg add <name>  - add package to management list
	pkg rm <name>   - remove package from management list
	pkg sync        - install packages from list
	pkg diff [name] - show diff between list and installed, matching optional name
	pkg show [name] - show packages and install status, matching optional name

`
	COMMAND_HELP = `
Valid commands:

	add <file>      - add a file to management directory
	rm <file>       - remove file from management directory
	show            - show what files, if any, are added under cm
	diff [file]     - show diff between files under cm
	sync [all]      - sync cm overlay to cwd, or / if "all"
	pkg add <name>  - add package to management list
	pkg rm <name>   - remove package from management list
	pkg sync        - install packages from list
	pkg diff [name] - show diff between list and installed, matching optional name
	pkg show [name] - show packages and install status, matching optional name

`
)

type Opts struct {
	Verbose bool
}

var opts Opts

func vPrintf(s string, x ...interface{}) {
	if opts.Verbose {
		fmt.Printf(s, x...)
	}
}

func init() {
	optarg.HeaderFmt = "%s"
	optarg.Header("")
	optarg.Add("h", "help", "show help", false)
	optarg.Add("", "version", "show version", false)
	optarg.Add("v", "verbose", "show more output", false)
	for opt := range optarg.Parse() {
		switch opt.Name {
		case "help":
			optarg.Usage()
			fmt.Printf(ABOUT)
			os.Exit(0)
		case "version":
			fmt.Printf("%s\n", VERSION)
			os.Exit(0)
		case "verbose":
			opts.Verbose = opt.Bool()
		}
	}
}

func main() {
	args := optarg.Remainder
	if len(args) == 0 {
		vPrintf("Nothing to do.  Run `%s --help` for usage.\n", os.Args[0])
		return
	}
	// sanity check, initialize the config path and confirm it's writable
	err := configPathInit()
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		return
	}

	cmd := args[0]
	args = args[1:]

	// work on cwd if no args are given
	if len(args) == 0 {
		args = append(args, ".")
	}

	switch cmd {
	case "add":
		// FIXME: symlink behavior?
		for _, arg := range args {
			err := Add(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error (Add): %s\n", err.Error())
			}
		}
	case "rm", "remove":
		for _, arg := range args {
			err := Rm(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error (Rm): %s\n", err.Error())
			}
		}
	case "show":
		for _, arg := range args {
			err := Show(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error (Show): %s\n", err.Error())
			}
		}
	case "sync", "update":
		for _, arg := range args {
			err := Sync(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error (Sync): %s\n", err.Error())
			}
		}
	case "diff":
		for _, arg := range args {
			err := Diff(arg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error (Diff): %s\n", err.Error())
			}
		}
	case "pkg", "package":
		subcmd := args[0]
		if subcmd == "." {
			fmt.Fprintf(os.Stderr, "Error: pkg command requires subcommand.\n")
			return
		}
		// args := args[1:]
		switch subcmd {
		case "add":
		case "rm", "remove":
		case "sync", "update":
		case "diff":
		case "show":
		default:
			fmt.Fprintf(os.Stderr, "Error: pkg subcommand %s not recognized\n", cmd)
			fmt.Fprintf(os.Stderr, COMMAND_HELP)
		}
	default:
		fmt.Fprintf(os.Stderr, "Error: command %s not recognized\n", cmd)
		fmt.Fprintf(os.Stderr, COMMAND_HELP)
	}

}
