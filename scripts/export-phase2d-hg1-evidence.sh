#!/bin/sh
set -eu

if [ "$#" -ne 3 ]; then
  printf '%s\n' "usage: $0 /absolute/path/Loom.app /absolute/evidence-root /absolute/app-diagnostics.json" >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY=1
export LOOM_PHASE2D_ACCEPTANCE_SOURCE_ONLY
# shellcheck disable=SC1091
. "$script_dir/phase2d-live-acceptance.sh"

phase2d_acceptance_export_hg1_evidence \
  "$1" "$2" "$3" "$script_dir/phase2d-hg1-trace.jq"
