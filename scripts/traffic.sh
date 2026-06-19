#!/usr/bin/env bash
# Bosfoot traffic report from Caddy's JSON access log (journald).
# Usage:  sudo ./traffic.sh ["since"]
#   since: any journalctl --since value. Default "24 hours ago".
#   e.g.  sudo ./traffic.sh "today"   sudo ./traffic.sh "2 days ago"
#
# Notes:
# - Site sits behind Cloudflare, so the real visitor IP/country come from the
#   Cf-Connecting-Ip / Cf-Ipcountry headers, NOT remote_ip (a Cloudflare IP).
# - Static assets and the facebookexternalhit crawler are filtered out so the
#   numbers reflect real people and real pages.

set -euo pipefail

SINCE="${1:-24 hours ago}"
ASSET_RE='\.(css|js|ttf|woff2|webp|jpg|jpeg|png|svg|ico|json|webmanifest|txt)$'

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required: sudo apt install jq" >&2
  exit 1
fi

# Pull the access lines once; reuse for every section.
LOG="$(journalctl -u caddy --since "$SINCE" -o cat | grep '"msg":"handled request"')"

echo "================================================================"
echo " Bosfoot traffic — since: $SINCE"
echo "================================================================"

TOTAL=$(printf '%s\n' "$LOG" | grep -c . || true)
echo "Total requests (incl. assets/bots): $TOTAL"

VISITORS=$(printf '%s\n' "$LOG" \
  | jq -r 'select(((.request.headers."User-Agent"[0]) // "") | test("facebookexternalhit")|not)
           | .request.headers."Cf-Connecting-Ip"[0] // "?"' \
  | sort -u | grep -vc '^?$' || true)
echo "Unique visitors (real, excl. FB crawler): $VISITORS"

echo
echo "---- Top pages (status 200, assets/query-strings stripped) ----"
printf '%s\n' "$LOG" \
  | jq -r --arg re "$ASSET_RE" 'select(.status==200 and ((.request.uri) | test($re)|not))
           | .request.uri | split("?")[0]' \
  | sort | uniq -c | sort -rn | head -25

echo
echo "---- Page views per hour (status 200, HTML only) ----"
printf '%s\n' "$LOG" \
  | jq -r --arg re "$ASSET_RE" 'select(.status==200 and ((.request.uri) | test($re)|not))
           | .ts | strftime("%m-%d %H:00")' \
  | sort | uniq -c

echo
echo "---- Visitors by country (Cloudflare geo) ----"
printf '%s\n' "$LOG" \
  | jq -r '.request.headers."Cf-Ipcountry"[0] // "?"' \
  | sort | uniq -c | sort -rn | head -15
