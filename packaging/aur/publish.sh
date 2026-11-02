#!/usr/bin/env bash
# Build and publish bnetctl-git. Run as a non-root user with makepkg available.
# DRY_RUN=1 builds and prints metadata without contacting the AUR.
set -euo pipefail
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp "$1" "$work/PKGBUILD"
cd "$work"
makepkg --syncdeps --force --noconfirm
# pkgver() updates PKGBUILD from the source's latest stable tag.
makepkg --printsrcinfo >.SRCINFO
pkgver=$(sed -n 's/^\tpkgver = //p' .SRCINFO)
if [[ ${DRY_RUN:-} == 1 ]]; then
    cat PKGBUILD .SRCINFO
    exit 0
fi

key=$work/aur-key
(umask 077 && printf '%s\n' "$AUR_KEY" >"$key")
# Pinned AUR host key, as in neferwl; do not trust an unauthenticated keyscan.
# SHA256:RFzBCUItH9LZS0cKB5UE6ceAYhBD5C8GeOBip8Z11+4
echo 'aur.archlinux.org ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEuBKrPzbawxA/k2g6NcyV5jmqwJ2s+zpgZGZ7tpLIcN' >"$work/known_hosts"
export GIT_SSH_COMMAND="ssh -i $key -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile=$work/known_hosts"
git clone ssh://aur@aur.archlinux.org/bnetctl-git.git aur
cp PKGBUILD .SRCINFO aur/
cd aur
git add PKGBUILD .SRCINFO
# Works for an unborn HEAD too: staged files differ from the empty tree.
if git diff --cached --quiet; then
    echo "bnetctl-git $pkgver is already on the AUR"
    exit 0
fi
git -c user.name=bnema -c user.email=b@bnema.dev \
    -c gpg.format=ssh -c user.signingkey="$key" \
    commit -S -m "Update to $pkgver"
git push origin HEAD:master
