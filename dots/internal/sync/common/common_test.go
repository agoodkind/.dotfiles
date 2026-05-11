package common

import (
	"reflect"
	"testing"
)

func TestDebianPrivilegePrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		euid          int
		sudoAvailable bool
		sudoUsable    bool
		want          []string
		wantErr       bool
	}{
		{
			name:    "root without sudo",
			euid:    0,
			wantErr: false,
		},
		{
			name:          "non-root with usable sudo",
			euid:          1000,
			sudoAvailable: true,
			sudoUsable:    true,
			want:          []string{"sudo", "-n"},
			wantErr:       false,
		},
		{
			name:          "non-root without sudo",
			euid:          1000,
			sudoAvailable: false,
			sudoUsable:    false,
			wantErr:       true,
		},
		{
			name:          "non-root with unusable sudo",
			euid:          1000,
			sudoAvailable: true,
			sudoUsable:    false,
			wantErr:       true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := DebianPrivilegePrefix(test.euid, test.sudoAvailable, test.sudoUsable)
			if test.wantErr {
				if err == nil {
					t.Fatal("DebianPrivilegePrefix returned nil error")
				}
				return
			}
			if err != nil {
				t.Fatalf("DebianPrivilegePrefix returned error: %v", err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("DebianPrivilegePrefix = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestDebianPrivilegedCommand(t *testing.T) {
	t.Parallel()

	command, args := DebianPrivilegedCommand(nil, "apt-get", "update")
	if command != "apt-get" {
		t.Fatalf("command = %q, want apt-get", command)
	}
	if !reflect.DeepEqual(args, []string{"update"}) {
		t.Fatalf("args = %#v, want update", args)
	}

	command, args = DebianPrivilegedCommand([]string{"sudo", "-n"}, "apt-get", "update")
	if command != "sudo" {
		t.Fatalf("command = %q, want sudo", command)
	}
	if !reflect.DeepEqual(args, []string{"-n", "apt-get", "update"}) {
		t.Fatalf("args = %#v, want sudo apt-get update args", args)
	}
}
