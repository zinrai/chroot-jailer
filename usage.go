package main

import "fmt"

func printVersion() {
	fmt.Printf("%s version %s\n", progName, version)
	fmt.Printf("commit: %s\n", commit)
	fmt.Printf("built: %s\n", date)
}

func printUsage() {
	const usage = `chroot-jailer - Build and enter a minimal chroot jail

Usage:
  chroot-jailer run --root /path/to/jail [--bind src[:dest]] [--robind src[:dest]] [command ...]
  chroot-jailer umount --root /path/to/jail
  chroot-jailer version
  chroot-jailer help

Commands:
  run       Create the jail if it does not exist, then execute the command
            inside it. Without a command the jail is only prepared.
  umount    Release every bind mount below the jail root. The jail directory
            itself is left in place.
  version   Print version information
  help      Print this message

The host system directories are bind mounted read-only, /etc/passwd and friends
are copied in, and the capability bounding set is reduced before the command is
executed.

Directories that usually hold user data, such as /home, /var and /usr/local, are
not exposed. Export them explicitly with --bind or --robind.

Options:
  --root PATH       Path to the chroot jail, absolute (required)
  --bind SPEC       Expose the directory or file src[:dest] to the jail
                    read-write, repeatable
  --robind SPEC     Expose the directory or file src[:dest] to the jail
                    read-only, repeatable

Every operation is reported on standard error as it is performed, leaving
standard output to the command executed inside the jail. Only operations that
changed something are reported, so reusing a jail that is already in place is
silent.

Examples:
  # Prepare the jail without running anything
  sudo chroot-jailer run --root /var/lib/jail/example

  # Restrict a scheduled job
  sudo chroot-jailer run --root /var/lib/jail/backup \
      --robind /etc/backup --bind /srv/backup \
      /usr/local/bin/backup.sh

  # Release the mounts
  sudo chroot-jailer umount --root /var/lib/jail/example

Note: This program requires root privileges (sudo)`

	fmt.Println(usage)
}
