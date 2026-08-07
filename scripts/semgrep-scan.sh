#!/usr/bin/env bash
# Runs the semgrep rulesets wgtui gates every push on (see docs/THREAT_MODEL.md
# for why: command injection, secret leakage, and general Go security bugs are
# exactly the failure modes this project's threat model calls out). Used by
# both .githooks/pre-push (local, fast feedback) and .github/workflows/ci.yml
# (the actual non-bypassable gate) so the ruleset only lives in one place.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

semgrep scan \
  --config p/golang \
  --config p/security-audit \
  --config p/secrets \
  --error \
  --metrics=off \
  "$@"
