#!/bin/bash
# Diagnostic script: run AFTER launching bnetctl (while Battle.net is open)
# Usage: ./scripts/diagnose.sh

set -euo pipefail

OUR_PFX="$HOME/.local/share/bnetctl/prefix"
STEAM_PFX="$HOME/.local/share/Steam/steamapps/compatdata/3810346437"
OUTDIR="/tmp/bnetctl-diag-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$OUTDIR"

echo "=== bnetctl diagnostics ==="
echo "Output: $OUTDIR"
echo ""

# 1. Capture running processes for our prefix
echo "[1/7] Capturing running processes..."
ps aux | grep -E '(\.exe|wine|proton|python3)' | grep -v grep > "$OUTDIR/processes.txt" 2>/dev/null || true

# Also capture which prefix each process belongs to
for pid in $(ps aux | grep '\.exe\|wineserver' | grep -v grep | awk '{print $2}'); do
  echo "--- PID $pid ---" >> "$OUTDIR/process-env.txt"
  cat /proc/$pid/environ 2>/dev/null | tr '\0' '\n' | grep -E 'STEAM_COMPAT|WINEPREFIX|DISPLAY|WAYLAND' >> "$OUTDIR/process-env.txt" 2>/dev/null || true
  echo "" >> "$OUTDIR/process-env.txt"
done

# 2. Latest Battle.net logs from both prefixes
echo "[2/7] Capturing Battle.net logs..."
LATEST_OUR=$(ls -t "$OUR_PFX/pfx/drive_c/users/steamuser/AppData/Local/Battle.net/Logs/battle.net-"*.log 2>/dev/null | head -1)
LATEST_STEAM=$(ls -t "$STEAM_PFX/pfx/drive_c/users/steamuser/AppData/Local/Battle.net/Logs/battle.net-"*.log 2>/dev/null | head -1)

if [ -n "$LATEST_OUR" ]; then
  cp "$LATEST_OUR" "$OUTDIR/bnet-log-ours.log"
fi
if [ -n "$LATEST_STEAM" ]; then
  cp "$LATEST_STEAM" "$OUTDIR/bnet-log-steam.log"
fi

# 3. CEF logs
echo "[3/7] Capturing CEF logs..."
LATEST_CEF_OUR=$(ls -t "$OUR_PFX/pfx/drive_c/users/steamuser/AppData/Local/Battle.net/Logs/libcef-"*.log 2>/dev/null | head -1)
LATEST_CEF_STEAM=$(ls -t "$STEAM_PFX/pfx/drive_c/users/steamuser/AppData/Local/Battle.net/Logs/libcef-"*.log 2>/dev/null | head -1)

if [ -n "${LATEST_CEF_OUR:-}" ]; then
  cp "$LATEST_CEF_OUR" "$OUTDIR/cef-log-ours.log"
fi
if [ -n "${LATEST_CEF_STEAM:-}" ]; then
  cp "$LATEST_CEF_STEAM" "$OUTDIR/cef-log-steam.log"
fi

# 4. Battle.net.config comparison
echo "[4/7] Comparing Battle.net configs..."
diff "$STEAM_PFX/pfx/drive_c/users/steamuser/AppData/Roaming/Battle.net/Battle.net.config" \
     "$OUR_PFX/pfx/drive_c/users/steamuser/AppData/Roaming/Battle.net/Battle.net.config" \
     > "$OUTDIR/config-diff.txt" 2>/dev/null || true

cp "$OUR_PFX/pfx/drive_c/users/steamuser/AppData/Roaming/Battle.net/Battle.net.config" "$OUTDIR/config-ours.json" 2>/dev/null || true
cp "$STEAM_PFX/pfx/drive_c/users/steamuser/AppData/Roaming/Battle.net/Battle.net.config" "$OUTDIR/config-steam.json" 2>/dev/null || true

# 5. Compare BrowserCaches contents (file lists only)
echo "[5/7] Comparing BrowserCaches..."
find "$OUR_PFX/pfx/drive_c/users/steamuser/AppData/Local/Battle.net/" -type f 2>/dev/null | sed "s|$OUR_PFX/pfx/||" | sort > "$OUTDIR/appdata-files-ours.txt"
find "$STEAM_PFX/pfx/drive_c/users/steamuser/AppData/Local/Battle.net/" -type f 2>/dev/null | sed "s|$STEAM_PFX/pfx/||" | sort > "$OUTDIR/appdata-files-steam.txt"
diff "$OUTDIR/appdata-files-steam.txt" "$OUTDIR/appdata-files-ours.txt" > "$OUTDIR/appdata-diff.txt" 2>/dev/null || true

# 6. Proton environment comparison
echo "[6/7] Capturing proton environment..."
# Dump what proton would see in each case
{
  echo "=== OUR prefix config_info ==="
  cat "$OUR_PFX/config_info" 2>/dev/null || echo "missing"
  echo ""
  echo "=== STEAM prefix config_info ==="  
  cat "$STEAM_PFX/config_info" 2>/dev/null || echo "missing"
} > "$OUTDIR/config-info-diff.txt"

# 7. Key summary: extract UAuth flow from both logs
echo "[7/7] Extracting auth flow comparison..."
{
  echo "=== OUR AUTH FLOW ==="
  grep -iE '(UAuth|tassadar|browser state|begin loading|finished loading|setting url|Certificate|Backend)' "$OUTDIR/bnet-log-ours.log" 2>/dev/null || echo "no log"
  echo ""
  echo "=== STEAM AUTH FLOW ==="
  grep -iE '(UAuth|tassadar|browser state|begin loading|finished loading|setting url|Certificate|Backend)' "$OUTDIR/bnet-log-steam.log" 2>/dev/null || echo "no log"
} > "$OUTDIR/auth-flow-comparison.txt"

echo ""
echo "=== Done. Key files: ==="
echo "  $OUTDIR/auth-flow-comparison.txt  <- auth flow diff (most important)"
echo "  $OUTDIR/appdata-diff.txt          <- browser cache diff"
echo "  $OUTDIR/config-diff.txt           <- config diff"
echo "  $OUTDIR/processes.txt             <- running processes"
echo "  $OUTDIR/bnet-log-ours.log         <- full bnet log"
echo ""
echo "Quick check:"
echo "  Auth flow: $(grep -c 'begin loading' "$OUTDIR/bnet-log-ours.log" 2>/dev/null || echo 0) page loads in ours vs $(grep -c 'begin loading' "$OUTDIR/bnet-log-steam.log" 2>/dev/null || echo 0) in steam"
echo "  BrowserCaches files: $(wc -l < "$OUTDIR/appdata-files-ours.txt") ours vs $(wc -l < "$OUTDIR/appdata-files-steam.txt") steam"
