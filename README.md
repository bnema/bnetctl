# bnetctl

Install, manage, and run [Battle.net](https://battle.net) on Linux via Wine.
Native Wayland is the default; X11/XWayland is available for compatibility.
GPU-specific settings are applied automatically for AMD, NVIDIA, and Intel.

## Requirements

- Wine. On Arch Linux, `wine` and `wine-staging` both work:
  ```bash
  sudo pacman -S wine
  ```
  HDR on native Wayland needs **Wine >= 11.19**. Older Wine still runs, but
  bnetctl warns and HDR is unavailable. CachyOS Wine (`wine-cachyos-opt`)
  currently ships 10.x, so it does not qualify.
- Vulkan drivers for your GPU, plus DXVK and VKD3D-proton package data.
  On CachyOS these are available as `dxvk-mingw-git` and
  `vkd3d-proton-mingw-git`; other distributions may package them differently.
- Optional: `icoutils` to extract the Battle.net icon for desktop entries.
- Optional: CachyOS Wine installed at `/opt/wine-cachyos` as a fallback.

## Install

On Arch Linux, install from the AUR:

```bash
paru -S bnetctl-bin   # Prebuilt release binary
paru -S bnetctl-git   # Built from the latest source
```

Other distributions can download a prebuilt archive (amd64 or arm64) from the
[releases page](https://git.bnema.dev/bnetctl/releases).

Alternatively, with Go installed:

```bash
go install github.com/bnema/bnetctl@latest
```

Make sure your Go binary directory (usually `~/go/bin`) is on `PATH`.

## Usage

```bash
bnetctl install                         # Download and install Battle.net
bnetctl launch                          # Launch with native Wayland
bnetctl install --display-driver x11     # Install through X11/XWayland
bnetctl launch --display-driver x11      # Launch through X11/XWayland
bnetctl launch --no-hdr                  # Disable automatic HDR settings
bnetctl desktop add                     # Add Wayland and X11 menu entries
bnetctl desktop remove                  # Remove menu entries
bnetctl kill                            # Stop Battle.net and leftover Wine processes
bnetctl stop                            # Alias for kill
bnetctl clean                           # Remove cache and prefix
bnetctl clean -a                        # Also remove desktop entries and data
```

### Wine selection

1. `BNETCTL_WINE`, when set, selects the Wine executable for every display mode.
2. Otherwise, use `wine` on `PATH`, falling back to `/opt/wine-cachyos/bin/wine`.
3. A tracked prefix requires Wine at least as new as its recorded version, including for stop; an older explicit override is rejected.
4. `wineboot` and `wineserver` must be beside the resolved Wine binary; Wayland warns below 11.19 (HDR).

For example: `BNETCTL_WINE=/usr/bin/wine bnetctl launch`.

### HDR

Native Wayland automatically enables HDR for supported World of Warcraft
executables through per-executable DXVK settings, not for the Battle.net
launcher. Use Wine >= 11.19, an HDR-capable compositor with HDR enabled, and
an HDR-capable monitor; also enable HDR in the game.

An existing `DXVK_HDR` takes precedence. `--no-hdr` disables automatic settings
and unsets inherited `DXVK_HDR`, but leaves user-provided `DXVK_CONFIG` intact.
X11 mode leaves the user's HDR environment untouched.

## Known issues

- **Battle.net UI under native Wayland:** Wine's Wayland driver cannot render
  the cross-process GPU surface used by Battle.net's embedded Chromium (CEF).
  bnetctl keeps CEF GPU rendering in-process and disables Chromium's background
  throttling; without that, the login popup loads but never returns, leaving
  the launcher on a spinner. If the UI still misbehaves, use
  `--display-driver x11`.
- **HDR:** only on native Wayland with Wine >= 11.19, and only for World of
  Warcraft. The Battle.net launcher itself never runs in HDR.
- **Wine systray:** disabled on every launch; on Wayland tiling compositors it
  shows up as an orphan floating window.
- **Login certificate check:** Battle.net's backend certificate validation fails
  under Wine. bnetctl seeds the cached login address so the sign-in flow can
  recover.

## Troubleshooting

- **Wayland rendering or login problems:** try `--display-driver x11` and check
  your Wine version and Vulkan drivers.
- **Wine version changes:** close Battle.net and its games before switching.
  bnetctl updates the prefix and verifies graphics DLLs when needed. Pending
  RunOnce/RunServicesOnce tasks can block maintenance; resolve the reported
  tasks before retrying. Stop and clean do not run migration preflight.
- **Prefix busy:** maintenance, shutdown, and deletion share an exclusion lock.
  Stop/clean wait up to 10 seconds for the lock; retry after maintenance finishes.
  Launch releases its lock once Wine starts, not at the end of the game session.
- **More detail:** run `bnetctl --verbose launch` to see runtime selection and
  maintenance diagnostics. Logs are saved to `~/.cache/bnetctl/bnetctl.log`.

## Directories

Paths are based on your home directory; `XDG_DATA_HOME` and `XDG_CACHE_HOME`
are not currently honored. Full cleanup retains the data directory and its
`prefix.lock` file so concurrent invocations keep using the same lock inode.

| Type | Default path |
|------|--------------|
| Data | `~/.local/share/bnetctl` |
| Prefix | `~/.local/share/bnetctl/prefix` |
| Cache | `~/.cache/bnetctl` |
| Log | `~/.cache/bnetctl/bnetctl.log` |

## License

MIT
