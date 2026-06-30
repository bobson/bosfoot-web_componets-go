#!/usr/bin/env bash
# Daily funnel snapshot for bosfoot: real visitors → product views → add-to-cart
# → checkout → reservations, with bots AND the owner's test device/email filtered
# out so the numbers reflect actual customers.
#
# Usage:  sudo bash scripts/funnel.sh ["since"]
#   since: any journalctl --since value. Default "24 hours ago".
#   e.g.  sudo bash scripts/funnel.sh "today"   sudo bash scripts/funnel.sh "7 days ago"
#
# Run ON the droplet (reads journald). Needs jq.
#  - Caddy access log → page traffic; real client IP/UA come from the Cloudflare
#    headers (Cf-Connecting-Ip / User-Agent), since remote_ip is a CF edge.
#  - bosfoot service log → add-to-cart beacons (msg "funnel") and placed orders.
#
# Tweak the noise filters below if you test from another phone, email, or IP.
set -u

SINCE="${1:-24 hours ago}"

# Noise filters (case-insensitive):
#  BOT     — crawlers / vulnerability scanners
#  TEST_UA — the owner's test phone (UA build id) so self-tests don't inflate
#  TEST_IPS — the owner's own connections, space-separated. Matched by PREFIX, so
#            a full address is an exact match and a partial one covers a range.
#            Keep IPv4 entries FULL (A1 is a big provider — a short prefix would
#            also drop real customers). For the laptop's IPv6, list the /64
#            prefix ending in ':' (the host bits rotate, so a full v6 won't stick).
#              146.255.75.165 = home (IPv4)   185.100.245.80 = mobile (A1, IPv4)
#            If the laptop reaches the site over IPv6, add e.g. 2a02:xxxx:xxxx:xxxx:
# Named crawlers + programmatic clients + a few notorious spoofed-browser
# fingerprints that are never real shoppers. Spoofed-UA bots that look like a
# normal browser are NOT caught here — the behavioural filter below handles those.
BOT='bot|crawl|spider|facebookexternalhit|headless|scan|python-requests|go-http|curl|wget|okhttp|libwww|java/|axios|node-fetch|ahrefs|semrush|mj12|dataforseo|iphone os 13_2_3|trident/|chrome/88\.|chrome/19\.'
TEST_UA='A142P'
TEST_EMAIL='bmarkoski@gmail.com'
TEST_IPS='146.255.75.165 185.100.245.80'
RE="$BOT|$TEST_UA"
# Static-asset URIs (query string stripped before matching). Used to tell a real
# browser (loads HTML *and* assets) from a bot that only grabs the HTML.
ASSET_RE='\.(css|js|mjs|ttf|woff2?|webp|avif|jpe?g|png|gif|svg|ico|json|webmanifest|txt|map|xml)$'

command -v jq >/dev/null 2>&1 || { echo "jq required: sudo apt install jq" >&2; exit 1; }

CADDY="$(journalctl -u caddy   --since "$SINCE" -o cat 2>/dev/null | grep '"msg":"handled request"')"
APP="$(journalctl   -u bosfoot --since "$SINCE" -o cat 2>/dev/null)"

# jq filter over the Caddy log ($re = bot/test UA regex, $ips = owner IP prefixes).
cq() { printf '%s\n' "$CADDY" | jq -r --arg re "$RE" --arg ips "$TEST_IPS" "$1" 2>/dev/null; }
# jq filter over the bosfoot app log ($ips = owner IP prefixes, $em = test email).
aq() { printf '%s\n' "$APP" | jq -r --arg ips "$TEST_IPS" --arg em "$TEST_EMAIL" "$1" 2>/dev/null; }

# Reusable jq sub-expression: pipe an IP string in; true when it is NOT an owner
# IP, i.e. it does not start with any prefix in $ips. (startswith ⇒ a full
# address matches exactly, a partial one matches a whole range; empty prefixes
# from a stray space are ignored so they can't exclude everyone.)
NOTOWN='(. as $ip | (any($ips | split(" ")[]; . as $p | ($p|length)>0 and ($ip | startswith($p))) | not))'
# Request is NOT a bot/test UA AND NOT from an owner IP.
HUMAN='((((.request.headers."User-Agent"[0]) // "") | test($re;"i") | not) and (((.request.headers."Cf-Connecting-Ip"[0]) // "") | '"$NOTOWN"'))'

# A "real visitor" is an IP that behaved like a browser: it loaded at least one
# static asset (CSS/JS/img) alongside its HTML, OR it viewed ≥2 distinct pages.
# Single-hit, asset-less IPs — the dominant bot pattern (scrapers spraying the
# homepage with a spoofed browser UA) — are dropped even though their UA looks
# real. (Assets can be Cloudflare-cached and miss the origin log, so the ≥2-page
# fallback keeps multi-page real sessions counted.)
visitors=$(printf '%s\n' "$CADDY" \
  | jq -r --arg re "$RE" --arg ips "$TEST_IPS" --arg asset "$ASSET_RE" \
      "select($HUMAN) | [ (.request.headers.\"Cf-Connecting-Ip\"[0] // \"\"),
         ((.request.uri|split(\"?\")[0]) | if test(\$asset) then \"a\" else \"p\" end),
         (.request.uri|split(\"?\")[0]) ] | @tsv" 2>/dev/null \
  | awk -F'\t' '
      $1=="" { next }
      { ips[$1]=1; if ($2=="a") a[$1]=1; else { k=$1 SUBSEP $3; if (!(k in sp)) { sp[k]=1; pc[$1]++ } } }
      END { n=0; for (ip in ips) if (a[ip] || pc[ip]>=2) n++; print n+0 }')
pviews=$(cq   "select(.status==200 and (.request.uri|test(\"/products/[^/]+/\")) and $HUMAN) | 1" | grep -c .)
checkouts=$(cq "select((.request.uri|test(\"/checkout\")) and $HUMAN) | 1" | grep -c .)
addcart=$(aq   "select((.event // \"\")==\"add_to_cart\" and ((.ip // \"\") | $NOTOWN)) | 1" | grep -c .)
realorders=$(aq "select(.msg==\"Order placed\" and ((.email // \"\")|ascii_downcase|contains(\$em|ascii_downcase)|not) and ((.ip // \"\") | $NOTOWN)) | 1" | grep -c .)

echo "═══════════════════════════════════════════════════════"
echo " Bosfoot funnel — since: $SINCE"
echo " (bots + owner test device/email filtered out)"
echo "═══════════════════════════════════════════════════════"
printf '  %-34s %s\n' "Real visitors"            "$visitors"
printf '  %-34s %s\n' "↳ Product page views"     "$pviews"
printf '  %-34s %s\n' "↳ Add-to-cart"            "$addcart"
printf '  %-34s %s\n' "↳ Checkout page views"    "$checkouts"
printf '  %-34s %s\n' "↳ Reservations (real)"    "$realorders"

echo
echo "Reservation POST attempts by status (real, excl. bots/test):"
status="$(cq "select(.request.method==\"POST\" and (.request.uri|test(\"/api/orders\")) and $HUMAN) | .status" | sort | uniq -c)"
[ -n "$status" ] && printf '%s\n' "$status" || echo "  (none)"

echo
echo "Placed orders (real, excl. your test email + IPs):"
orders="$(aq "select(.msg==\"Order placed\" and ((.email // \"\")|ascii_downcase|contains(\$em|ascii_downcase)|not) and ((.ip // \"\") | $NOTOWN)) | \"  #\(.order_id)  \(.total_mkd) MKD  \(.items) item(s)  \(.email // \"(no email)\")\"")"
[ -n "$orders" ] && printf '%s\n' "$orders" || echo "  (none yet)"
