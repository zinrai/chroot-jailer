package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// setupJail creates the jail if it is missing and mounts everything it needs.
//
// It is idempotent: existing directories, files and mounts are left alone. A run
// that fails halfway can simply be repeated once the cause is fixed, which is
// why nothing is rolled back on error.
func setupJail(root string, specs []bindSpec) error {
	if err := ensureDir(root, 0755); err != nil {
		return err
	}

	for _, dir := range newDirs {
		if err := ensureDir(filepath.Join(root, dir), 0755); err != nil {
			return err
		}
	}

	if err := createTempDirs(root); err != nil {
		return err
	}

	for _, file := range copyFiles {
		if err := copyFile(filepath.Join("/", file), filepath.Join(root, file)); err != nil {
			return err
		}
	}

	if err := bindSystemDirs(root); err != nil {
		return err
	}

	if err := bindCustomDirs(root, specs); err != nil {
		return err
	}

	if err := symlink("../run/lock", filepath.Join(root, "var/lock")); err != nil {
		return err
	}

	return createDevices(root)
}

// createTempDirs creates the world writable directories. The mode given to
// mkdir(2) is masked by the umask, so the permissions are set explicitly
// afterwards rather than changing the umask this process passes on.
func createTempDirs(root string) error {
	for _, dir := range tempDirs {
		path := filepath.Join(root, dir)

		if err := ensureDir(path, 0777); err != nil {
			return err
		}

		if err := chmod(path, 0777|os.ModeSticky); err != nil {
			return err
		}
	}

	return nil
}

// bindSystemDirs exposes the host system directories read-only. Only the ones
// present on this host are considered, and a symlink on the host is recreated as
// a symlink instead of being bind mounted.
//
// The mountinfo snapshot is read once: the mounts made here are the only ones
// that change it, and each of them has a distinct destination.
func bindSystemDirs(root string) error {
	mounted, err := readMountInfo()
	if err != nil {
		return err
	}

	for _, dir := range bindDirs {
		src := filepath.Join("/", dir)
		dest := filepath.Join(root, dir)

		info, err := os.Lstat(src)
		if err != nil {
			// This host does not have the directory, so there is nothing to expose.
			continue
		}

		if info.Mode()&os.ModeSymlink != 0 {
			if err := copySymlink(src, dest); err != nil {
				return err
			}
			continue
		}

		if !info.IsDir() || isMounted(mounted, dest) {
			continue
		}

		if err := ensureDir(dest, 0755); err != nil {
			return err
		}

		if err := bindMount(src, dest, true); err != nil {
			return err
		}
	}

	return nil
}

// bindCustomDirs mounts the paths given with -bind and -robind, in the order
// they appeared on the command line. It runs after bindSystemDirs so that a
// writable directory can be placed inside a read-only system directory.
func bindCustomDirs(root string, specs []bindSpec) error {
	if len(specs) == 0 {
		return nil
	}

	mounted, err := readMountInfo()
	if err != nil {
		return err
	}

	for _, spec := range specs {
		dest := filepath.Join(root, spec.dest)
		if isMounted(mounted, dest) {
			continue
		}

		if err := ensureMountPoint(spec.src, dest); err != nil {
			return err
		}

		if err := bindMount(spec.src, dest, spec.readOnly); err != nil {
			return err
		}
	}

	return nil
}

// ensureMountPoint creates what a bind mount of src needs at dest. The kernel
// wants the two to match in kind, so anything that is not a directory gets an
// empty file to mount onto rather than a directory.
//
// The source is stated here rather than left to mount(8) so that a path that
// does not exist is reported before any of the jail is built.
func ensureMountPoint(src, dest string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", src, err)
	}

	if info.IsDir() {
		return ensureDir(dest, 0755)
	}

	if err := ensureDir(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	return ensureFile(dest)
}

func createDevices(root string) error {
	if err := ensureDir(filepath.Join(root, "dev"), 0755); err != nil {
		return err
	}

	for _, dev := range devices {
		if err := createDevice(filepath.Join(root, dev.path), dev.mode, dev.major, dev.minor); err != nil {
			return err
		}
	}

	return nil
}

// createDevice creates a single character device. The mode given to mknod(2) is
// masked by the umask, so the permissions are set explicitly afterwards.
func createDevice(path string, mode os.FileMode, major, minor uint32) error {
	if pathExists(path) {
		return nil
	}

	if err := syscall.Mknod(path, syscall.S_IFCHR|uint32(mode.Perm()), mkdev(major, minor)); err != nil {
		return fmt.Errorf("failed to create device %s: %w", path, err)
	}

	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", path, err)
	}

	trace("mknod -m %04o %s c %d %d", mode.Perm(), path, major, minor)

	return nil
}

// mkdev encodes a major/minor pair the way the kernel expects it.
func mkdev(major, minor uint32) int {
	dev := uint64(major&0x00000fff) << 8
	dev |= uint64(major&0xfffff000) << 32
	dev |= uint64(minor & 0x000000ff)
	dev |= uint64(minor&0xffffff00) << 12

	return int(dev)
}
