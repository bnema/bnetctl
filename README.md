# bnetctl

A Go CLI tool to install, manage, and run [Battle.net](https://battle.net) on Linux via [wine-cachyos](https://github.com/CachyOS/wine-cachyos).

Built for **Wayland** tiling WMs (niri, Hyprland) with XWayland. Works on X11 too. Automatic GPU detection (AMD/NVIDIA/Intel).

## Dependencies

```bash
# Required
sudo pacman -S wine-cachyos

# Required (game performance — DXVK/VKD3D translate D3D to Vulkan)
sudo pacman -S dxvk-mingw-git vkd3d-proton-mingw-git

# Optional (extracts Battle.net icon for .desktop entry)
sudo pacman -S icoutils
```

## Install

```bash
go install github.com/bnema/bnetctl@latest
```

## Usage

```bash
bnetctl install          # Download, create prefix, install Battle.net
bnetctl launch           # Start Battle.net
bnetctl desktop add      # Create .desktop menu entry
bnetctl desktop remove   # Remove .desktop menu entry
bnetctl kill             # Stop Battle.net
bnetctl kill -a          # Kill all orphaned Wine processes
bnetctl clean            # Remove cache and prefix
bnetctl clean -a         # Full purge (cache, prefix, desktop entry, data)
```

## What it does

- Creates a Wine prefix with DXVK + VKD3D-proton auto-installed
- Downloads and runs the official Battle.net installer
- Sets up NTSync (auto-detected), esync/fsync, GPU-specific env vars
- Disables Wine systray (orphan floating window on Wayland tiling WMs)
- Extracts Battle.net icon and creates .desktop entry
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
