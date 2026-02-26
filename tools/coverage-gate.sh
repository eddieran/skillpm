#!/usr/bin/env bash
set -euo pipefail

# Critical package coverage gates (v1 quality floor)
# Format: <package> <min_percent>
GATES=$(cat <<'EOF'
./internal/store 50
./internal/sync 80
./internal/importer 80
./internal/audit 80
./internal/config 50
EOF
)

fail=0
while IFS=' ' read -r pkg threshold; do
  [ -z "$pkg" ] && continue
  out="$(go test -cover "$pkg" 2>&1)"
  echo "$out"
  cov="$(echo "$out" | sed -nE 's/.*coverage: ([0-9]+\.?[0-9]*)% of statements.*/\1/p' | tail -1)"
  if [ -z "$cov" ]; then
    echo "[coverage-gate] ERROR: unable to parse coverage for $pkg"
    fail=1
    continue
  fi
  if awk -v c="$cov" -v t="$threshold" 'BEGIN { exit !(c+0 >= t+0) }'; then
    echo "[coverage-gate] PASS: $pkg coverage ${cov}% >= ${threshold}%"
  else
    echo "[coverage-gate] FAIL: $pkg coverage ${cov}% < ${threshold}%"
    fail=1
  fi
  echo "---"
done <<EOF
$GATES
EOF

if [ "$fail" -ne 0 ]; then
  echo "[coverage-gate] one or more coverage gates failed"
  exit 1
fi

echo "[coverage-gate] all coverage gates passed"
