package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/bnema/bnetctl/internal/adapters/env"
)

func TestResolveDisplayDriver(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wayland     bool
		x11         bool
		want        env.DisplayDriver
		wantErrText string
	}{
		{name: "wayland", raw: "wayland", wayland: true, want: env.DisplayDriverWayland},
		{name: "wayland with xwayland also present", raw: "wayland", wayland: true, x11: true, want: env.DisplayDriverWayland},
		{name: "x11", raw: "x11", x11: true, want: env.DisplayDriverX11},
		{name: "x11 with wayland also present", raw: "x11", wayland: true, x11: true, want: env.DisplayDriverX11},
		{name: "wayland without display server", raw: "wayland", wantErrText: "WAYLAND_DISPLAY"},
		{name: "x11 without display server", raw: "x11", wantErrText: "DISPLAY"},
		{name: "unknown driver", raw: "xwayland", wayland: true, x11: true, wantErrText: "invalid display driver"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wayland {
				t.Setenv("WAYLAND_DISPLAY", "wayland-0")
			} else {
				t.Setenv("WAYLAND_DISPLAY", "")
			}
			if tt.x11 {
				t.Setenv("DISPLAY", ":0")
			} else {
				t.Setenv("DISPLAY", "")
			}

			got, err := resolveDisplayDriver(tt.raw)
			if tt.wantErrText != "" {
				if err == nil {
					t.Fatalf("expected error for %q", tt.raw)
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Fatalf("error %q does not mention %q", err, tt.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got driver %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisplayDriverFlagRegistered(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
	}{
		{name: "install", cmd: installCmd},
		{name: "launch", cmd: launchCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := tt.cmd.Flags().Lookup("display-driver")
			if flag == nil {
				t.Fatalf("%s has no display-driver flag", tt.name)
			}
			if flag.DefValue != string(env.DisplayDriverWayland) {
				t.Fatalf("%s default is %q, want %q", tt.name, flag.DefValue, env.DisplayDriverWayland)
			}
		})
	}
}

func TestDisplayDriverFlagsBindToSeparateVariables(t *testing.T) {
	t.Cleanup(func() {
		_ = installCmd.Flags().Set("display-driver", string(env.DisplayDriverWayland))
	})

	if err := installCmd.Flags().Set("display-driver", "x11"); err != nil {
		t.Fatal(err)
	}
	if installDisplayDriver != "x11" {
		t.Fatalf("install flag bound to %q, want x11", installDisplayDriver)
	}
	if displayDriver == "x11" {
		t.Fatal("setting the install flag also changed the launch variable")
	}
}
