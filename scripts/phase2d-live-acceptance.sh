#!/bin/sh
#
# Phase 2D installed-live acceptance harness (source library).
#
# Safe by construction: this library never reads, writes or transmits
# credentials, secrets, prompts or Provider bodies. It checks prerequisites and
# exports bounded privacy-safe evidence. The live gates themselves are executed
# by the operator in the App; this harness only validates readiness and
# collects the evidence each gate records.
#
# Source-only mode: set LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY=1 to load the
# functions without any filesystem side effects (used by the source test).
set -eu

phase2d_acceptance_gates="G1 G2 G3 G4 G5 G6"

# phase2d_acceptance_installed_app <path-to-Loom.app>
# Prints ok / missing / invalid. Never inspects bundle contents for secrets.
phase2d_acceptance_installed_app() {
  [ "$#" -eq 1 ] || { printf '%s\n' invalid; return 1; }
  app=$1
  case "$app" in
    /*/Loom.app) ;;
    *) printf '%s\n' invalid; return 1 ;;
  esac
  [ -d "$app" ] && [ ! -L "$app" ] || { printf '%s\n' missing; return 0; }
  [ -x "$app/Contents/MacOS/LoomLocalApp" ] || {
    printf '%s\n' missing; return 0
  }
  helper="$app/Contents/Library/Helpers/loomd"
  [ -x "$helper" ] || { printf '%s\n' missing_helper; return 0; }
  printf '%s\n' ok
}

# phase2d_acceptance_required_gate_documents <evidence-root>
# Prints the list of gates whose evidence files are missing (OPEN), or "ok".
phase2d_acceptance_required_gate_documents() {
  [ "$#" -eq 1 ] || { printf '%s\n' invalid; return 1; }
  root=$1
  [ -d "$root" ] || { printf '%s\n' "$phase2d_acceptance_gates"; return 0; }
  missing=""
  for gate in $phase2d_acceptance_gates; do
    document="$root/$gate-*.md"
    # shellcheck disable=SC2086
    [ -n "$(find "$root" -maxdepth 1 -name "$gate-*.md" -type f -print -quit 2>/dev/null)" ] || {
      missing="$missing $gate"
    }
  done
  if [ -z "$missing" ]; then
    printf '%s\n' ok
  else
    # shellcheck disable=SC2086
    printf '%s\n' $missing
  fi
}

# phase2d_acceptance_prerequisite_summary <app> <evidence-root>
# Prints one line per check: "PASS <check>" or "OPEN <check>".
phase2d_acceptance_prerequisite_summary() {
  [ "$#" -eq 2 ] || { printf '%s\n' "OPEN usage"; return 1; }
  app=$1
  root=$2
  case "$(phase2d_acceptance_installed_app "$app")" in
    ok) printf '%s\n' "PASS installed_app" ;;
    *) printf '%s\n' "OPEN installed_app" ;;
  esac
  missing=$(phase2d_acceptance_required_gate_documents "$root")
  if [ "$missing" = "ok" ]; then
    printf '%s\n' "PASS gate_evidence_present"
  else
    # shellcheck disable=SC2086
    for gate in $missing; do
      printf '%s\n' "OPEN gate_evidence_$gate"
    done
  fi
  printf '%s\n' "PASS no_credential_required_for_harness"
}

# phase2d_acceptance_export_evidence <app> <evidence-root> <gate> <incident-id>
# Writes one owner-only privacy-safe evidence file for a gate and prints its
# absolute path. Only versions/hashes/health are captured; never secrets.
phase2d_acceptance_export_evidence() {
  [ "$#" -eq 4 ] || { printf '%s\n' invalid; return 1; }
  app=$1
  root=$2
  gate=$3
  incident=$4
  case " $phase2d_acceptance_gates " in
    *" $gate "*) ;;
    *) printf '%s\n' invalid_gate; return 1 ;;
  esac
  case "$incident" in
    ''|*[!A-Za-z0-9._-]*) printf '%s\n' invalid_incident; return 1 ;;
  esac
  [ -d "$root" ] || mkdir -m 700 -p "$root" 2>/dev/null || {
    printf '%s\n' unavailable; return 0
  }
  document="$root/$gate-$incident.md"
  app_result=$(phase2d_acceptance_installed_app "$app")
  now=$(/bin/date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || printf '%s' unknown)
  umask 077
  {
    printf '%s\n' "# Phase 2D gate evidence $gate"
    printf '%s\n' "- incident_id: $incident"
    printf '%s\n' "- exported_at: $now"
    printf '%s\n' "- app: $app"
    printf '%s\n' "- app_check: $app_result"
    printf '%s\n' "- gate: $gate"
    printf '%s\n'
    printf '%s\n' "This evidence is bounded and privacy-safe. It contains no"
    printf '%s\n' "credential, secret, Prompt, conversation content or Provider"
    printf '%s\n' "body. Full pass criteria are in"
    printf '%s\n' ".loom-evidence/phase2d/acceptance/PHASE-2D-LIVE-ACCEPTANCE.md."
  } > "$document"
  printf '%s\n' "$document"
}

# phase2d_acceptance_summary <app> <evidence-root>
# Prints the final PASS/OPEN summary for every gate.
phase2d_acceptance_summary() {
  phase2d_acceptance_prerequisite_summary "$1" "$2"
}

if [ "${LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY:-0}" = 1 ]; then
  # shellcheck disable=SC2034
  phase2d_acceptance_source_only=1
  return 0 2>/dev/null || exit 0
fi
