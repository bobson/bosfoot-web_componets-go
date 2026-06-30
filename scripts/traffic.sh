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
# - The owner's own connections (OWN_IPS, real Cf-Connecting-Ip) are excluded
#   from the real-visitor metrics. Matched by PREFIX: a full address is an exact
#   match; for the laptop's IPv6 list the /64 prefix ending in ':' (host bits
#   rotate). Keep IPv4 entries FULL — A1 is a big provider, so a short prefix
#   would also drop real customers. The two "incl. bots" raw totals are left
#   unfiltered on purpose.

set -euo pipefail

SINCE="${1:-24 hours ago}"
ASSET_RE='\.(css|js|mjs|ttf|woff2?|webp|avif|jpe?g|png|gif|svg|ico|json|webmanifest|txt|map|xml)$'
OWN_IPS='146.255.75.165 185.100.245.80'   # 146… = home   185… = mobile (A1)

# Bot/test-UA filter (case-insensitive). Mirrors scripts/funnel.sh: named crawlers
# + programmatic clients + a few notorious spoofed-browser fingerprints, plus the
# owner's test phone (A142P). Spoofed-UA bots that look like a normal browser are
# caught instead by the behavioural filter in the VISITORS metric below.
BOT='bot|crawl|spider|facebookexternalhit|headless|scan|python-requests|go-http|curl|wget|okhttp|libwww|java/|axios|node-fetch|ahrefs|semrush|mj12|dataforseo|iphone os 13_2_3|trident/|chrome/88\.|chrome/19\.'
TEST_UA='A142P'
RE="$BOT|$TEST_UA"

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

# Real visitor = an IP that behaved like a browser (loaded HTML *and* at least one
# static asset, OR viewed ≥2 distinct pages) and is not a bot/test-UA/owner IP.
# This drops the single-hit, asset-less spray that spoofed-UA bots produce.
VISITORS=$(printf '%s\n' "$LOG" \
  | jq -r --arg re "$RE" --arg ips "$OWN_IPS" --arg asset "$ASSET_RE" \
     'select( ((((.request.headers."User-Agent"[0]) // "") | test($re;"i")) | not)
        and (((.request.headers."Cf-Connecting-Ip"[0]) // "") as $ip
              | (any($ips | split(" ")[]; . as $p | ($p|length)>0 and ($ip | startswith($p))) | not)) )
      | [ (.request.headers."Cf-Connecting-Ip"[0] // ""),
          ((.request.uri|split("?")[0]) | if test($asset) then "a" else "p" end),
          (.request.uri|split("?")[0]) ] | @tsv' \
  | awk -F'\t' '
      $1=="" { next }
      { ips[$1]=1; if ($2=="a") a[$1]=1; else { k=$1 SUBSEP $3; if (!(k in sp)) { sp[k]=1; pc[$1]++ } } }
      END { n=0; for (ip in ips) if (a[ip] || pc[ip]>=2) n++; print n+0 }' || true)
echo "Unique visitors (real browser: HTML+asset or ≥2 pages; excl. bots/test/owner): $VISITORS"

echo
echo "---- Top pages (status 200, assets/query-strings stripped) ----"
printf '%s\n' "$LOG" \
  | jq -r --arg re "$ASSET_RE" --arg ips "$OWN_IPS" 'select(.status==200 and ((.request.uri) | test($re)|not)
           and (((.request.headers."Cf-Connecting-Ip"[0]) // "") as $ip | (any($ips | split(" ")[]; . as $p | ($p|length)>0 and ($ip | startswith($p))) | not)))
           | .request.uri | split("?")[0]' \
  | sort | uniq -c | sort -rn | head -25

echo
echo "---- Page views per hour (status 200, HTML only) ----"
printf '%s\n' "$LOG" \
  | jq -r --arg re "$ASSET_RE" --arg ips "$OWN_IPS" 'select(.status==200 and ((.request.uri) | test($re)|not)
           and (((.request.headers."Cf-Connecting-Ip"[0]) // "") as $ip | (any($ips | split(" ")[]; . as $p | ($p|length)>0 and ($ip | startswith($p))) | not)))
           | .ts | strftime("%m-%d %H:00")' \
  | sort | uniq -c

echo
echo "---- Requests by country (Cloudflare geo, incl. bots) ----"
printf '%s\n' "$LOG" \
  | jq -r '.request.headers."Cf-Ipcountry"[0] // "?"' \
  | sort | uniq -c | sort -rn | head -15

echo
echo "---- Unique visitors by country (real, excl. bots/crawlers) ----"
printf '%s\n' "$LOG" \
  | jq -r --arg ips "$OWN_IPS" 'select((((.request.headers."User-Agent"[0]) // "") | test("facebookexternalhit|bot|crawl|spider|scan")|not)
           and (((.request.headers."Cf-Connecting-Ip"[0]) // "") as $ip | (any($ips | split(" ")[]; . as $p | ($p|length)>0 and ($ip | startswith($p))) | not)))
           | [(.request.headers."Cf-Ipcountry"[0] // "?"), (.request.headers."Cf-Connecting-Ip"[0] // "?")] | @tsv' \
  | sort -u | cut -f1 | sort | uniq -c | sort -rn | head -15
