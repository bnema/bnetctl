package env

import (
	"os"
	"slices"
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
		{"default", DisplayDriverWayland, false, nil, wowHDRConfig, "wayland default", true},
		{"user on", DisplayDriverWayland, false, str("1"), "", "user DXVK_HDR=1", true},
		{"user off", DisplayDriverWayland, false, str("0"), "", "user DXVK_HDR=0", false},
		{"user empty", DisplayDriverWayland, false, str(""), "", "user DXVK_HDR=", false},
		{"opt out", DisplayDriverWayland, true, str("1"), "", "--no-hdr", false},
		{"x11", DisplayDriverX11, false, nil, "", "x11", false},
		{"x11 inherited on", DisplayDriverX11, false, str("1"), "", "user env", true},
		{"x11 inherited off", DisplayDriverX11, false, str("0"), "", "user env", false},
		{"x11 inherited empty", DisplayDriverX11, false, str(""), "", "user env", false},
		{"x11 ignores opt out", DisplayDriverX11, true, str("1"), "", "user env", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DXVK_CONFIG", "")
			t.Setenv("DXVK_HDR", "")
			if tt.user == nil {
				if err := os.Unsetenv("DXVK_HDR"); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("DXVK_HDR", *tt.user)
			}
			got := BuildGPUEnvForDisplayHDR(tt.driver, tt.noHDR)
			if _, ok := got.Vars["DXVK_HDR"]; ok {
				t.Fatal("unexpected global HDR assignment")
			}
			if got.Vars["DXVK_CONFIG"] != tt.want {
				t.Fatalf("config: %q, want %q", got.Vars["DXVK_CONFIG"], tt.want)
			}
			if tt.want == "" {
				if _, ok := got.Vars["DXVK_CONFIG"]; ok {
					t.Fatal("unexpected config assignment")
				}
			}
			if unset := slices.Contains(got.Unset, "DXVK_HDR"); unset != (tt.driver == DisplayDriverWayland && tt.noHDR) {
				t.Fatalf("unexpected HDR unset policy: %v", got.Unset)
			}
			enabled, reason := HDRStatus(tt.driver, tt.noHDR)
			if enabled != tt.enabled || reason != tt.reason {
				t.Fatalf("status: %v %q", enabled, reason)
			}
		})
	}
}
func str(s string) *string { return &s }

func TestHDRPreservesUserConfig(t *testing.T) {
	const entries = "[WowClassic.exe];dxgi.enableHDR = True;[Wow.exe];dxgi.enableHDR = True;[WoW.exe];dxgi.enableHDR = True;[WowClassicT.exe];dxgi.enableHDR = True;[WowT.exe];dxgi.enableHDR = True;[WoWT.exe];dxgi.enableHDR = True"
	for _, userConfig := range []string{"", "dxgi.maxFrameLatency = 1", "[Battle.net.exe];dxgi.enableHDR = False", "[Other.exe];dxgi.syncInterval = 0;"} {
		t.Run(userConfig, func(t *testing.T) {
			t.Setenv("DXVK_CONFIG", userConfig)
			t.Setenv("DXVK_HDR", "")
			if err := os.Unsetenv("DXVK_HDR"); err != nil {
				t.Fatal(err)
			}
			want := entries
			if userConfig != "" {
				want = userConfig + ";" + entries
			}
			got := BuildGPUEnvForDisplayHDR(DisplayDriverWayland, false)
			if got.Vars["DXVK_CONFIG"] != want {
				t.Fatalf("config: %q, want %q", got.Vars["DXVK_CONFIG"], want)
			}
			got = BuildGPUEnvForDisplayHDR(DisplayDriverWayland, true)
			if _, ok := got.Vars["DXVK_CONFIG"]; ok {
				t.Fatal("opt out must not add config without inherited DXVK_HDR")
			}
			for _, noHDR := range []bool{false, true} {
				t.Setenv("DXVK_HDR", "1")
				got = BuildGPUEnvForDisplayHDR(DisplayDriverWayland, noHDR)
				if _, ok := got.Vars["DXVK_CONFIG"]; ok {
					t.Fatal("must leave inherited config untouched")
				}
				if os.Getenv("DXVK_CONFIG") != userConfig {
					t.Fatal("changed user environment")
				}
			}
		})
	}
}
