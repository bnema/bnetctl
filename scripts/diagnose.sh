#!/bin/bash
# Diagnostic script: run AFTER launching bnetctl (while Battle.net is open)
# Usage: ./scripts/diagnose.sh

set -uo pipefail

OUR_PFX="$HOME/.local/share/bnetctl/prefix"
WINE_USER="${USER:-$(whoami)}"
OUTDIR="/tmp/bnetctl-diag-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTDIR"

echo "=== bnetctl diagnostics ==="
echo "Output: $OUTDIR"
echo "Wine user: $WINE_USER"
echo ""

# 1. Capture running processes for our prefix
echo "[1/6] Capturing running processes..."
ps aux | grep -E '(\.exe|wine|wineserver)' | grep -v grep > "$OUTDIR/processes.txt" 2>/dev/null || true

# Also capture which prefix each process belongs to
for pid in $(ps aux | grep '\.exe\|wineserver' | grep -v grep | awk '{print $2}'); do
  echo "--- PID $pid ---" >> "$OUTDIR/process-env.txt"
  cat /proc/$pid/environ 2>/dev/null | tr '\0' '\n' | grep -E 'WINEPREFIX|DISPLAY|WAYLAND' >> "$OUTDIR/process-env.txt" 2>/dev/null || true
  echo "" >> "$OUTDIR/process-env.txt"
done

# 2. Latest Battle.net logs
echo "[2/6] Capturing Battle.net logs..."
LATEST_OUR=$(ls -t "$OUR_PFX/pfx/drive_c/users/$WINE_USER/AppData/Local/Battle.net/Logs/battle.net-"*.log 2>/dev/null | head -1)

if [ -n "$LATEST_OUR" ]; then
  cp "$LATEST_OUR" "$OUTDIR/bnet-log.log"
fi

# 3. CEF logs
echo "[3/6] Capturing CEF logs..."
LATEST_CEF_OUR=$(ls -t "$OUR_PFX/pfx/drive_c/users/$WINE_USER/AppData/Local/Battle.net/Logs/libcef-"*.log 2>/dev/null | head -1)

if [ -n "${LATEST_CEF_OUR:-}" ]; then
  cp "$LATEST_CEF_OUR" "$OUTDIR/cef-log.log"
fi

# 4. Battle.net.config
echo "[4/6] Capturing Battle.net config..."
cp "$OUR_PFX/pfx/drive_c/users/$WINE_USER/AppData/Roaming/Battle.net/Battle.net.config" "$OUTDIR/config.json" 2>/dev/null || true

# 5. BrowserCaches contents (file list)
echo "[5/6] Listing BrowserCaches..."
find "$OUR_PFX/pfx/drive_c/users/$WINE_USER/AppData/Local/Battle.net/" -type f 2>/dev/null | sed "s|$OUR_PFX/pfx/||" | sort > "$OUTDIR/appdata-files.txt" || touch "$OUTDIR/appdata-files.txt"

# 6. Extract auth flow from log
echo "[6/6] Extracting auth flow..."
if [ -f "$OUTDIR/bnet-log.log" ]; then
  grep -iE '(UAuth|tassadar|browser state|begin loading|finished loading|setting url|Certificate|Backend)' "$OUTDIR/bnet-log.log" > "$OUTDIR/auth-flow.txt" 2>/dev/null || echo "no auth entries" > "$OUTDIR/auth-flow.txt"
fi

echo ""
echo "=== Done. Key files: ==="
echo "  $OUTDIR/auth-flow.txt     <- auth flow (most important)"
echo "  $OUTDIR/appdata-files.txt <- browser cache listing"
echo "  $OUTDIR/config.json       <- Battle.net config"
echo "  $OUTDIR/processes.txt     <- running processes"
echo "  $OUTDIR/bnet-log.log      <- full bnet log"
echo ""
echo "Quick check:"
echo "  Auth flow: $(grep -c 'begin loading' "$OUTDIR/bnet-log.log" 2>/dev/null || echo 0) page loads"
echo "  BrowserCaches files: $(wc -l < "$OUTDIR/appdata-files.txt" 2>/dev/null || echo 0) files"
