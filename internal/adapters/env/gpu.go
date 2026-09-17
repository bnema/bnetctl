package env

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bnema/bnetctl/internal/domain"
)

// GPUVendor represents a GPU manufacturer
type GPUVendor string

const (
	GPUVendorAMD     GPUVendor = "amd"
	GPUVendorNVIDIA  GPUVendor = "nvidia"
	GPUVendorIntel   GPUVendor = "intel"
	GPUVendorUnknown GPUVendor = "unknown"
)

type DisplayDriver string

const (
	DisplayDriverWayland DisplayDriver = "wayland"
	DisplayDriverX11     DisplayDriver = "x11"

	BattleNetInProcessGPUArg = "--in-process-gpu"
)

// DetectGPU reads /sys/class/drm to determine the GPU vendor
func DetectGPU() GPUVendor {
	drmPath := "/sys/class/drm"
	entries, err := os.ReadDir(drmPath)
	if err != nil {
		return GPUVendorUnknown
	}

	for _, entry := range entries {
		vendorFile := filepath.Join(drmPath, entry.Name(), "device", "vendor")
		data, err := os.ReadFile(vendorFile)
		if err != nil {
			continue
		}

		vendor := strings.TrimSpace(string(data))
		switch vendor {
		case "0x1002":
			return GPUVendorAMD
		case "0x10de":
			return GPUVendorNVIDIA
		case "0x8086":
			return GPUVendorIntel
		}
	}

	return GPUVendorUnknown
}

// IsWayland checks if the current session is running Wayland
func IsWayland() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

// BuildGPUEnv returns environment variables optimized for the detected GPU.
// Native Wayland is the default display driver.
func BuildGPUEnv() *domain.WineEnv {
	return BuildGPUEnvForDisplay(DisplayDriverWayland)
}

// BattleNetArgsForDisplay returns launcher-only compatibility arguments.
// Wine Wayland cannot render CEF's cross-process Vulkan surface, so keep GPU
// rendering in Battle.net's window-owning process. The argument is not inherited
// through the environment, leaving games free to use their normal GPU setup.
func BattleNetArgsForDisplay(driver DisplayDriver) []string {
	if driver == DisplayDriverWayland {
		return []string{BattleNetInProcessGPUArg}
	}
	return nil
}

func BuildGPUEnvForDisplay(driver DisplayDriver) *domain.WineEnv {
	env := &domain.WineEnv{
		Vars: make(map[string]string),
	}

	gpu := DetectGPU()

	// GPU-specific optimizations
	switch gpu {
	case GPUVendorAMD:
		env.Vars["AMD_VULKAN_ICD"] = "RADV"
	case GPUVendorNVIDIA:
		if IsWayland() {
			// Required for NVIDIA on Wayland compositors
			env.Vars["GBM_BACKEND"] = "nvidia-drm"
			env.Vars["__GLX_VENDOR_LIBRARY_NAME"] = "nvidia"
		}
	}

	if driver == DisplayDriverWayland {
		// Without an X11 display, Wine selects its native Wayland driver.
		env.Unset = append(env.Unset, "DISPLAY")
	}

	// NOTE: do NOT set DISABLE_LSFGVK here. lsfg-vk is a GLOBAL Vulkan
	// layer that stays idle unless the exe matches a profile's active_in
	// list, so Battle.net.exe is unaffected once the user config is valid
	// v2 (see lsfg-vk.dev). Games started from Battle.net inherit this
	// environment, so force-disabling would also kill frame generation
	// for games. Keep user Vulkan layers untouched.

	// NTSync: wine-cachyos detects /dev/ntsync automatically. No env var needed.

	// Wine sync primitives — enable esync/fsync for frame pacing.
	// NTSync (preferred) is auto-detected by wine-cachyos from /dev/ntsync.
	// esync/fsync are fallbacks if ntsync is unavailable.
	env.Vars["WINEESYNC"] = "1"
	env.Vars["WINEFSYNC"] = "1"

	// Suppress Wine debug output for performance
	env.Vars["WINEDEBUG"] = "-all"

	// FreeType hinting for better font rendering in Wine
	env.Vars["FREETYPE_PROPERTIES"] = "truetype:interpreter-version=35"

	return env
}
