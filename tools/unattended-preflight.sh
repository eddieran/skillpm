#!/usr/bin/env bash
set -u
set -o pipefail

usage() {
  cat <<'EOF'
Usage: unattended-preflight.sh [--repo-root PATH] [--host HOST[:PORT]] [--no-default-hosts]

Runs unattended runner startup checks and emits a text report suitable for
copying directly into the ticket workpad.

Defaults:
- repo root: current working directory
- required hosts: github.com
- adds formulae.brew.sh automatically on macOS or when Homebrew is present
- adds any comma/space-separated hosts from SKILLPM_PREFLIGHT_EXTRA_HOSTS
EOF
}

repo_root="$(pwd)"
use_default_hosts=1
hosts=()
failures=()
fingerprint_parts=()
last_output=""
last_status=0
network_backend=""

contains_value() {
  local needle="$1"
  shift
  local value
  for value in "$@"; do
    if [ "$value" = "$needle" ]; then
      return 0
    fi
  done
  return 1
}

add_host() {
  local host="$1"
  if [ -z "$host" ]; then
    return 0
  fi
  if [ "${#hosts[@]}" -gt 0 ] && contains_value "$host" "${hosts[@]}"; then
    return 0
  fi
  hosts+=("$host")
}

add_failure() {
  local failure="$1"
  if [ "${#failures[@]}" -gt 0 ] && contains_value "$failure" "${failures[@]}"; then
    return 0
  fi
  failures+=("$failure")
}

add_fingerprint() {
  fingerprint_parts+=("$1")
}

join_by() {
  local sep="$1"
  shift
  local out=""
  local value
  for value in "$@"; do
    if [ -n "$out" ]; then
      out="${out}${sep}${value}"
    else
      out="$value"
    fi
  done
  printf '%s' "$out"
}

emit_result() {
  local label="$1"
  local display="$2"
  local output="$3"
  local status="$4"

  printf '>>> %s\n' "$label"
  printf '$ %s\n' "$display"
  if [ -n "$output" ]; then
    printf '%s\n' "$output"
  fi
  printf '[exit %s]\n\n' "$status"
}

run_probe() {
  local label="$1"
  local display="$2"
  shift 2

  local output
  local status
  output="$("$@" 2>&1)"
  status=$?

  emit_result "$label" "$display" "$output" "$status"
  last_output="$output"
  last_status="$status"
  return 0
}

check_binary() {
  local name="$1"
  local output
  local status

  output="$(command -v "$name" 2>/dev/null)"
  status=$?
  if [ "$status" -ne 0 ] || [ -z "$output" ]; then
    output="$name not found on PATH"
    status=1
    add_failure "missing-$name"
    add_fingerprint "binary:$name=missing"
  else
    add_fingerprint "binary:$name=$output"
  fi

  emit_result "binary:$name" "command -v $name" "$output" "$status"
}

detect_network_backend() {
  if command -v python3 >/dev/null 2>&1; then
    network_backend="python3"
    return 0
  fi
  if command -v python >/dev/null 2>&1; then
    network_backend="python"
    return 0
  fi
  if command -v curl >/dev/null 2>&1; then
    network_backend="curl"
    return 0
  fi
  return 1
}

probe_host_with_python() {
  local python_bin="$1"
  local host_spec="$2"

  "$python_bin" - "$host_spec" <<'PY'
import socket
import sys

spec = sys.argv[1]
host = spec
port = 443

if spec.startswith("[") and "]:" in spec:
    host, port_text = spec[1:].split("]:", 1)
    port = int(port_text)
elif spec.count(":") == 1 and spec.rsplit(":", 1)[1].isdigit():
    host, port_text = spec.rsplit(":", 1)
    port = int(port_text)

try:
    infos = socket.getaddrinfo(host, port, socket.AF_UNSPEC, socket.SOCK_STREAM)
except Exception as exc:
    print(f"dns lookup failed for {host}:{port}: {exc}")
    sys.exit(1)

addresses = []
for info in infos:
    addr = info[4][0]
    if addr not in addresses:
        addresses.append(addr)

print(f"resolved {host}:{port} -> {', '.join(addresses)}")

try:
    conn = socket.create_connection((host, port), timeout=5)
    conn.close()
except Exception as exc:
    print(f"tcp connect failed for {host}:{port}: {exc}")
    sys.exit(1)

print(f"tcp connect ok {host}:{port}")
PY
}

probe_host_with_curl() {
  local host_spec="$1"
  local url="https://$host_spec/"

  curl --head --silent --show-error --location --max-time 10 "$url" >/dev/null
  printf 'https reachable %s\n' "$host_spec"
}

check_gh_auth() {
  local gh_path
  gh_path="$(command -v gh 2>/dev/null)"
  if [ -z "$gh_path" ]; then
    emit_result "binary:gh" "command -v gh" "gh not found on PATH" 1
    add_failure "missing-gh"
    add_fingerprint "binary:gh=missing"
    add_fingerprint "gh-auth=skipped-missing-gh"
    return 0
  fi

  emit_result "binary:gh" "command -v gh" "$gh_path" 0
  add_fingerprint "binary:gh=$gh_path"

  run_probe "gh-auth" "gh auth status" gh auth status
  if [ "$last_status" -ne 0 ]; then
    add_failure "gh-auth"
  fi
  add_fingerprint "gh-auth=$last_status|$last_output"
}

check_host() {
  local host_spec="$1"

  case "$network_backend" in
    python3|python)
      run_probe "host:$host_spec" "$network_backend - <network probe> $host_spec" probe_host_with_python "$network_backend" "$host_spec"
      ;;
    curl)
      run_probe "host:$host_spec" "curl --head --silent --show-error --location --max-time 10 https://$host_spec/" probe_host_with_curl "$host_spec"
      ;;
    *)
      emit_result "host:$host_spec" "network probe $host_spec" "no supported network probe backend available" 1
      add_failure "network-probe-backend"
      add_fingerprint "network_backend=missing"
      return 0
      ;;
  esac

  if [ "$last_status" -ne 0 ]; then
    add_failure "host:$host_spec"
  fi
  add_fingerprint "host:$host_spec=$last_status|$last_output"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --repo-root)
      if [ $# -lt 2 ]; then
        echo "[preflight] missing value for --repo-root" >&2
        usage >&2
        exit 64
      fi
      repo_root="$2"
      shift 2
      ;;
    --host)
      if [ $# -lt 2 ]; then
        echo "[preflight] missing value for --host" >&2
        usage >&2
        exit 64
      fi
      add_host "$2"
      shift 2
      ;;
    --no-default-hosts)
      use_default_hosts=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "[preflight] unknown argument: $1" >&2
      usage >&2
      exit 64
      ;;
  esac
done

if ! repo_root="$(cd "$repo_root" 2>/dev/null && pwd -P)"; then
  echo "[preflight] repo root not found: $repo_root" >&2
  exit 64
fi

os_name="$(uname -s 2>/dev/null || printf 'unknown')"
if [ "$use_default_hosts" -eq 1 ]; then
  add_host "github.com"
  if [ "$os_name" = "Darwin" ] || command -v brew >/dev/null 2>&1; then
    add_host "formulae.brew.sh"
  fi
fi

if [ -n "${SKILLPM_PREFLIGHT_EXTRA_HOSTS:-}" ]; then
  extra_hosts="$(printf '%s' "$SKILLPM_PREFLIGHT_EXTRA_HOSTS" | tr ',\n' '  ')"
  for extra_host in $extra_hosts; do
    add_host "$extra_host"
  done
fi

go_repo=0
if [ -f "$repo_root/go.mod" ]; then
  go_repo=1
fi

required_hosts="none"
if [ "${#hosts[@]}" -gt 0 ]; then
  required_hosts="$(join_by "," "${hosts[@]}")"
fi

printf '== unattended runner preflight ==\n'
printf 'repo_root=%s\n' "$repo_root"
printf 'go_repo=%s\n' "$go_repo"
printf 'required_hosts=%s\n\n' "$required_hosts"

add_fingerprint "repo_root=$repo_root"
add_fingerprint "go_repo=$go_repo"
add_fingerprint "required_hosts=$required_hosts"

if [ "$go_repo" -eq 1 ]; then
  check_binary "go"
  check_binary "gofmt"
fi

check_gh_auth

if [ "${#hosts[@]}" -gt 0 ]; then
  if detect_network_backend; then
    emit_result "network-backend" "auto-detect network probe backend" "$network_backend" 0
    add_fingerprint "network_backend=$network_backend"

    for host in "${hosts[@]}"; do
      check_host "$host"
    done
  else
    emit_result "network-backend" "auto-detect network probe backend" "no python3, python, or curl available on PATH" 1
    add_failure "network-probe-backend"
    add_fingerprint "network_backend=missing"
  fi
fi

status="pass"
if [ "${#failures[@]}" -gt 0 ]; then
  status="fail"
fi

failure_list="none"
if [ "${#failures[@]}" -gt 0 ]; then
  failure_list="$(join_by "," "${failures[@]}")"
fi

fingerprint_input=""
if [ "${#fingerprint_parts[@]}" -gt 0 ]; then
  fingerprint_input="$(printf '%s\n' "${fingerprint_parts[@]}")"
fi
fingerprint="$(printf '%s' "$fingerprint_input" | cksum | awk '{print $1 "-" $2}')"

printf 'PREFLIGHT_STATUS=%s\n' "$status"
printf 'PREFLIGHT_FAILURES=%s\n' "$failure_list"
printf 'PREFLIGHT_REQUIRED_HOSTS=%s\n' "$required_hosts"
printf 'PREFLIGHT_FINGERPRINT=%s\n' "$fingerprint"

if [ "$status" = "pass" ]; then
  exit 0
fi

exit 1
