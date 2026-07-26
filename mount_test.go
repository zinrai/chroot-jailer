package main

import (
	"strings"
	"testing"
)

// A trimmed but otherwise verbatim /proc/self/mountinfo. The escaped mount point
// is what the kernel writes for a path containing a space.
const sampleMountInfo = `25 30 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw
26 25 0:22 / /proc rw,nosuid,nodev,noexec,relatime shared:14 - proc proc rw
27 25 0:6 / /usr ro,nosuid,nodev,relatime shared:2 - ext4 /dev/sda2 ro
28 25 0:7 / /mnt/with\040space rw,relatime shared:3 - ext4 /dev/sdb1 rw
`

func TestParseMountInfo(t *testing.T) {
	targets, err := parseMountInfo(strings.NewReader(sampleMountInfo))
	if err != nil {
		t.Fatalf("parseMountInfo returned an error: %v", err)
	}

	want := []string{"/", "/proc", "/usr", "/mnt/with space"}

	if len(targets) != len(want) {
		t.Fatalf("got %v, want %v", targets, want)
	}

	for i := range want {
		if targets[i] != want[i] {
			t.Errorf("entry %d: got %q, want %q", i, targets[i], want[i])
		}
	}
}

func TestIsMounted(t *testing.T) {
	targets, err := parseMountInfo(strings.NewReader(sampleMountInfo))
	if err != nil {
		t.Fatalf("parseMountInfo returned an error: %v", err)
	}

	if !isMounted(targets, "/usr") {
		t.Error("/usr should be reported as mounted")
	}
	if isMounted(targets, "/usr/bin") {
		t.Error("/usr/bin is not a mount point and should not be reported as one")
	}
}

// The jail mounts a writable directory inside a read-only one, so unmounting in
// mountinfo order would hit EBUSY on the parent.
func TestUmountOrder(t *testing.T) {
	const nested = `25 30 8:1 / / rw,relatime shared:1 - ext4 /dev/sda1 rw
39 25 8:1 / /var/jail rw,relatime - ext4 /dev/sda1 rw
40 25 8:1 /usr /var/jail/usr ro,relatime - ext4 /dev/sda1 ro
41 40 8:1 /srv /var/jail/usr/local/data rw,relatime - ext4 /dev/sda1 rw
42 25 8:1 /bin /var/jail/bin ro,relatime - ext4 /dev/sda1 ro
43 25 8:1 / /var/jail-other rw,relatime - ext4 /dev/sda1 rw
`

	targets, err := parseMountInfo(strings.NewReader(nested))
	if err != nil {
		t.Fatalf("parseMountInfo returned an error: %v", err)
	}

	// The jail root itself and a path that merely shares its prefix are both
	// left alone.
	got := umountOrder(targets, "/var/jail")
	want := []string{
		"/var/jail/usr/local/data",
		"/var/jail/usr",
		"/var/jail/bin",
	}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
