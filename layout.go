package main

import "os"

// This file holds every policy decision the jail makes: which directories are
// created, which host directories are exposed, which files are copied in, which
// devices exist and which capabilities survive. Changing what the jail looks
// like means changing this file and nothing else.

// newDirs are created empty inside the jail.
var newDirs = []string{
	"etc",
	"run",
	"usr",
	"var/log",
}

// bindDirs are the host system directories exposed to the jail read-only.
// Directories that usually hold user data, such as /home, /var and /usr/local,
// are deliberately absent and have to be exported with -bind or -robind.
//
// The order matters. The custom binds run afterwards, so a writable directory
// can be placed inside one of these read-only trees.
var bindDirs = []string{
	"bin",
	"etc/alternatives",
	"etc/ssl/certs",
	"lib",
	"lib64",
	"sbin",
	"usr/bin",
	"usr/include",
	"usr/lib",
	"usr/lib64",
	"usr/libexec",
	"usr/sbin",
	"usr/share",
	"usr/src",
}

// tempDirs are created world writable with the sticky bit set.
var tempDirs = []string{
	"tmp",
	"run/lock",
	"var/tmp",
}

// copyFiles are copied from the host once. They are never overwritten, so the
// jail can be given its own /etc/passwd afterwards.
var copyFiles = []string{
	"etc/group",
	"etc/passwd",
	"etc/resolv.conf",
	"etc/hosts",
}

// device is a character device created inside the jail.
type device struct {
	path  string
	mode  os.FileMode
	major uint32
	minor uint32
}

var devices = []device{
	{"dev/null", 0666, 1, 3},
	{"dev/zero", 0666, 1, 5},
	{"dev/random", 0444, 1, 8},
	{"dev/urandom", 0444, 1, 9},
}

// keepCaps is the set of capabilities left in the bounding set. It covers what a
// typical daemon needs: start as root, bind a privileged port, drop to an
// unprivileged user and manage its children.
var keepCaps = map[uintptr]bool{
	0:  true, // CAP_CHOWN
	1:  true, // CAP_DAC_OVERRIDE
	5:  true, // CAP_KILL
	6:  true, // CAP_SETGID
	7:  true, // CAP_SETUID
	10: true, // CAP_NET_BIND_SERVICE
}
