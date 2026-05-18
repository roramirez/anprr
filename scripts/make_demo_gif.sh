#!/usr/bin/env bash
# Generates assets/demo.gif by recording an anprr --demo session inside mmterm.
# Requirements: mmterm (release build), xdotool, ffmpeg, gifsicle, DISPLAY set.

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
ANPRR="$REPO/anprr"
OUT_DIR="$REPO/assets"
TMP_MP4="/tmp/anprr_demo_$$.mp4"
OUT_GIF="$OUT_DIR/demo.gif"
CFG_DIR="/tmp/anprr_demo_cfg_$$"

die() { echo "ERROR: $*" >&2; exit 1; }

[[ -x "$ANPRR" ]] || die "anprr not built — run: go build -o anprr ."

MMTERM="mmterm"
command -v mmterm >/dev/null || die "mmterm not found on PATH"
command -v xdotool  >/dev/null || die "xdotool not found"
command -v ffmpeg   >/dev/null || die "ffmpeg not found"
command -v gifsicle >/dev/null || die "gifsicle not found"
[[ -n "${DISPLAY:-}" ]]        || die "DISPLAY not set"

mkdir -p "$OUT_DIR" "$CFG_DIR/mmterm"

# ── Shell wrapper that launches anprr --demo directly ─────────────────────────
cat > /tmp/anprr_demo_shell.sh << SHEOF
#!/usr/bin/env bash
export TERM=xterm-256color
cd "$REPO"
clear
exec "$ANPRR" --demo
SHEOF
chmod +x /tmp/anprr_demo_shell.sh

# ── mmterm config: 1280×720, clean font, our shell wrapper ───────────────────
cat > "$CFG_DIR/mmterm/config.toml" << TOML
[font]
family = "monospace"
size   = 14.0

[window]
width           = 1280
height          = 720
title           = "anprr demo"
cursor_blink_ms = 500

[shell]
program = "/tmp/anprr_demo_shell.sh"
TOML

# ── Launch mmterm ─────────────────────────────────────────────────────────────
echo "→ Launching mmterm…"
FFMPEG_PID=0
XDG_CONFIG_HOME="$CFG_DIR" "$MMTERM" &
MMTERM_PID=$!
trap 'kill $MMTERM_PID 2>/dev/null; [[ $FFMPEG_PID -ne 0 ]] && kill $FFMPEG_PID 2>/dev/null; rm -rf "$CFG_DIR" /tmp/anprr_demo_shell.sh; wait 2>/dev/null' EXIT

# Wait for window (up to 8 s)
WID=""
for i in $(seq 1 40); do
    WID=$(xdotool search --pid "$MMTERM_PID" --onlyvisible 2>/dev/null | head -1 || true)
    [[ -n "$WID" ]] && break
    sleep 0.2
done
[[ -n "$WID" ]] || die "mmterm window never appeared"

xdotool windowraise "$WID"
xdotool windowfocus --sync "$WID"
sleep 2.5   # wait for anprr to load data and render

# Window geometry
WIN_INFO=$(xwininfo -id "$WID" 2>/dev/null)
PX=$(echo "$WIN_INFO" | awk '/Absolute upper-left X:/ {print $NF}')
PY=$(echo "$WIN_INFO" | awk '/Absolute upper-left Y:/ {print $NF}')
W=$(echo  "$WIN_INFO" | awk '/Width:/  {print $NF}')
H=$(echo  "$WIN_INFO" | awk '/Height:/ {print $NF}')
echo "→ Window $WID  ${W}x${H} at ${PX},${PY}"

# ── Start recording ───────────────────────────────────────────────────────────
echo "→ Recording…"
ffmpeg -y \
    -f x11grab -r 20 -s "${W}x${H}" -i ":0.0+${PX},${PY}" \
    -c:v libx264 -preset ultrafast -crf 18 \
    "$TMP_MP4" 2>/dev/null &
FFMPEG_PID=$!
sleep 0.5

# ── Helpers ───────────────────────────────────────────────────────────────────
K()     { xdotool key  --window "$WID" --clearmodifiers "$@"; sleep 0.08; }
PAUSE() { sleep "${1:-1}"; }

# ── Demo script ───────────────────────────────────────────────────────────────
PAUSE 1   # let recording settle

# Scene 1: Tab [1] My PRs — browse the list
PAUSE 1.5
K Down;   PAUSE 0.4
K Down;   PAUSE 0.6

# Scene 2: Switch to [2] Needs Review
K 2;      PAUSE 1.2

# Scene 3: Switch to [3] All Open
K 3;      PAUSE 1.2

# Scene 4: Back to [1], open PR #42 detail
K 1;      PAUSE 0.6
K Return; PAUSE 2    # wait for diff to load

# Scene 5: Scroll the diff
K j; PAUSE 0.3
K j; PAUSE 0.3
K j; PAUSE 0.3
K j; PAUSE 0.3
K j; PAUSE 0.5
K k; PAUSE 0.3
K k; PAUSE 0.6

# Scene 6: Toggle split view
K s;      PAUSE 1.5

# Scene 7: Toggle back to unified
K s;      PAUSE 1

# Scene 8: Enter line-select mode for inline comment
K n;      PAUSE 0.8
K j;      PAUSE 0.3
K j;      PAUSE 0.3
K j;      PAUSE 0.3
K n;      PAUSE 0.8   # open comment input on selected line

# Scene 9: Type an inline comment and submit
xdotool type --window "$WID" --delay 60 "This needs a nil check before dereferencing"
PAUSE 0.8
K ctrl+d; PAUSE 1     # submit inline comment

# Scene 10: Exit line mode, show approve confirmation
K Escape; PAUSE 0.6
K a;      PAUSE 1.8   # approve confirm prompt

# Scene 11: Cancel approve, show help
K Escape; PAUSE 0.6
K question; PAUSE 2   # help overlay

# Scene 12: Close help and back to list
K question; PAUSE 0.5
K b;        PAUSE 1

PAUSE 0.5

# ── Stop recording ────────────────────────────────────────────────────────────
kill $FFMPEG_PID
wait $FFMPEG_PID 2>/dev/null || true
echo "→ Recording stopped."
kill $MMTERM_PID 2>/dev/null || true
trap - EXIT
rm -rf "$CFG_DIR" /tmp/anprr_demo_shell.sh
wait 2>/dev/null || true

# ── MP4 → GIF ────────────────────────────────────────────────────────────────
echo "→ Converting to GIF…"
PALETTE="/tmp/anprr_pal_$$.png"

# Skip first 0.5 s (startup flicker), 18 fps, scale to 1280 wide
VFBASE="trim=start=0.5,setpts=PTS-STARTPTS,fps=18,scale=1280:-1:flags=lanczos"

ffmpeg -y -i "$TMP_MP4" \
    -vf "${VFBASE},palettegen=stats_mode=diff" \
    -update 1 "$PALETTE" 2>/dev/null

ffmpeg -y -i "$TMP_MP4" -i "$PALETTE" \
    -filter_complex "${VFBASE}[x];[x][1:v]paletteuse=dither=bayer:bayer_scale=5" \
    "$OUT_GIF" 2>/dev/null

rm -f "$PALETTE" "$TMP_MP4"

echo "→ Optimising…"
gifsicle -O3 --lossy=60 --colors 256 "$OUT_GIF" -o "$OUT_GIF"

SIZE=$(du -sh "$OUT_GIF" | cut -f1)
echo "✓  $OUT_GIF  ($SIZE)"
