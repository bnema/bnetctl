package env

import (
	"slices"
	"testing"
)

func TestBattleNetArgsForDisplay(t *testing.T) {
	tests := []struct {
		driver DisplayDriver
		want   []string
	}{
		{driver: DisplayDriverWayland, want: []string{
			"--in-process-gpu",
			"--disable-backgrounding-occluded-windows",
			"--disable-renderer-backgrounding",
			"--disable-background-timer-throttling",
		}},
		{driver: DisplayDriverX11},
	}

	for _, tt := range tests {
		if got := BattleNetArgsForDisplay(tt.driver); !slices.Equal(got, tt.want) {
			t.Fatalf("driver %q: got args %v, want %v", tt.driver, got, tt.want)
		}
	}
}

func TestBuildGPUEnvKeepsUserVulkanLayers(t *testing.T) {
	// Games started from Battle.net inherit this env, so bnetctl must not
	// force-disable user Vulkan layers (lsfg-vk stays idle via active_in).
	for _, driver := range []DisplayDriver{DisplayDriverWayland, DisplayDriverX11} {
		got := BuildGPUEnvForDisplay(driver)
		if v, ok := got.Vars["DISABLE_LSFGVK"]; ok {
			t.Fatalf("driver %q must not force-disable lsfg-vk (got %q): games inherit this env", driver, v)
		}
		for _, key := range got.Unset {
			if key == "DISABLE_LSFGVK" || key == "VK_LOADER_LAYERS_DISABLE" || key == "VK_LOADER_LAYERS_ENABLE" {
				t.Fatalf("driver %q must not unset user Vulkan layer config %q", driver, key)
			}
		}
	}
}

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
