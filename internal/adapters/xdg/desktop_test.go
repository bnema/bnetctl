package xdg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bnema/bnetctl/internal/ports"
)

func TestQuoteDesktopExec(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		args []string
		want string
	}{
		{
			name: "plain path unquoted",
			exe:  "/usr/bin/bnetctl",
			args: []string{"launch", "--display-driver", "wayland"},
			want: "/usr/bin/bnetctl launch --display-driver wayland",
		},
		{
			name: "path with spaces quoted",
			exe:  "/home/user/my apps/bnetctl",
			args: []string{"launch", "--display-driver", "x11"},
			want: `"/home/user/my apps/bnetctl" launch --display-driver x11`,
		},
		{
			name: "backslashes and quotes escaped",
			exe:  `C:\we"ird\path`,
			want: `"C:\\we\"ird\\path"`,
		},
		{
			name: "percent escaped",
			exe:  `/home/user/50%/bnetctl`,
			want: `"/home/user/50%%/bnetctl"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := QuoteDesktopExec(tt.exe, tt.args...); got != tt.want {
				t.Fatalf("QuoteDesktopExec() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateDualEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	d := NewDesktop()
	icon := "/tmp/battlenet.png"

	entries := map[string]struct {
		display string
		driver  string
	}{
		ports.EntryWayland: {display: ports.DisplayNameWayland, driver: "wayland"},
		ports.EntryX11:     {display: ports.DisplayNameX11, driver: "x11"},
	}

	for name, e := range entries {
		execCmd := QuoteDesktopExec("/usr/bin/bnetctl", "launch", "--display-driver", e.driver)
		if err := d.CreateEntry(name, e.display, execCmd, icon); err != nil {
			t.Fatalf("CreateEntry(%s): %v", name, err)
		}
	}

	if ports.DisplayNameWayland != "Battle.net (bnetctl/Wayland)" {
		t.Fatalf("wayland display name = %q", ports.DisplayNameWayland)
	}
	if ports.DisplayNameX11 != "Battle.net (bnetctl/X11)" {
		t.Fatalf("x11 display name = %q", ports.DisplayNameX11)
	}

	for name, e := range entries {
		path := filepath.Join(home, ".local", "share", "applications", name+".desktop")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		content := string(data)
		if !strings.Contains(content, "Name="+e.display+"\n") {
			t.Fatalf("%s missing Name=%s:\n%s", name, e.display, content)
		}
		if !strings.Contains(content, "Exec=/usr/bin/bnetctl launch --display-driver "+e.driver) {
			t.Fatalf("%s missing driver exec line:\n%s", name, content)
		}
	}

	// Removal covers both entries plus legacy name without error.
	for _, name := range []string{ports.EntryWayland, ports.EntryX11, ports.EntryLegacy} {
		if err := d.RemoveEntry(name); err != nil {
			t.Fatalf("RemoveEntry(%s): %v", name, err)
		}
		if d.EntryExists(name) {
			t.Fatalf("EntryExists(%s) = true after remove", name)
		}
	}
}
