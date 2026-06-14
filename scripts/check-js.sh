#!/usr/bin/env bash
# Syntax-check every browser JS module under public/.
#
# The files are ES modules (they use `import`), so each is checked as a .mjs —
# `node --check foo.js` would treat a .js file as CommonJS and false-error on the
# `import` statements. A real syntax error (e.g. a broken template literal) blocks
# the whole app.js module chain at PARSE time, before any code runs, killing all
# interactivity site-wide. This is the cheap gate that catches that class of bug
# before it ships — run it locally before pushing; CI runs it before deploy.
set -euo pipefail

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fail=0
while IFS= read -r -d '' f; do
  cp "$f" "$tmp/check.mjs"
  if err="$(node --check "$tmp/check.mjs" 2>&1)"; then
    echo "  ok   $f"
  else
    echo "  FAIL $f"
    echo "$err" | sed 's/^/       /'
    fail=1
  fi
done < <(find public -name '*.js' -type f -print0 | sort -z)

if [ "$fail" -ne 0 ]; then
  echo "JS syntax check FAILED"
  exit 1
fi
echo "JS syntax check passed"
