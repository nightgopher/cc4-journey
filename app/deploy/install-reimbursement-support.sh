#!/usr/bin/env bash
# One-time OS dependency for validating one-page PDF receipts. No OCR key here.
set -Eeuo pipefail
[[ $(id -u) -eq 0 ]]
if ! command -v pdfinfo >/dev/null 2>&1; then
  apt-get update -qq
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends poppler-utils
fi
pdfinfo -v 2>&1 | head -1
