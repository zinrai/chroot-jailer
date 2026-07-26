package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// enterJail chroots into root, drops the capability bounding set and replaces
// this process with argv. It only returns on failure.
//
// The order is fixed. The bounding set is reduced before the chroot because it
// applies at the execve rather than at the chroot, and because the verification
// reads /proc, which is not reachable from inside the jail. The command is
// resolved after the chroot so that it is looked up inside it.
func enterJail(root string, argv []string) error {
	if err := dropCapabilities(); err != nil {
		return err
	}

	bounding, err := verifyBoundingSet()
	if err != nil {
		return err
	}

	trace("prctl(2) capability bounding set reduced to CapBnd=%016x", bounding)

	if err := syscall.Chroot(root); err != nil {
		return fmt.Errorf("failed to chroot to %s: %w", root, err)
	}

	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("failed to chdir to /: %w", err)
	}

	trace("chroot(2) %s", root)

	path, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("failed to find %s in the jail: %w", argv[0], err)
	}

	trace("execve(2) %s", path)

	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		return fmt.Errorf("failed to exec %s: %w", path, err)
	}

	return nil
}
