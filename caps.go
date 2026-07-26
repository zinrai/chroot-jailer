package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// prctl(2) operations on the capability bounding set. The standard syscall
// package does not wrap prctl, so they go through RawSyscall.
const (
	prCapbsetRead = 23
	prCapbsetDrop = 24
)

// dropCapabilities removes every capability outside keepCaps from the bounding
// set of the calling thread. It reaches the command through the execve that
// follows, which is why the caller must not change threads in between.
func dropCapabilities() error {
	var i uintptr

	for ; ; i++ {
		// PR_CAPBSET_READ reports EINVAL once i is past the last capability the
		// running kernel knows about.
		if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, prCapbsetRead, i, 0); errno != 0 {
			break
		}

		if keepCaps[i] {
			continue
		}

		if _, _, errno := syscall.RawSyscall(syscall.SYS_PRCTL, prCapbsetDrop, i, 0); errno != 0 {
			return fmt.Errorf("failed to drop capability %d: %w", i, errno)
		}
	}

	if i == 0 {
		return errors.New("failed to read the capability bounding set")
	}

	return nil
}

// verifyBoundingSet reads the bounding set back and fails if anything outside
// keepCaps survived. It returns the set it read, which is the only report this
// program can give of a restriction that leaves no other trace.
//
// This guards the one way this program can fail without saying so. The bounding
// set is a per-thread attribute, so if the goroutine ever moved between the drop
// and the execve the command would start with its capabilities intact and
// nothing would report it. /proc/thread-self reports the calling thread, which
// /proc/self does not.
func verifyBoundingSet() (uint64, error) {
	data, err := os.ReadFile("/proc/thread-self/status")
	if err != nil {
		return 0, fmt.Errorf("failed to read the capability bounding set: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		value, found := strings.CutPrefix(line, "CapBnd:")
		if !found {
			continue
		}

		bounding, err := strconv.ParseUint(strings.TrimSpace(value), 16, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse CapBnd %q: %w", strings.TrimSpace(value), err)
		}

		if bounding&^keepCapsMask() != 0 {
			return 0, fmt.Errorf("capabilities outside the intended set survived: CapBnd=%016x", bounding)
		}

		return bounding, nil
	}

	return 0, errors.New("CapBnd is missing from /proc/thread-self/status")
}

// keepCapsMask is the bounding set keepCaps adds up to. It is derived rather
// than written out so that the two cannot drift apart.
func keepCapsMask() uint64 {
	var mask uint64

	for index := range keepCaps {
		mask |= uint64(1) << index
	}

	return mask
}
