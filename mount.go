package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
)

// mountInfoUnescaper decodes the octal escapes that mountinfo uses for space,
// tab, newline and backslash inside paths.
var mountInfoUnescaper = strings.NewReplacer(
	"\\040", " ",
	"\\011", "\t",
	"\\012", "\n",
	"\\134", "\\",
)

// readMountInfo returns every mount point known to this process.
//
// /proc/self/mountinfo is the single source of truth for what is already
// mounted, which is why no state is kept anywhere else.
func readMountInfo() ([]string, error) {
	file, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return nil, fmt.Errorf("failed to open /proc/self/mountinfo: %w", err)
	}
	defer file.Close()

	return parseMountInfo(file)
}

// parseMountInfo reads the mountinfo format.
func parseMountInfo(r io.Reader) ([]string, error) {
	targets := []string{}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		// mount ID / parent ID / major:minor / root / mount point / options / ...
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) < 5 {
			continue
		}

		targets = append(targets, mountInfoUnescaper.Replace(fields[4]))
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read /proc/self/mountinfo: %w", err)
	}

	return targets, nil
}

// isMounted reports whether path is a mount point. mountinfo is consulted rather
// than comparing st_dev with the parent directory, because a bind mount within
// the same filesystem keeps the device number of its source and would not be
// detected that way.
func isMounted(targets []string, path string) bool {
	for _, target := range targets {
		if target == path {
			return true
		}
	}

	return false
}

// bindMount binds src at dest, optionally read-only.
//
// Mounting goes through mount(8) rather than mount(2). The read-only pass has to
// repeat the per-mount flags already in effect, otherwise the kernel clears them
// and a read-only bind silently loses nosuid and nodev. util-linux is the thing
// that knows which flags those are and that tracks the ones the kernel adds.
// Splitting the two halves of one operation across two mechanisms would be worse
// than paying for a fork.
func bindMount(src, dest string, readOnly bool) error {
	if err := runMount("--bind", src, dest); err != nil {
		return err
	}

	if !readOnly {
		return nil
	}

	return runMount("-o", "remount,ro,bind", dest)
}

// runMount runs mount(8), reporting what it printed when it fails. The traced
// line is the argv that was executed rather than a description of it.
func runMount(args ...string) error {
	output, err := exec.Command("mount", args...).CombinedOutput()
	if err == nil {
		trace("mount %s", strings.Join(args, " "))
		return nil
	}

	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}

	return fmt.Errorf("mount %s: %s", strings.Join(args, " "), message)
}

// umountOrder returns the mount points below root, deepest first. A nested mount
// point always has its parent as a string prefix, so ordering by descending
// length puts every child before its parent and no parent is left busy.
//
// root itself is excluded: a filesystem mounted there was not put in place by
// this program.
func umountOrder(targets []string, root string) []string {
	below := []string{}
	for _, target := range targets {
		if strings.HasPrefix(target, root+"/") {
			below = append(below, target)
		}
	}

	sort.Slice(below, func(i, j int) bool {
		return len(below[i]) > len(below[j])
	})

	return below
}

// umountJail unmounts everything below root. Unmounting needs nothing that
// util-linux knows, so it goes straight to the kernel.
func umountJail(root string) error {
	targets, err := readMountInfo()
	if err != nil {
		return err
	}

	for _, target := range umountOrder(targets, root) {
		if err := syscall.Unmount(target, 0); err != nil {
			return fmt.Errorf("failed to unmount %s: %w", target, err)
		}
		trace("umount(2) %s", target)
	}

	return nil
}
