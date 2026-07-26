package main

import "testing"

func TestParseBindSpec(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		readOnly bool
		want     bindSpec
		wantErr  bool
	}{
		{
			name:  "dest defaults to src",
			value: "/usr/local/apache",
			want:  bindSpec{src: "/usr/local/apache", dest: "/usr/local/apache"},
		},
		{
			name:  "explicit dest",
			value: "/srv/myapp/data:/var/data",
			want:  bindSpec{src: "/srv/myapp/data", dest: "/var/data"},
		},
		{
			name:     "read-only is carried through",
			value:    "/etc/myapp",
			readOnly: true,
			want:     bindSpec{src: "/etc/myapp", dest: "/etc/myapp", readOnly: true},
		},
		{
			name:  "paths are cleaned",
			value: "/srv/myapp/./data/:/var/data",
			want:  bindSpec{src: "/srv/myapp/data", dest: "/var/data"},
		},
		{
			name:    "relative src is rejected",
			value:   "srv/myapp",
			wantErr: true,
		},
		{
			name:    "relative dest is rejected",
			value:   "/srv/myapp:var/data",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBindSpec(tt.value, tt.readOnly)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("got %+v, want an error", got)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseBindSpec returned an error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// -bind and -robind write into the same slice, so the order given on the command
// line has to survive. bindCustomDirs relies on it to let a writable directory
// land inside a read-only one.
func TestBindFlagKeepsOrder(t *testing.T) {
	var specs []bindSpec

	readWrite := &bindFlag{specs: &specs, readOnly: false}
	readOnly := &bindFlag{specs: &specs, readOnly: true}

	if err := readOnly.Set("/etc/myapp"); err != nil {
		t.Fatalf("Set returned an error: %v", err)
	}
	if err := readWrite.Set("/srv/myapp/data"); err != nil {
		t.Fatalf("Set returned an error: %v", err)
	}
	if err := readOnly.Set("/usr/local/apache"); err != nil {
		t.Fatalf("Set returned an error: %v", err)
	}

	want := []bindSpec{
		{src: "/etc/myapp", dest: "/etc/myapp", readOnly: true},
		{src: "/srv/myapp/data", dest: "/srv/myapp/data"},
		{src: "/usr/local/apache", dest: "/usr/local/apache", readOnly: true},
	}

	if len(specs) != len(want) {
		t.Fatalf("got %+v, want %+v", specs, want)
	}

	for i := range want {
		if specs[i] != want[i] {
			t.Fatalf("got %+v, want %+v", specs, want)
		}
	}
}
