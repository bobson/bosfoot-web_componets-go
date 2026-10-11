#!/usr/bin/env bash
# Render an ad card: bash ads/render.sh out.png bg.jpg "headline|line2" "model" "price" "sub" [old-price] [badge] [layout]
# layout: empty = headline top-left, "right" = headline top-right
set -eu
cd "$(dirname "$0")"
enc() { python3 -c 'import sys,urllib.parse;print(urllib.parse.quote(sys.argv[1]))' "$1"; }
url="file://$PWD/card.html?bg=$(enc "$2")&headline=$(enc "$3")&model=$(enc "$4")&price=$(enc "$5")&sub=$(enc "$6")&old=$(enc "${7:-}")&badge=$(enc "${8:-}")&layout=$(enc "${9:-}")"
chromium --headless --disable-gpu --hide-scrollbars --allow-file-access-from-files \
  --virtual-time-budget=3000 --window-size=1080,1080 --screenshot="$PWD/$1" "$url" 2>/dev/null
echo "wrote ads/$1"
