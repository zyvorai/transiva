#!/usr/bin/env bash
# Render Transiva social cards with Google Chrome + macOS `sips` (nothing to install).
#   ./docs/social/build-social-card.sh
# Writes (names unchanged; the dark card is for the README <picture>):
#   docs/social/transiva-share-card.png       1200x630  README hero (light) / Open Graph
#   docs/social/transiva-share-card-dark.png  1200x630  README hero (dark)
#   docs/social/transiva-social-card.jpg      1600x900  LinkedIn / X
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
CHROME="${CHROME:-/Applications/Google Chrome.app/Contents/MacOS/Google Chrome}"
[[ -x "$CHROME" ]] || { echo "Google Chrome not found (set CHROME=...)" >&2; exit 1; }

SOCIAL_OUT="${1:-$HERE/transiva-social-card.jpg}"
SHARE_OUT="${2:-$HERE/transiva-share-card.png}"
DARK_OUT="${3:-$HERE/transiva-share-card-dark.png}"

shot() {  # shot HTML WIDTH HEIGHT OUT FORMAT
  local html="$1" w="$2" h="$3" out="$4" fmt="$5" raw
  raw="$(mktemp "${TMPDIR:-/tmp}/transiva-card.XXXXXX.png")"
  "$CHROME" --headless=new --disable-gpu --hide-scrollbars --force-device-scale-factor=1 \
    --window-size="${w},${h}" --screenshot="$raw" "file://$html" >/dev/null 2>&1
  if [[ "$fmt" == jpeg ]]; then
    sips -s format jpeg -s formatOptions 92 "$raw" --out "$out" >/dev/null
  else
    sips -s format png "$raw" --out "$out" >/dev/null
  fi
  rm -f "$raw"
  echo "wrote $out ($(sips -g pixelWidth -g pixelHeight "$out" | awk '/pixel/{printf "%s ", $2}')px, $(du -k "$out" | cut -f1) KB)"
}

shot "$HERE/transiva-social-card.html"     1600 900 "$SOCIAL_OUT" jpeg
shot "$HERE/transiva-share-card.html"      1200 630 "$SHARE_OUT"  png
shot "$HERE/transiva-share-card-dark.html" 1200 630 "$DARK_OUT"   png
