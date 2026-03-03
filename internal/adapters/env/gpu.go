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

// hasNTSync checks if the ntsync kernel module is loaded
func hasNTSync() bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ntsync ") {
			return true
		}
	}
	return false
}

// BuildGPUEnv returns environment variables optimized for the detected GPU
// and the current desktop session (focused on Wayland tiling WMs like niri, hyprland).
func BuildGPUEnv() *domain.ProtonEnv {
	env := &domain.ProtonEnv{
		Vars: make(map[string]string),
	}

	gpu := DetectGPU()

	// GPU-specific optimizations
	switch gpu {
	case GPUVendorAMD:
		env.Vars["AMD_VULKAN_ICD"] = "RADV"
		env.Vars["RADV_PERFTEST"] = "gpl"
	case GPUVendorNVIDIA:
		if IsWayland() {
			// Required for NVIDIA on Wayland compositors
			env.Vars["GBM_BACKEND"] = "nvidia-drm"
			env.Vars["__GLX_VENDOR_LIBRARY_NAME"] = "nvidia"
		}
	}

	// Wine/Proton should use XWayland (via DISPLAY) for Battle.net/CEF apps.
	// Wine's native Wayland driver has issues with CEF-based apps like Battle.net.
	// On tiling WMs (niri, hyprland, sway), xwayland-satellite provides DISPLAY.
	// We do NOT unset DISPLAY or force Wayland — XWayland is more stable for gaming.

	// NTSync: if the kernel module is loaded, Proton will use it automatically.
	// No env var needed — proton-cachyos detects /dev/ntsync at runtime.

	// FreeType hinting for better font rendering in Wine
	env.Vars["FREETYPE_PROPERTIES"] = "truetype:interpreter-version=35"

	return env
}
