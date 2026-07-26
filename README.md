# chroot-jailer

A command-line tool to build a chroot jail out of the running host and execute a command inside it.

The jail is assembled by bind mounting the host system directories read-only, so there is no image to build and no root filesystem to unpack. A jail costs a handful of bind mounts and a few kilobytes of directories, and the command inside it runs against the same `/usr` the host does. The design follows [kazuho/jailing](https://github.com/kazuho/jailing).

The goal is restriction rather than isolation. No namespace is involved, so the command keeps its place in the host process table, binds host ports directly and receives signals as before. What changes is what it can read and write.

## Features

- Creates the jail on first use and reuses it afterwards
- Exposes the host system directories read-only, so a command cannot modify them
- Hides the directories that usually hold user data, unless they are exported explicitly
- Copies `/etc/passwd` and friends into the jail, where they can then be edited independently
- Reduces the capability bounding set before executing the command
- Replaces itself with the command, leaving no supervisor process behind
- A single static binary with no Go module dependencies, so installing it is copying it

## Requirements

- Linux
- Root privileges (sudo)
- `mount(8)` from util-linux

## Quick Start

```bash
# Prepare the jail without running anything
$ sudo chroot-jailer run --root /var/lib/jail/example
mkdir -p /var/lib/jail/example
cp /etc/passwd /var/lib/jail/example/etc/passwd
mount --bind /usr/bin /var/lib/jail/example/usr/bin
mount -o remount,ro,bind /var/lib/jail/example/usr/bin
mknod -m 0666 /var/lib/jail/example/dev/null c 1 3
...
jail is ready!

# Run a command inside it
$ sudo chroot-jailer run --root /var/lib/jail/example /bin/sh
prctl(2) capability bounding set reduced to CapBnd=00000000000004e3
chroot(2) /var/lib/jail/example
execve(2) /bin/sh

# Release the mounts
$ sudo chroot-jailer umount --root /var/lib/jail/example
umount(2) /var/lib/jail/example/usr/share
...
```

`run` creates the jail if it does not exist and executes the command inside it. `umount` releases the mounts. Run `chroot-jailer help` for the full list of options.

## Use Cases

### Restricting a scheduled job

A backup or maintenance script needs the host's `rsync`, `pg_dump` and CA certificates, but it has no business reading `/home` or writing anywhere outside its own output directory. Wrapping the cron entry gives it exactly that view:

```bash
sudo chroot-jailer run --root /var/lib/jail/backup \
    --robind /etc/backup \
    --bind /srv/backup \
    /usr/local/bin/backup.sh
```

The script still runs against the host userland, so nothing has to be packaged or kept in sync. `/home`, `/var` and `/usr/local` are simply not there, and an unset variable in an `rm -rf` cannot reach them.

### Inspecting what a program produced

The jail is a plain directory and it outlives the command, so whatever the command wrote is still there afterwards. This makes it a place to run an installer or a build script whose behaviour is not fully known:

```bash
sudo chroot-jailer run --root /var/tmp/audit-20260725 \
    --robind /opt/vendor \
    /opt/vendor/bin/installer

sudo find /var/tmp/audit-20260725 -xdev -type f
```

`-xdev` keeps `find` on the jail's own filesystem, so the read-only system directories are not walked and only what the command created is listed.

Two things matter for this to work. Do not export the output directory with `--bind`, because writes to a bind mount go through to the host and are not captured. Use a fresh `--root` for each run, because an existing jail is reused and the contents of separate runs would mix.

### Running a daemon without a unit file

Where there is no unit file to put the hardening directives in, such as a SysV init script, runit, s6, a CI runner or the entrypoint of a container, the same restrictions can be applied from the command line:

```bash
sudo chroot-jailer run --root /var/lib/jail/myapp \
    --bind /opt/myapp \
    /opt/myapp/bin/myappd -f /opt/myapp/conf/myappd.conf
```

The command replaces this process rather than being supervised by it, so process supervisors see the daemon itself. `CAP_NET_BIND_SERVICE` is kept, so a daemon that binds a privileged port and then drops to an unprivileged user still works.

## How It Works

The jail holds no state of its own. What exists is exactly what `/proc/self/mountinfo` reports below the jail root. Anything already in place is left alone, so `run` on an existing jail is safe to repeat.

`--bind` and `--robind` are applied after the system directories, in the order they appear on the command line, so a writable directory can be placed inside a read-only one.

The bind mounts stay visible on the host until `umount` is given. They do not disappear when the command exits. Removing a jail is `umount` followed by `rm -rf` on the directory, and the `umount` command never deletes anything.

A write to a read-only location fails with `EROFS`. It is not recorded anywhere, so this is not a way to find out what a program would have changed under `/usr`.

## Scope

This is a helper for running a program under a restricted view of the host. It is not a container runtime and does not try to become one.

The program it is meant for is one you trust but do not assume to be free of bugs, such as a daemon or a script of your own. Restricting what that program can reach limits the damage a mistake in it can do.

The following are out of scope:

- Namespaces. The jail shares the host process table, network stack and mount table.
- Resource limits. There is no cgroup involvement.
- Unprivileged operation. Root is required.
- Sandboxing untrusted code. The command runs as root and keeps `CAP_DAC_OVERRIDE`, so it can still reach anything exposed to the jail regardless of file permissions. That suits a program whose mistakes you want to contain, not a program whose intent you doubt.
- Building a root filesystem. The jail shares the host userland, so a different distribution is not possible.
- Changing user. A daemon that needs to drop privileges does so itself, which is why `CAP_SETUID` and `CAP_SETGID` are kept.

## License

This project is licensed under the [MIT License](./LICENSE).
