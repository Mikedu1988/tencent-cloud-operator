#!/usr/bin/env bash
# shellcheck source=../lib/setup.sh
source "$(dirname "${BASH_SOURCE[0]}")/workflow/lib/setup.sh" || exit "${EXIT_FAILED_TO_SOURCE}"

log_info "Installing pip and python requirements (pre-commit)."
pip install --upgrade "pip==21.1.1"
pip install --requirement "requirements.txt"

# check some dependencies
fail_msgs=()
command -v "docker" >"/dev/null" || fail_msgs+=("docker not present in PATH. Install it: file://$(pwd)/docs/onboarding.md#docker")
command -v "go" >"/dev/null" || fail_msgs+=("go not present in PATH. Install it: https://golang.org/dl/")
command -v "envsubst" >"/dev/null" || fail_msgs+=("envsubst is not present in PATH. Install it: https://command-not-found.com/envsubst (On macOS, \`brew install gettext\`)")
command -v "openssl" >"/dev/null" || fail_msgs+=("openssl is not present in PATH. Install it: https://command-not-found.com/openssl")
command -v "kustomize" >"/dev/null" || fail_msgs+=("kustomize is not present in PATH. Install it: https://kubectl.docs.kubernetes.io/installation/kustomize")

if [[ ${#fail_msgs[@]} -ne 0 ]]; then
  log_fatal "Error with following dependencies:\n$(
    IFS=$'\n'
    echo "${fail_msgs[*]}"
  )"
fi

bash --version | grep -E "GNU bash, version [4-5].*" || log_fatal "bash must be version >= 4. Probably you're on macOS; \`brew install bash\`."

log_info "Running direnv allow."
log_action "ACTION REQUIRED: Please inspect .envrc to make sure you understand how your environment is being modified."
direnv allow

log_info "Setting origin/HEAD to master (this is required by pre-commit)."
git version
git branch -a
git remote set-head "origin" master

#workflow/config/setup-tools.sh
