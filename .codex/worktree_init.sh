#!/usr/bin/env bash
set -eo pipefail
cd "$(cd "$(dirname "$0")/.." && pwd)"
go mod download
