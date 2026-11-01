package env

import (
	"os"
	"testing"
)

func TestHDRPolicy(t *testing.T) {
	for _, tt := range []struct {
		name         string
		driver       DisplayDriver
		noHDR        bool
		user         *string
		want, reason string
		enabled      bool
	}{
		{"default", DisplayDriverWayland, false, nil, "1", "wayland default", true},
		{"user on", DisplayDriverWayland, false, str("1"), "", "user DXVK_HDR=1", true},
		{"user off", DisplayDriverWayland, false, str("0"), "", "user DXVK_HDR=0", false},
		{"user empty", DisplayDriverWayland, false, str(""), "", "user DXVK_HDR=", false},
		{"opt out", DisplayDriverWayland, true, str("1"), "0", "--no-hdr", false},
		{"x11", DisplayDriverX11, false, nil, "", "x11", false},
		{"x11 inherited on", DisplayDriverX11, false, str("1"), "", "user env", true},
		{"x11 inherited off", DisplayDriverX11, false, str("0"), "", "user env", false},
		{"x11 inherited empty", DisplayDriverX11, false, str(""), "", "user env", false},
		{"x11 ignores opt out", DisplayDriverX11, true, str("1"), "", "user env", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DXVK_HDR", "")
			if tt.user == nil {
				if err := os.Unsetenv("DXVK_HDR"); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("DXVK_HDR", *tt.user)
			}
			got := BuildGPUEnvForDisplayHDR(tt.driver, tt.noHDR)
			if got.Vars["DXVK_HDR"] != tt.want {
				t.Fatalf("env: %v", got.Vars)
			}
			if tt.want == "" {
				if _, ok := got.Vars["DXVK_HDR"]; ok {
					t.Fatal("unexpected HDR assignment")
				}
			}
			enabled, reason := HDRStatus(tt.driver, tt.noHDR)
			if enabled != tt.enabled || reason != tt.reason {
				t.Fatalf("status: %v %q", enabled, reason)
			}
		})
	}
}
func str(s string) *string { return &s }
