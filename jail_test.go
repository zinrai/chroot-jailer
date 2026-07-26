package main

import "testing"

func TestMkdev(t *testing.T) {
	tests := []struct {
		name  string
		major uint32
		minor uint32
		want  int
	}{
		{"null", 1, 3, 0x103},
		{"zero", 1, 5, 0x105},
		{"random", 1, 8, 0x108},
		{"urandom", 1, 9, 0x109},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mkdev(tt.major, tt.minor); got != tt.want {
				t.Errorf("got %#x, want %#x", got, tt.want)
			}
		})
	}
}

// The devices the jail creates have to keep their distinct minor numbers.
// /dev/random and /dev/urandom differ only there, and giving both the urandom
// minor would silently change what the jail exposes.
func TestDeviceLayout(t *testing.T) {
	want := map[string]int{
		"dev/null":    0x103,
		"dev/zero":    0x105,
		"dev/random":  0x108,
		"dev/urandom": 0x109,
	}

	if len(devices) != len(want) {
		t.Fatalf("got %d devices, want %d", len(devices), len(want))
	}

	for _, dev := range devices {
		got := mkdev(dev.major, dev.minor)
		if got != want[dev.path] {
			t.Errorf("%s: got %#x, want %#x", dev.path, got, want[dev.path])
		}
	}
}
