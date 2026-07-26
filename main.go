package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const progName = "chroot-jailer"

// Injected at build time by goreleaser via -ldflags -X
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// The capability bounding set is a per-thread attribute, so dropping it and
	// the execve that follows have to happen on the same OS thread. Without this
	// the goroutine may be rescheduled onto another thread in between and the
	// command would start with the bounding set untouched. verifyBoundingSet
	// catches it if that ever stops holding.
	runtime.LockOSThread()

	err := dispatch(os.Args[1:])
	if err == nil {
		return
	}

	// Asking for the usage is not a failure, wherever the request came from.
	if errors.Is(err, flag.ErrHelp) {
		printUsage()
		return
	}

	fmt.Fprintf(os.Stderr, "%s: %v\n", progName, err)
	os.Exit(1)
}

// dispatch runs the command named by the first argument.
//
// Every failure below this point is returned rather than exiting, so that main
// is the only place this program reports an error and the only place it decides
// which stream to report it on and what to exit with.
func dispatch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command given, run %s help for usage", progName)
	}

	switch args[0] {
	case "run":
		return runJail(args[1:])
	case "umount":
		return releaseJail(args[1:])
	case "version":
		printVersion()
	case "help":
		return flag.ErrHelp
	default:
		return fmt.Errorf("unknown command %q, run %s help for usage", args[0], progName)
	}

	return nil
}

// newFlagSet returns a flag set that neither prints nor exits on its own. What
// it would have printed comes back as an error instead, so a bad flag reaches
// the user by the same path as every other failure.
func newFlagSet(name string) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.Usage = func() {}

	return set
}

// runJail creates the jail if it is missing and executes the command inside it.
// Without a command the jail is only prepared.
func runJail(args []string) error {
	var specs []bindSpec

	set := newFlagSet("run")
	root := set.String("root", "", "Path to the chroot jail, absolute (required)")
	set.Var(&bindFlag{specs: &specs, readOnly: false}, "bind", "Expose the directory or file src[:dest] to the jail read-write")
	set.Var(&bindFlag{specs: &specs, readOnly: true}, "robind", "Expose the directory or file src[:dest] to the jail read-only")

	// Parsing stops at the first non-flag argument, so the command to run inside
	// the jail keeps its own flags without needing a -- separator.
	if err := set.Parse(args); err != nil {
		return err
	}

	jailRoot, err := resolveJailRoot(*root)
	if err != nil {
		return err
	}

	if err := setupJail(jailRoot, specs); err != nil {
		return fmt.Errorf("failed to setup jail: %w\n"+
			"The mounts already made are left in place. Rerun once the cause is fixed, "+
			"or release them with: %s umount --root=%s", err, progName, jailRoot)
	}

	argv := set.Args()
	if len(argv) == 0 {
		fmt.Fprintln(os.Stderr, "jail is ready!")
		return nil
	}

	// Only returns on failure: on success this process is replaced by argv.
	return enterJail(jailRoot, argv)
}

// releaseJail unmounts everything below the jail root. The jail directory itself
// is left in place.
func releaseJail(args []string) error {
	set := newFlagSet("umount")
	root := set.String("root", "", "Path to the chroot jail, absolute (required)")

	if err := set.Parse(args); err != nil {
		return err
	}

	// The flags that only make sense while building the jail are not defined on
	// this set, so the flag package rejects them. A trailing command is rejected
	// rather than ignored, so that a command meant for run is not dropped in
	// silence while the unmounting goes ahead.
	if set.NArg() > 0 {
		return fmt.Errorf("umount takes no command: %s", set.Arg(0))
	}

	jailRoot, err := resolveJailRoot(*root)
	if err != nil {
		return err
	}

	return umountJail(jailRoot)
}

// resolveJailRoot checks the privileges this program needs and normalises the
// jail root. The privilege check lives here rather than in init() so that the
// package stays testable.
func resolveJailRoot(root string) (string, error) {
	if os.Geteuid() != 0 {
		return "", errors.New("this program must be run as root (sudo)")
	}

	if root == "" {
		return "", errors.New("please specify the jail root using --root")
	}

	if !strings.HasPrefix(root, "/") {
		return "", fmt.Errorf("--root must be an absolute path: %s", root)
	}

	return filepath.Clean(root), nil
}
