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
bnetctl install --display-driver x11 # Install through X11/XWayland
bnetctl launch           # Start Battle.net using native Wayland
bnetctl launch --display-driver x11 # Start through X11/XWayland
bnetctl desktop add      # Create Wayland and X11 menu entries
bnetctl desktop remove   # Remove .desktop menu entry
bnetctl kill             # Stop Battle.net, including leftover Wine processes
bnetctl clean            # Remove cache and prefix
bnetctl clean -a         # Full purge (cache, prefix, desktop entry, data)
```

## What it does

- Prefers `/opt/wine-cachyos/bin/wine`; in Wayland mode, prefers Wine >= 11.19, falling back to `wine` on PATH when necessary. `BNETCTL_WINE` always wins
- Creates a Wine prefix with DXVK + VKD3D-proton auto-installed
- Downloads and runs the official Battle.net installer
- Uses native Wayland by default for both `install` and `launch`, with `--display-driver x11` available for XWayland
- Sets up NTSync (auto-detected), esync/fsync, GPU-specific env vars
- Disables Wine systray (orphan floating window on Wayland tiling WMs)
- Extracts the Battle.net icon and creates separate Wayland and X11 desktop entries
- Graceful process management with orphan cleanup

## HDR on Wayland

HDR is enabled automatically for native Wayland (`DXVK_HDR=1`). It requires
**Wine >= 11.19**, an HDR-capable compositor with HDR enabled, and an HDR-capable
monitor/game. An existing `DXVK_HDR` environment variable is respected, including
an empty value. To disable HDR explicitly (also overriding `DXVK_HDR=1`):

```bash
bnetctl launch --no-hdr
```

X11 mode does not set `DXVK_HDR`. Games started by Battle.net inherit its HDR
policy; enable HDR in the game's own settings too.

For Wayland `install` and `launch`, bnetctl prefers CachyOS Wine when it meets the
minimum, otherwise qualifying `wine` on PATH. If neither qualifies, it retains
the usual preference and warns. X11 selection is unchanged. `BNETCTL_WINE`
always wins, e.g. `BNETCTL_WINE=/usr/bin/wine`; companion `wineboot` and
`wineserver` must be beside its resolved binary.

On the first launch of an existing prefix without a version marker, or after a
Wine version change, bnetctl runs `wineboot -u -r`, waits for Wine to exit, and
directly reinstalls and verifies DXVK/vkd3d-proton DLL symlinks and `native`
overrides. No setup scripts are executed. Package data is found beside the
resolved setup scripts, falling back to `/usr/share/dxvk` and
`/usr/share/vkd3d-proton` (including Arch's vkd3d wrapper).

**Maintenance limitation:** `-r` skips Run keys and Startup, but Wine still
processes RunServicesOnce and HKLM RunOnce (`programs/wineboot/wineboot.c`,
lines 1931 and 1945; Run/Startup are gated at 1947–1952). Before maintenance,
bnetctl reads `pfx/system.reg` and `pfx/user.reg` and refuses if RunOnce or
RunServicesOnce, including Wow6432Node variants, contains any values. The error
lists the entries. bnetctl never deletes or disables these registry values.
Resolve pending one-time startup tasks yourself before retrying maintenance.

The `prefix/wine-version` marker is committed only after verification. A
`prefix/wine-version.migrating` record is written before updating; failures
retain it and force a retry, even if the committed version already matches.
Both records establish a minimum Wine version: X11 and Wayland will use an
available runtime at least that new, or refuse. An older explicit `BNETCTL_WINE`
is rejected rather than silently overridden. Switching display modes never
downgrades a tracked prefix. Version probes (`wine --version`) may run during
selection; downgrade refusals run no prefix Wine commands.

An exclusive `prefix/.bnetctl.lock` protects selection and maintenance until the
application process starts; concurrent invocations fail immediately. Close
Battle.net and all its games before switching Wine versions. The first update
may take longer. The default prefix is `~/.local/share/bnetctl/prefix`.

Use `bnetctl --verbose launch --display-driver wayland` to see the selected Wine,
HDR policy, and any one-time prefix update on stderr. Logs are always saved to
`~/.cache/bnetctl/bnetctl.log` (or the corresponding XDG cache directory).

## Directories

| Type | Path |
|------|------|
| Data | `~/.local/share/bnetctl` |
| Cache | `~/.cache/bnetctl` |
| Prefix | `~/.local/share/bnetctl/prefix` |
| Log | `~/.cache/bnetctl/bnetctl.log` |

## License

MIT
