# bnetctl

A Go CLI tool to install, manage, and run [Battle.net](https://battle.net) on Linux via [wine-cachyos](https://github.com/CachyOS/wine-cachyos).

Built for **Wayland** tiling WMs such as niri and Hyprland. Native Wayland is the default, with an explicit X11/XWayland mode for compatibility. Automatic GPU detection supports AMD, NVIDIA, and Intel.

## Dependencies

**CachyOS** (all packages available in the CachyOS repos):

```bash
# Required — optimized Wine installed under /opt/wine-cachyos
sudo pacman -S wine-cachyos-opt

# Required — DXVK/VKD3D translate Direct3D to Vulkan (critical for frame pacing)
sudo pacman -S dxvk-mingw-git vkd3d-proton-mingw-git

# Optional — extracts Battle.net icon for the .desktop entry
sudo pacman -S icoutils
```

**Arch Linux** (non-CachyOS): `wine-cachyos` requires the [CachyOS repositories](https://wiki.cachyos.net/adding_repo/). The closest alternative is `wine-tkg-git` (AUR) — untested.

## Install

```bash
go install github.com/bnema/bnetctl@latest
```

## Usage

```bash
bnetctl install          # Download, create prefix, install Battle.net
bnetctl launch           # Start Battle.net using native Wayland
bnetctl launch --display-driver x11 # Start through X11/XWayland
bnetctl desktop add      # Create Wayland and X11 menu entries
bnetctl desktop remove   # Remove .desktop menu entry
bnetctl kill             # Stop Battle.net
bnetctl kill -a          # Kill all orphaned Wine processes
bnetctl clean            # Remove cache and prefix
bnetctl clean -a         # Full purge (cache, prefix, desktop entry, data)
```

## What it does

- Uses `/opt/wine-cachyos/bin/wine` by default, or `BNETCTL_WINE` when set
- Creates a Wine prefix with DXVK + VKD3D-proton auto-installed
- Downloads and runs the official Battle.net installer
- Uses native Wayland by default, with `--display-driver x11` available for XWayland
- Sets up NTSync (auto-detected), esync/fsync, GPU-specific env vars
- Disables Wine systray (orphan floating window on Wayland tiling WMs)
- Extracts the Battle.net icon and creates separate Wayland and X11 desktop entries
- Graceful process management with orphan cleanup

## Directories

| Type | Path |
|------|------|
| Data | `~/.local/share/bnetctl` |
| Cache | `~/.cache/bnetctl` |
| Prefix | `~/.local/share/bnetctl/prefix` |
| Log | `~/.cache/bnetctl/bnetctl.log` |

## License

MIT
