package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// trace reports an operation as it is performed. It goes to standard error so
// that standard output belongs to the command executed inside the jail.
//
// Only operations that changed something are traced, so a jail that is already
// in place produces no output at all.
func trace(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// unixMode renders a FileMode the way chmod(1) takes it. Sticky is the only bit
// beyond the permission bits this program sets.
func unixMode(mode os.FileMode) uint32 {
	bits := uint32(mode.Perm())

	if mode&os.ModeSticky != 0 {
		bits |= syscall.S_ISVTX
	}

	return bits
}

// ensureDir creates a directory if it does not exist. MkdirAll does not report
// whether it created anything, so existence is checked separately to keep a
// repeated run out of the trace.
func ensureDir(path string, perm os.FileMode) error {
	existed := pathExists(path)

	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	if !existed {
		trace("mkdir -p %s", path)
	}

	return nil
}

// chmod sets the permissions of path. It does nothing when they already are what
// they should be, so repeating a run stays silent.
func chmod(path string, mode os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", path, err)
	}

	if unixMode(info.Mode()) == unixMode(mode) {
		return nil
	}

	if err := os.Chmod(path, mode); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", path, err)
	}

	trace("chmod %04o %s", unixMode(mode), path)

	return nil
}

func ensureFile(path string) error {
	if pathExists(path) {
		return nil
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("failed to close %s: %w", path, err)
	}

	trace("touch %s", path)

	return nil
}

// copyFile copies src to dest keeping its permission bits. It is a no-op when
// dest already exists, so a jail can be given its own copy of the file.
func copyFile(src, dest string) error {
	if pathExists(dest) {
		return nil
	}

	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat %s: %w", src, err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}

	if err := os.WriteFile(dest, data, info.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to write %s: %w", dest, err)
	}

	if err := os.Chmod(dest, info.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to chmod %s: %w", dest, err)
	}

	trace("cp %s %s", src, dest)

	return nil
}

// copySymlink recreates a host symlink inside the jail.
func copySymlink(src, dest string) error {
	if pathExists(dest) {
		return nil
	}

	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", src, err)
	}

	if err := ensureDir(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	return symlink(target, dest)
}

// symlink creates a symlink at path pointing at target. It is a no-op when path
// already exists.
func symlink(target, path string) error {
	if pathExists(path) {
		return nil
	}

	if err := os.Symlink(target, path); err != nil && !os.IsExist(err) {
		return fmt.Errorf("failed to create symlink %s: %w", path, err)
	}

	trace("ln -s %s %s", target, path)

	return nil
}
