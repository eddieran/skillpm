#!/usr/bin/env bash
set -euo pipefail

RESULTS="${1:?Usage: bench-compare.sh <results-file>}"
BASELINE="tools/bench-baseline.txt"
THRESHOLD=20

if [ ! -f "$BASELINE" ]; then
    echo "[bench] No baseline found. Saving current as baseline."
    cp "$RESULTS" "$BASELINE"
    exit 0
fi

echo "[bench] Comparing against baseline..."
fail=0

while IFS= read -r line; do
    name=$(echo "$line" | awk '{print $1}')
    [[ "$name" != Benchmark* ]] && continue
    cur_ns=$(echo "$line" | grep -oP '\d+(\.\d+)?\s+ns/op' | awk '{print $1}')
    [ -z "$cur_ns" ] && continue

    base_line=$(grep "^$name " "$BASELINE" 2>/dev/null || true)
    [ -z "$base_line" ] && continue
    base_ns=$(echo "$base_line" | grep -oP '\d+(\.\d+)?\s+ns/op' | awk '{print $1}')
    [ -z "$base_ns" ] && continue

    delta=$(awk -v c="$cur_ns" -v b="$base_ns" 'BEGIN { if (b > 0) printf "%.1f", ((c - b) / b) * 100; else print "0" }')
    exceeded=$(awk -v d="$delta" -v t="$THRESHOLD" 'BEGIN { print (d > t) ? 1 : 0 }')

    if [ "$exceeded" -eq 1 ]; then
        echo "[bench] REGRESSION: $name  baseline=${base_ns}ns  current=${cur_ns}ns  delta=+${delta}%"
        fail=1
    else
        echo "[bench] OK: $name  delta=${delta}%"
    fi
done < "$RESULTS"

if [ "$fail" -ne 0 ]; then
    echo "[bench] FAIL: performance regressions detected (threshold=${THRESHOLD}%)"
    exit 1
fi
echo "[bench] PASS: no regressions"
