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
# Tweak the noise filters below if you test from another phone or email.
set -u

SINCE="${1:-24 hours ago}"

# Noise filters (case-insensitive):
#  BOT     — crawlers / vulnerability scanners
#  TEST_UA — the owner's test phone (UA build id) so self-tests don't inflate
BOT='bot|crawl|spider|facebookexternalhit|headless|scan'
TEST_UA='A142P'
TEST_EMAIL='bmarkoski@gmail.com'
RE="$BOT|$TEST_UA"

command -v jq >/dev/null 2>&1 || { echo "jq required: sudo apt install jq" >&2; exit 1; }

CADDY="$(journalctl -u caddy   --since "$SINCE" -o cat 2>/dev/null | grep '"msg":"handled request"')"
APP="$(journalctl   -u bosfoot --since "$SINCE" -o cat 2>/dev/null)"

# Run a jq filter over the Caddy log. $re holds the bot/test UA regex.
cq() { printf '%s\n' "$CADDY" | jq -r --arg re "$RE" "$1" 2>/dev/null; }

# Reusable jq sub-expression: request is NOT a bot and NOT the test device.
HUMAN='(((.request.headers."User-Agent"[0]) // "") | test($re;"i") | not)'

visitors=$(cq "select($HUMAN) | .request.headers.\"Cf-Connecting-Ip\"[0] // empty" | sort -u | grep -c .)
pviews=$(cq   "select(.status==200 and (.request.uri|test(\"/products/[^/]+/\")) and $HUMAN) | 1" | grep -c .)
checkouts=$(cq "select((.request.uri|test(\"/checkout\")) and $HUMAN) | 1" | grep -c .)
addcart=$(printf '%s\n' "$APP" | grep -c add_to_cart)
realorders=$(printf '%s\n' "$APP" | grep '"Order placed"' | grep -vc "$TEST_EMAIL")

echo "═══════════════════════════════════════════════════════"
echo " Bosfoot funnel — since: $SINCE"
echo " (bots + owner test device/email filtered out)"
echo "═══════════════════════════════════════════════════════"
printf '  %-34s %s\n' "Real visitors"            "$visitors"
printf '  %-34s %s\n' "↳ Product page views"     "$pviews"
printf '  %-34s %s  %s\n' "↳ Add-to-cart"        "$addcart" "(not device-filtered — subtract your own tests)"
printf '  %-34s %s\n' "↳ Checkout page views"    "$checkouts"
printf '  %-34s %s\n' "↳ Reservations (real)"    "$realorders"

echo
echo "Reservation POST attempts by status (real, excl. bots/test):"
status="$(cq "select(.request.method==\"POST\" and (.request.uri|test(\"/api/orders\")) and $HUMAN) | .status" | sort | uniq -c)"
[ -n "$status" ] && printf '%s\n' "$status" || echo "  (none)"

echo
echo "Placed orders (real, excl. your test email):"
orders="$(printf '%s\n' "$APP" | grep '"Order placed"' | grep -v "$TEST_EMAIL" \
  | jq -r '"  #\(.order_id)  \(.total_mkd) MKD  \(.items) item(s)  \(.email // "(no email)")"' 2>/dev/null)"
[ -n "$orders" ] && printf '%s\n' "$orders" || echo "  (none yet)"
