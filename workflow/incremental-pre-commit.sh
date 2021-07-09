#!/usr/bin/env bash
# shellcheck source=lib/setup.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/setup.sh" || exit "${EXIT_FAILED_TO_SOURCE}"

log_info "pre-commit run start"

# https://pre-commit.com/#usage-in-continuous-integration
args=(
  "--source=origin/HEAD"
  "--origin=HEAD"
  "--show-diff-on-failure"
)
[[ -n "${SCRIPT_DEBUG:-}" ]] && args+=("--verbose")

pre-commit run \
  "${args[@]}" \
  "${@}"

log_info "pre-commit run enc"
