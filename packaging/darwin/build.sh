#!/bin/bash
set -euo pipefail
cd "$(dirname "$0")/../.."
: "${MYGITTU_APP_GROUP:?Set MYGITTU_APP_GROUP to TEAMID.mygittu}"
: "${SIGN_IDENTITY:?Set SIGN_IDENTITY to your Developer ID Application identity}"
exec bash packaging/darwin/release.sh "${1:-}" --app-only
