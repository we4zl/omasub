#!/usr/bin/env bash

set -euo pipefail

cd "$(dirname "$0")/.."

forbidden_pattern=$(printf '\163\165\142\167\145\141\172\154\174\167\145\141\172\154\174\167\145\141\172')

if rg -i --hidden \
  --glob '!.git/**' \
  "$forbidden_pattern" .; then
  echo "Legacy branding remains in shipped material." >&2
  exit 1
fi

oversized=$(find cmd internal widget -type f \( -name '*.go' -o -name '*.qml' -o -name '*.js' \) -exec wc -l {} + |
  awk '$1 > 300 && $2 != "total" { print }')
if [[ -n $oversized ]]; then
  echo "First-party source files over 300 lines:" >&2
  echo "$oversized" >&2
  exit 1
fi

echo "Source checks passed."
