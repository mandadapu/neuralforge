#!/usr/bin/env bash
# Regenerate requirements.txt with cryptographic hash verification.
#
# Run this script whenever requirements.in changes, then commit the
# updated requirements.txt.  The --generate-hashes flag pins every
# transitive dependency to a sha256 hash so that `pip install
# --require-hashes -r requirements.txt` verifies package integrity at
# install time.
#
# Usage:
#   cd nw-agent/
#   bash compile-requirements.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Installing pip-tools..."
pip install --quiet pip-tools

echo "Compiling requirements.txt with hash verification..."
pip-compile \
  --generate-hashes \
  --allow-unsafe \
  --output-file requirements.txt \
  requirements.in

echo "Done. Verify with:"
echo "  pip install --require-hashes -r requirements.txt"
