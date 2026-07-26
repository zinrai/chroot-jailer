package main

import "testing"

// The mask the verification compares against has to match the capabilities that
// are kept, so it is derived from them rather than written out.
func TestKeepCapsMask(t *testing.T) {
	const want = uint64(0x4e3) // CHOWN|DAC_OVERRIDE|KILL|SETGID|SETUID|NET_BIND_SERVICE

	if got := keepCapsMask(); got != want {
		t.Errorf("got %#x, want %#x", got, want)
	}
}
