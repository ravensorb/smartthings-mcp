#!/usr/bin/env bash
# Thin wrapper around get-token.py
# Usage: ./scripts/get-token.sh [.env file]
exec python3 "$(dirname "$0")/get-token.py" "$@"
