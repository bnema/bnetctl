package env

import (
	"slices"
	"testing"
)

func TestBuildGPUEnvForDisplayWaylandUnsetsDisplay(t *testing.T) {
	got := BuildGPUEnvForDisplay(DisplayDriverWayland)
	if !slices.Contains(got.Unset, "DISPLAY") {
		t.Fatalf("Wayland environment must unset DISPLAY: %#v", got.Unset)
	}
}

func TestBuildGPUEnvForDisplayX11KeepsDisplay(t *testing.T) {
	got := BuildGPUEnvForDisplay(DisplayDriverX11)
	if slices.Contains(got.Unset, "DISPLAY") {
		t.Fatalf("X11 environment must inherit DISPLAY: %#v", got.Unset)
	}
}
