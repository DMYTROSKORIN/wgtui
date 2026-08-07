#!/usr/bin/env bash
# One-time setup: points this clone's git at the versioned hooks in
# .githooks/ (git does not do this automatically - .git/hooks/ itself is
# never checked in). Run this once after cloning.
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"

git config core.hooksPath .githooks
echo "Installed: git hooks now run from .githooks/ (pre-push runs semgrep - see scripts/semgrep-scan.sh)."
