#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
harness="$repo_root/scripts/phase2d-live-acceptance.sh"

if [ ! -f "$harness" ]; then
  echo "RED: Phase 2D acceptance harness is missing" >&2
  exit 1
fi
sh -n "$harness"

# Source-only mode must load without side effects.
LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY=1
# shellcheck disable=SC1090
. "$harness"
test "${phase2d_acceptance_source_only:-0}" = 1

# No secret-shaped markers may appear in the harness source.
if grep -Eiq 'api[_-]?key|authorization|bearer|BEGIN (RSA |EC |OPENSSH )?PRIVATE|DEEPSEEK-|sk-[A-Za-z0-9]' "$harness"; then
  echo "RED: harness contains a secret-shaped marker" >&2
  exit 1
fi

# Required functions must exist.
for fn in \
  phase2d_acceptance_installed_app \
  phase2d_acceptance_required_gate_documents \
  phase2d_acceptance_prerequisite_summary \
  phase2d_acceptance_export_evidence \
  phase2d_acceptance_summary
do
  grep -q "^$fn()" "$harness" || {
    echo "RED: harness missing function $fn" >&2
    exit 1
  }
done

# Behavior checks in a private temp dir (no real App, no credentials).
private_root=$(mktemp -d "${TMPDIR:-/tmp}/loom-phase2d-acceptance.XXXXXX")
trap 'chmod -R u+rwX "$private_root" 2>/dev/null || true; rm -rf "$private_root"' EXIT HUP INT TERM
chmod 700 "$private_root"

test "$(phase2d_acceptance_installed_app "$private_root/Applications/Loom.app")" = "missing"
test "$(phase2d_acceptance_installed_app "relative/Loom.app")" = "invalid"
test "$(phase2d_acceptance_installed_app "$private_root/Applications/Other.app")" = "invalid"

missing=$(phase2d_acceptance_required_gate_documents "$private_root/evidence")
for expected_gate in G1 G2 G3 G4 G5 G6; do
  case " $missing " in
    *" $expected_gate "*) ;;
    *)
      echo "RED: expected gate $expected_gate OPEN, got: $missing" >&2
      exit 1
      ;;
  esac
done

document=$(phase2d_acceptance_export_evidence \
  "$private_root/Applications/Loom.app" \
  "$private_root/evidence" \
  G1 \
  "incident-0001")
[ -f "$document" ] || {
  echo "RED: evidence file not written" >&2
  exit 1
}
permissions=$(/usr/bin/stat -f '%Lp' "$document")
[ "$permissions" = "600" ] || {
  echo "RED: evidence file not owner-only: $permissions" >&2
  exit 1
}
grep -q "incident_id: incident-0001" "$document" || {
  echo "RED: evidence incident not recorded" >&2
  exit 1
}

test "$(phase2d_acceptance_export_evidence \
  "$private_root" "$private_root/evidence" G9 "incident-0002")" = "invalid_gate"
test "$(phase2d_acceptance_export_evidence \
  "$private_root" "$private_root/evidence" G1 "bad incident!")" = "invalid_incident"

echo "phase2d acceptance harness fixture PASS"
